package handlers

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// MetadataFormData represents form data for editing media metadata
type MetadataFormData struct {
	Name             string `json:"name" form:"name"`
	Author           string `json:"author" form:"author"`
	Description      string `json:"description" form:"description"`
	Year             int    `json:"year" form:"year"`
	OriginalLanguage string `json:"original_language" form:"original_language"`
	Type             string `json:"manga_type" form:"manga_type"`
	Status           string `json:"status" form:"status"`
	ContentRating    string `json:"content_rating" form:"content_rating"`
	Tags             string `json:"tags" form:"tags"`
	CoverURL         string `json:"cover_url" form:"cover_url"`
}

// HandleUpdateMetadataMedia displays search results for updating a local media's metadata.
func HandleUpdateMetadataMedia(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	search := c.Query("search")

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		return sendInternalServerError(c, ErrMetadataProviderError, err)
	}

	// Search using the provider
	results, err := provider.Search(search)
	if err != nil {
		return sendInternalServerError(c, ErrMetadataSearchFailed, err)
	}

	// Sort results by similarity score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].SimilarityScore > results[j].SimilarityScore
	})

	return HandleView(c, views.UpdateMetadataResults(results, mangaSlug))
}

// HandleEditMetadataMedia applies selected metadata to an existing media.
func HandleEditMetadataMedia(c *fiber.Ctx) error {
	metadataID := c.Query("id")
	mangaSlug := c.Query("slug")
	if mangaSlug == "" {
		return sendBadRequestError(c, ErrRequiredField)
	}

	existingMedia, err := models.GetMedia(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if existingMedia == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		return sendInternalServerError(c, ErrMetadataProviderError, err)
	}

	// Fetch metadata using the provider
	meta, err := provider.GetMetadata(metadataID)
	if err != nil {
		return sendInternalServerError(c, ErrMetadataSyncFailed, err)
	}

	// Get cover URL and download/cache it
	coverURL := provider.GetCoverImageURL(meta)
	var cachedImageURL string
	if coverURL != "" {
		cachedImageURL, err = scheduler.DownloadAndCacheImage(existingMedia.Slug, coverURL)
		if err != nil {
			log.Warnf("Failed to download cover art: %v", err)
			// Try local images as fallback
			cachedImageURL, _ = scheduler.HandleLocalImages(existingMedia.Slug, existingMedia.Path)
		}
	}

	// Update media with metadata
	originalType := existingMedia.Type
	metadata.UpdateMedia(existingMedia, meta, cachedImageURL)

	// Check if the media is a webtoon by checking image dimensions, and overwrite type if detected
	detectedType := scheduler.DetectWebtoonFromImages(existingMedia.Path, existingMedia.Slug)
	if detectedType != "" {
		if originalType == "media" && detectedType == "webtoon" {
			log.Infof("Overriding media type from 'media' to 'webtoon' for '%s' based on image aspect ratio", existingMedia.Slug)
		}
		existingMedia.SetType(detectedType)
	}

	// Persist tags
	if len(meta.Tags) > 0 {
		log.Debugf("Setting %d tags for media '%s' from metadata update: %v", len(meta.Tags), existingMedia.Slug, meta.Tags)
		if err := models.SetTagsForMedia(existingMedia.Slug, meta.Tags); err != nil {
			log.Warnf("Failed to persist tags: %v", err)
		}
	} else {
		log.Debugf("No tags in metadata for media '%s'", existingMedia.Slug)
	}

	if err := models.UpdateMedia(existingMedia); err != nil {
		return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
	}

	triggerNotification(c, "Metadata updated successfully", "success")
	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleManualEditMetadata handles manual metadata updates by moderators or admins
func HandleManualEditMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return sendBadRequestError(c, ErrRequiredField)
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if existingMedia == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Parse form data
	var formData MetadataFormData
	if err := c.BodyParser(&formData); err != nil {
		log.Errorf("Failed to parse metadata form data: %v", err)
		return sendBadRequestError(c, ErrBadRequest)
	}

	// Update fields
	existingMedia.Name = formData.Name
	existingMedia.Author = formData.Author
	existingMedia.Description = formData.Description
	existingMedia.Year = formData.Year
	existingMedia.OriginalLanguage = formData.OriginalLanguage
	if formData.Type != "" {
		existingMedia.Type = formData.Type
	}
	if formData.Status != "" {
		existingMedia.Status = formData.Status
	}
	if formData.ContentRating != "" {
		existingMedia.ContentRating = formData.ContentRating
	}

	// Process tags (comma-separated list)
	var tags []string
	for _, tag := range strings.Split(formData.Tags, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if err := models.SetTagsForMedia(existingMedia.Slug, tags); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Process cover art URL (download and cache)
	if formData.CoverURL != "" {
		cachedImageURL, err := scheduler.DownloadAndCacheImage(existingMedia.Slug, formData.CoverURL)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		existingMedia.CoverArtURL = cachedImageURL
	}

	// Update media in database
	if err := models.UpdateMedia(existingMedia); err != nil {
		return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
	}

	// Return success response for HTMX
	triggerNotification(c, "Metadata updated successfully", "success")
	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleReindexChapters re-indexes chapters without fetching external metadata
func HandleReindexChapters(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return sendBadRequestError(c, ErrRequiredField)
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if existingMedia == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Re-index chapters (this will detect new/removed chapters without deleting the media)
	added, deleted, newChapterSlugs, _, err := scheduler.IndexChapters(existingMedia.Slug, existingMedia.Path, false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// If new chapters were added, notify users
	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(existingMedia.Slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in media '%s': %s", existingMedia.Slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		log.Infof("Re-indexed chapters for media '%s' (added: %d, deleted: %d)", mangaSlug, added, deleted)
	} else {
		log.Infof("Chapter re-index complete for media '%s' (no changes)", mangaSlug)
	}

	// Return success response for HTMX
	message := "Chapters re-indexed successfully"
	if added > 0 || deleted > 0 {
		message = fmt.Sprintf("Chapters re-indexed: %d added, %d removed", added, deleted)
	}
	triggerNotification(c, message, "success")
	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleRefreshMetadata refreshes media metadata and chapters without resetting creation date
func HandleRefreshMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return sendBadRequestError(c, ErrRequiredField)
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if existingMedia == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		log.Warnf("Failed to get app config: %v", err)
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		log.Warnf("Failed to get metadata provider: %v", err)
		return sendInternalServerError(c, ErrMetadataProviderError, err)
	}

	// Fetch fresh metadata from the configured provider
	meta, err := provider.FindBestMatch(existingMedia.Name)
	if err != nil {
		// Log the warning but don't fail - fall back to local metadata
		log.Warnf("Failed to fetch metadata from %s for '%s': %v. Falling back to local metadata.", provider.Name(), existingMedia.Name, err)
	}

	if meta != nil {
		// Get the cover art URL from the provider
		coverURL := provider.GetCoverImageURL(meta)

		// Download and cache the new cover art if available
		var cachedImageURL string
		if coverURL != "" {
			log.Debugf("Attempting to download cover art from provider for media '%s': %s", mangaSlug, coverURL)
			cachedImageURL, err = scheduler.DownloadAndCacheImage(mangaSlug, coverURL)
			if err != nil {
				log.Warnf("Failed to download cover art during metadata refresh: %v", err)
				// Try to fall back to local images
				log.Debugf("Falling back to local images for poster generation for media '%s'", mangaSlug)
				cachedImageURL, _ = scheduler.HandleLocalImages(mangaSlug, existingMedia.Path)
			}
		} else {
			// No cover URL from provider, try local images
			log.Debugf("No cover URL from provider for media '%s', trying local images", mangaSlug)
			cachedImageURL, _ = scheduler.HandleLocalImages(mangaSlug, existingMedia.Path)
		}

		if cachedImageURL != "" {
			log.Debugf("Successfully set poster URL for media '%s': %s", mangaSlug, cachedImageURL)
			existingMedia.CoverArtURL = cachedImageURL
		} else {
			log.Warnf("No poster URL could be generated for media '%s' during metadata refresh", mangaSlug)
		}

		// Update metadata from provider while preserving creation date
		originalType := existingMedia.Type
		metadata.UpdateMedia(existingMedia, meta, existingMedia.CoverArtURL)

		// Check if the media is a webtoon by checking image dimensions, and overwrite type if detected
		detectedType := scheduler.DetectWebtoonFromImages(existingMedia.Path, existingMedia.Slug)
		if detectedType != "" {
			if originalType == "media" && detectedType == "webtoon" {
				log.Infof("Overriding media type from 'media' to 'webtoon' for '%s' based on image aspect ratio", existingMedia.Slug)
			}
			existingMedia.SetType(detectedType)
		}

		// Persist tags
		if len(meta.Tags) > 0 {
			if err := models.SetTagsForMedia(existingMedia.Slug, meta.Tags); err != nil {
				log.Warnf("Failed to persist tags for media '%s': %v", mangaSlug, err)
			}
		}

		// Update media metadata without changing created_at
		if err := models.UpdateMediaMetadata(existingMedia); err != nil {
			return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
		}
	} else {
		// No metadata match - update with local metadata
		log.Debugf("No metadata match found for '%s' from %s. Updating with local metadata.", existingMedia.Name, provider.Name())

		// Update name from path
		baseName := filepath.Base(existingMedia.Path)
		cleanedName := utils.RemovePatterns(baseName)
		if cleanedName != "" {
			existingMedia.Name = cleanedName
		}

		// Detect type from images
		detectedType := scheduler.DetectWebtoonFromImages(existingMedia.Path, existingMedia.Slug)
		if detectedType != "" {
			existingMedia.SetType(detectedType)
		}

		// Try to set poster from local images
		cachedImageURL, _ := scheduler.HandleLocalImages(existingMedia.Slug, existingMedia.Path)
		if cachedImageURL != "" {
			existingMedia.CoverArtURL = cachedImageURL
		}

		// Update media metadata without changing created_at
		if err := models.UpdateMediaMetadata(existingMedia); err != nil {
			return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
		}
	}

	// Re-index chapters (this will detect new/removed chapters without deleting the media)
	added, deleted, newChapterSlugs, _, err := scheduler.IndexChapters(existingMedia.Slug, existingMedia.Path, false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// If new chapters were added, notify users
	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(existingMedia.Slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in media '%s': %s", existingMedia.Slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		log.Infof("Refreshed metadata for media '%s' (added: %d, deleted: %d)", mangaSlug, added, deleted)
	} else {
		log.Infof("Metadata refresh complete for media '%s' (no chapter changes)", mangaSlug)
	}

	// Return success response for HTMX
	message := "Metadata refreshed successfully"
	if added > 0 || deleted > 0 {
		message = fmt.Sprintf("Metadata refreshed: %d chapters added, %d removed", added, deleted)
	}
	triggerNotification(c, message, "success")
	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}
