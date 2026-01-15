package handlers

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// getMediaPathFromChapters returns a representative path for a media by using the first chapter's path
func getMediaPathFromChapters(mediaSlug string) (string, error) {
	chapters, err := models.GetChapters(mediaSlug)
	if err != nil || len(chapters) == 0 {
		return "", fmt.Errorf("no chapters found for media '%s'", mediaSlug)
	}

	// Get the library for the first chapter
	library, err := models.GetLibrary(chapters[0].LibrarySlug)
	if err != nil {
		return "", fmt.Errorf("failed to get library '%s': %w", chapters[0].LibrarySlug, err)
	}

	// Use the first library folder as root
	if len(library.Folders) == 0 {
		return "", fmt.Errorf("library '%s' has no folders configured", chapters[0].LibrarySlug)
	}

	// Return the directory containing the first chapter
	return filepath.Dir(filepath.Join(library.Folders[0], chapters[0].File)), nil
}

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

	// Enhanced metadata fields
	Authors           string  `json:"authors" form:"authors"`
	Artists           string  `json:"artists" form:"artists"`
	StartDate         string  `json:"start_date" form:"start_date"`
	EndDate           string  `json:"end_date" form:"end_date"`
	ChapterCount      int     `json:"chapter_count" form:"chapter_count"`
	VolumeCount       int     `json:"volume_count" form:"volume_count"`
	AverageScore      float64 `json:"average_score" form:"average_score"`
	Popularity        int     `json:"popularity" form:"popularity"`
	Favorites         int     `json:"favorites" form:"favorites"`
	Demographic       string  `json:"demographic" form:"demographic"`
	Publisher         string  `json:"publisher" form:"publisher"`
	Magazine          string  `json:"magazine" form:"magazine"`
	Serialization     string  `json:"serialization" form:"serialization"`
	Genres            string  `json:"genres" form:"genres"`
	Characters        string  `json:"characters" form:"characters"`
	AlternativeTitles string  `json:"alternative_titles" form:"alternative_titles"`
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

	return handleView(c, views.UpdateMetadataResults(results, mangaSlug))
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

	// Get cover URL and download/store it
	coverURL := provider.GetCoverImageURL(meta)
	var storedImageURL string
	if coverURL != "" {
		storedImageURL, err = scheduler.DownloadAndStoreImage(existingMedia.Slug, coverURL)
		if err != nil {
			log.Warnf("Failed to download cover art: %v", err)
			// Try local images as fallback
			mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
			if pathErr == nil {
				storedImageURL, _ = scheduler.HandleLocalImages(existingMedia.Slug, mediaPath)
			}
		}
	}

	// Update media with metadata
	originalType := existingMedia.Type
	metadata.UpdateMedia(existingMedia, meta, storedImageURL)

	// Check if the media is a webtoon by checking image dimensions, and overwrite type if detected
	mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
	detectedType := ""
	if pathErr == nil {
		detectedType = scheduler.DetectWebtoonFromImages(mediaPath, existingMedia.Slug)
	}
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
	for tag := range strings.SplitSeq(formData.Tags, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if err := models.SetTagsForMedia(existingMedia.Slug, tags); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Process authors (comma-separated list, format: "Name (Role)" or "Name")
	var authors []models.AuthorInfo
	for author := range strings.SplitSeq(formData.Authors, ",") {
		trimmed := strings.TrimSpace(author)
		if trimmed != "" {
			// Parse "Name (Role)" format
			if strings.Contains(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
				parts := strings.SplitN(trimmed, " (", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					role := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")
					authors = append(authors, models.AuthorInfo{Name: name, Role: role})
				}
			} else {
				authors = append(authors, models.AuthorInfo{Name: trimmed, Role: "author"})
			}
		}
	}
	existingMedia.Authors = authors

	// Process artists (comma-separated list, format: "Name (Role)" or "Name")
	var artists []models.AuthorInfo
	for artist := range strings.SplitSeq(formData.Artists, ",") {
		trimmed := strings.TrimSpace(artist)
		if trimmed != "" {
			// Parse "Name (Role)" format
			if strings.Contains(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
				parts := strings.SplitN(trimmed, " (", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					role := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")
					artists = append(artists, models.AuthorInfo{Name: name, Role: role})
				}
			} else {
				artists = append(artists, models.AuthorInfo{Name: trimmed, Role: "artist"})
			}
		}
	}
	existingMedia.Artists = artists

	// Process genres (comma-separated list)
	var genres []string
	for genre := range strings.SplitSeq(formData.Genres, ",") {
		trimmed := strings.TrimSpace(genre)
		if trimmed != "" {
			genres = append(genres, trimmed)
		}
	}
	existingMedia.Genres = genres

	// Process characters (comma-separated list)
	var characters []string
	for character := range strings.SplitSeq(formData.Characters, ",") {
		trimmed := strings.TrimSpace(character)
		if trimmed != "" {
			characters = append(characters, trimmed)
		}
	}
	existingMedia.Characters = characters

	// Process alternative titles (comma-separated list)
	var alternativeTitles []string
	for title := range strings.SplitSeq(formData.AlternativeTitles, ",") {
		trimmed := strings.TrimSpace(title)
		if trimmed != "" {
			alternativeTitles = append(alternativeTitles, trimmed)
		}
	}
	existingMedia.AlternativeTitles = alternativeTitles

	// Update other enhanced fields
	existingMedia.StartDate = formData.StartDate
	existingMedia.EndDate = formData.EndDate
	existingMedia.ChapterCount = formData.ChapterCount
	existingMedia.VolumeCount = formData.VolumeCount
	existingMedia.AverageScore = formData.AverageScore
	existingMedia.Popularity = formData.Popularity
	existingMedia.Favorites = formData.Favorites
	existingMedia.Demographic = formData.Demographic
	existingMedia.Publisher = formData.Publisher
	existingMedia.Magazine = formData.Magazine
	existingMedia.Serialization = formData.Serialization

	// Process cover art URL (download and store)
	if formData.CoverURL != "" {
		storedImageURL, err := scheduler.DownloadAndStoreImage(existingMedia.Slug, formData.CoverURL)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		existingMedia.CoverArtURL = storedImageURL
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

	// Find the library that contains this media
	mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
	if pathErr != nil {
		return sendInternalServerError(c, "Could not determine media path", pathErr)
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	var librarySlug string
	for _, lib := range libraries {
		for _, folder := range lib.Folders {
			if strings.HasPrefix(mediaPath, folder) {
				librarySlug = lib.Slug
				break
			}
		}
		if librarySlug != "" {
			break
		}
	}
	if librarySlug == "" {
		return sendInternalServerError(c, "Could not determine library for media", fmt.Errorf("no library found for path %s", mediaPath))
	}

	// Get the library to find the root path
	library, err := models.GetLibrary(librarySlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if len(library.Folders) == 0 {
		return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("library has no folders"))
	}

	// Find the root path that contains the media path
	var rootPath string
	for _, folder := range library.Folders {
		if strings.HasPrefix(mediaPath, folder) {
			rootPath = folder
			break
		}
	}
	if rootPath == "" {
		return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("could not find root path for %s in library %s", mediaPath, librarySlug))
	}

	// Re-index chapters (this will detect new/removed chapters without deleting the media)
	added, deleted, newChapterSlugs, _, err := scheduler.IndexChapters(existingMedia.Slug, mediaPath, librarySlug, false)
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

	// Fetch fresh aggregated metadata from all providers
	aggregatedMeta, err := metadata.QueryAllProviders(existingMedia.Name)
	if err != nil {
		// Log the warning but don't fail - fall back to local metadata
		log.Warnf("Failed to fetch aggregated metadata for '%s': %v. Falling back to local metadata.", existingMedia.Name, err)
	}

	if aggregatedMeta != nil {
		// Get the cover art URL from the aggregated metadata
		var coverURL string
		if len(aggregatedMeta.CoverArtURLs) > 0 {
			coverURL = aggregatedMeta.CoverArtURLs[0] // Use first cover URL
		}

		// Download and store the new cover art if available
		var storedImageURL string
		if coverURL != "" {
			log.Debugf("Attempting to download cover art from aggregated metadata for media '%s': %s", mangaSlug, coverURL)
			storedImageURL, err = scheduler.DownloadAndStoreImage(mangaSlug, coverURL)
			if err != nil {
				log.Warnf("Failed to download cover art during metadata refresh: %v", err)
				// Try to fall back to local images
				log.Debugf("Falling back to local images for poster generation for media '%s'", mangaSlug)
				mediaPath, pathErr := getMediaPathFromChapters(mangaSlug)
				if pathErr == nil {
					storedImageURL, _ = scheduler.HandleLocalImages(mangaSlug, mediaPath)
				}
			}
		} else {
			// No cover URL from aggregated metadata, try local images
			log.Debugf("No cover URL from aggregated metadata for media '%s', trying local images", mangaSlug)
			mediaPath, pathErr := getMediaPathFromChapters(mangaSlug)
			if pathErr == nil {
				storedImageURL, _ = scheduler.HandleLocalImages(mangaSlug, mediaPath)
			}
		}

		if storedImageURL != "" {
			log.Debugf("Successfully set poster URL for media '%s': %s", mangaSlug, storedImageURL)
			existingMedia.CoverArtURL = storedImageURL
		} else {
			log.Warnf("No poster URL could be generated for media '%s' during metadata refresh", mangaSlug)
		}

		// Update metadata from aggregated metadata while preserving creation date
		originalType := existingMedia.Type
		metadata.UpdateMediaFromAggregated(existingMedia, aggregatedMeta, existingMedia.CoverArtURL)

		// Fetch potential poster URLs from all metadata providers
		var allPosterURLs []string
		providerNames := metadata.ListProviders()

		for _, providerName := range providerNames {
			provider, err := metadata.GetProvider(providerName, "")
			if err != nil {
				log.Debugf("Skipping provider %s for potential posters: %v", providerName, err)
				continue
			}

			results, err := provider.Search(existingMedia.Name)
			if err != nil {
				log.Debugf("Provider %s search failed for potential posters: %v", providerName, err)
				continue
			}

			// Filter results by similarity score >= 0.9 and collect URLs
			for _, result := range results {
				if result.SimilarityScore >= 0.9 && result.CoverArtURL != "" {
					allPosterURLs = append(allPosterURLs, result.CoverArtURL)
				}
			}
		}

		// Remove duplicates and limit to reasonable number
		uniqueURLs := make(map[string]bool)
		var uniquePosterURLs []string
		for _, url := range allPosterURLs {
			if !uniqueURLs[url] {
				uniqueURLs[url] = true
				uniquePosterURLs = append(uniquePosterURLs, url)
			}
		}

		// Limit to top 20 to keep database size reasonable
		if len(uniquePosterURLs) > 20 {
			uniquePosterURLs = uniquePosterURLs[:20]
		}

		existingMedia.PotentialPosterURLs = uniquePosterURLs
		log.Debugf("Saved %d potential poster URLs from all providers for media '%s'", len(uniquePosterURLs), mangaSlug)

		// Check if the media is a webtoon by checking image dimensions, and overwrite type if detected
		mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
		detectedType := ""
		if pathErr == nil {
			detectedType = scheduler.DetectWebtoonFromImages(mediaPath, existingMedia.Slug)
		}
		if detectedType != "" {
			if originalType == "media" && detectedType == "webtoon" {
				log.Infof("Overriding media type from 'media' to 'webtoon' for '%s' based on image aspect ratio", existingMedia.Slug)
			}
			existingMedia.SetType(detectedType)
		}

		// Persist tags
		if len(aggregatedMeta.Tags) > 0 {
			if err := models.SetTagsForMedia(existingMedia.Slug, aggregatedMeta.Tags); err != nil {
				log.Warnf("Failed to persist tags for media '%s': %v", mangaSlug, err)
			}
		}

		// Update media metadata without changing created_at
		if err := models.UpdateMediaMetadata(existingMedia); err != nil {
			return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
		}
	} else {
		// No metadata match - update with local metadata
		log.Debugf("No aggregated metadata match found for '%s'. Updating with local metadata.", existingMedia.Name)

		// Try to detect type from images
		mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
		if pathErr == nil {
			detectedType := scheduler.DetectWebtoonFromImages(mediaPath, existingMedia.Slug)
			if detectedType != "" {
				existingMedia.SetType(detectedType)
			}

			// Try to set poster from local images
			storedImageURL, _ := scheduler.HandleLocalImages(existingMedia.Slug, mediaPath)
			if storedImageURL != "" {
				existingMedia.CoverArtURL = storedImageURL
			}
		}

		// Update media metadata without changing created_at
		if err := models.UpdateMediaMetadata(existingMedia); err != nil {
			return sendInternalServerError(c, ErrMetadataUpdateFailed, err)
		}
	}

	// Find the library that contains this media
	mediaPath, pathErr := getMediaPathFromChapters(existingMedia.Slug)
	if pathErr != nil {
		return sendInternalServerError(c, "Could not determine media path", pathErr)
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	var librarySlug string
	for _, lib := range libraries {
		for _, folder := range lib.Folders {
			if strings.HasPrefix(mediaPath, folder) {
				librarySlug = lib.Slug
				break
			}
		}
		if librarySlug != "" {
			break
		}
	}
	if librarySlug == "" {
		return sendInternalServerError(c, "Could not determine library for media", fmt.Errorf("no library found for path %s", mediaPath))
	}

	// Re-index chapters (this will detect new/removed chapters without deleting the media)
	added, deleted, newChapterSlugs, _, err := scheduler.IndexChapters(existingMedia.Slug, mediaPath, librarySlug, false)
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
