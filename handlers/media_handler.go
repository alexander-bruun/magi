package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/nwaples/rardecode"
)

const (
	defaultPage     = 1
	defaultPageSize = 16
	searchPageSize  = 10
)

// getFirstChapterFilePath returns the path to the first chapter file (.cbz, .cbr, etc.)
// from a media directory. Returns error if no chapters or archive files are found.
func getFirstChapterFilePath(media *models.Media) (string, error) {
	chapters, err := models.GetChapters(media.Slug)
	if err != nil || len(chapters) == 0 {
		return "", fmt.Errorf("no chapters found")
	}

	// Try to construct path from first chapter slug
	chapterPath := filepath.Join(media.Path, chapters[0].Slug+".cbz")
	if _, err := os.Stat(chapterPath); err == nil {
		return chapterPath, nil
	}

	// Fallback: search directory for first archive file
	entries, err := os.ReadDir(media.Path)
	if err != nil {
		return "", fmt.Errorf("cannot access media directory: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no files found in media directory")
	}

	// Find first archive file
	for _, entry := range entries {
		if !entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, ".cbz") ||
				strings.HasSuffix(name, ".cbr") ||
				strings.HasSuffix(name, ".zip") ||
				strings.HasSuffix(name, ".rar") {
				return filepath.Join(media.Path, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("no archive files found in media directory")
}

// HandleMedias lists media with filtering, sorting, and HTMX fragment support.
func HandleMedias(c *fiber.Ctx) error {
	params := ParseQueryParams(c)

	cfg, err := models.GetAppConfig()
	if err != nil {
		return handleError(c, err)
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return handleError(c, err)
	}

	// Search media using options (supports tags, tagMode, and types)
	opts := models.SearchOptions{
		Filter:              params.SearchFilter,
		Page:                params.Page,
		PageSize:            defaultPageSize,
		SortBy:              params.Sort,
		SortOrder:           params.Order,
		LibrarySlug:         params.LibrarySlug,
		Tags:                params.Tags,
		TagMode:             params.TagMode,
		Types:               params.Types,
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  cfg.ContentRatingLimit,
	}
	media, count, err := models.SearchMediasWithOptions(opts)

	if err != nil {
		return handleError(c, err)
	}

	totalPages := CalculateTotalPages(count, defaultPageSize)

	// Fetch all known tags for the dropdown
	allTags, err := models.GetAllTags()
	if err != nil {
		return handleError(c, err)
	}
	// Fetch all known types for the new types dropdown
	allTypes, err := models.GetAllMediaTypes()
	if err != nil {
		return handleError(c, err)
	}

	// If HTMX request targeting the listing results container, render just the listing fragment
	if IsHTMXRequest(c) {
		target := GetHTMXTarget(c)
		if target == "media-listing" {
			return HandleView(c, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				_, err := w.Write([]byte(`<div id="media-listing">`))
				if err != nil {
					return err
				}
				err = views.GenericMediaListingWithTypes("/series", "media-listing", true, media, params.Page, totalPages, params.Sort, params.Order, "No media have been indexed yet.", params.Tags, params.TagMode, allTags, params.Types, allTypes, params.SearchFilter).Render(ctx, w)
				if err != nil {
					return err
				}
				_, err = w.Write([]byte(`</div>`))
				return err
			}))
		} else if target == "media-listing-results" {
			path := "/series"
			targetID := "media-listing-results"
			emptyMessage := "No media have been indexed yet."
			return HandleView(c, views.MediaListingFragment(
				media, 
				params.Page, 
				totalPages, 
				params.Sort, 
				params.Order, 
				emptyMessage, 
				path, 
				targetID, 
				params.Tags, 
				params.TagMode, 
				params.Types, 
				params.SearchFilter,
			))
		}
	}

	return HandleView(c, views.MediasWithTypes(media, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.Types, allTypes, params.SearchFilter))
}

// HandleMedia renders a media detail page including chapters and per-user state.
func HandleMedia(c *fiber.Ctx) error {
	slug := c.Params("media")
	media, err := models.GetMedia(slug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, media.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this media"), fiber.StatusForbidden)
	}

	chapters, err := models.GetChapters(slug)
	if err != nil {
		return handleError(c, err)
	}

	// Precompute first/last chapter slugs before reversing
	firstSlug, lastSlug := models.GetFirstAndLastChapterSlugs(chapters)

	reverse := c.Query("reverse") == "true"
	if reverse {
		slices.Reverse(chapters)
	}
	
	// Get user role for conditional rendering
	userRole := ""
	userName := GetUserContext(c)
	lastReadChapterSlug := ""
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			userRole = user.Role
		}
		// If a user is logged in, fetch their read chapters and annotate the list
		readMap, err := models.GetReadChaptersForUser(userName, slug)
		if err == nil {
			for i := range chapters {
				chapters[i].Read = readMap[chapters[i].Slug]
			}
		}
		// Fetch the last read chapter for the resume button
		lastReadChapter, err := models.GetLastReadChapter(userName, slug)
		if err == nil {
			lastReadChapterSlug = lastReadChapter
		}
	}
		
	if IsHTMXRequest(c) && c.Query("reverse") != "" {
		return HandleView(c, views.MediaChaptersSection(*media, chapters, reverse, lastReadChapterSlug))
	}
	
	if IsHTMXRequest(c) {
		return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse))
	}
	
	return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse))
}

// HandleChapter shows a chapter reader with navigation and optional read tracking.
func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := string([]byte(c.Params("media")))
	chapterSlug := string([]byte(c.Params("chapter")))

	// Validate media slug to prevent malformed URLs
	if strings.ContainsAny(mangaSlug, "/,") {
		return handleErrorWithStatus(c, fmt.Errorf("invalid media slug"), fiber.StatusBadRequest)
	}

	media, chapters, err := models.GetMediaAndChapters(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, media.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}
	if chapter == nil {
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
	}

	prevSlug, nextSlug, err := models.GetAdjacentChapters(chapter.Slug, mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	// If media type is novel, handle as EPUB
	if media.Type == "novel" {
		// Determine the actual chapter file path
		chapterFilePath := media.Path
		if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
			chapterFilePath = filepath.Join(media.Path, chapter.File)
		}

		// Check if the file exists
		if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
		}

		// Get TOC and content
		toc := utils.GetTOC(chapterFilePath)
		content := utils.GetBookContent(chapterFilePath, mangaSlug, chapterSlug)

		// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
		rev := make([]models.Chapter, len(chapters))
		for i := range chapters {
			rev[i] = chapters[len(chapters)-1-i]
		}

		return HandleView(c, views.NovelChapter(prevSlug, chapter.Slug, nextSlug, *media, *chapter, rev, toc, content))
	}

	// Note: chapter is normally marked read by an HTMX trigger in the view.
	// As a safe fallback, if this request is a full page load (not an HTMX request)
	// and the user is logged in, mark the chapter read server-side so the
	// media list can reflect the read state for non-HTMX navigation.
	if userName := GetUserContext(c); userName != "" && !IsHTMXRequest(c) {
		_ = models.MarkChapterRead(userName, mangaSlug, chapterSlug)
	}

	images, err := models.GetChapterImages(media, chapter)
	if err != nil {
		return handleError(c, err)
	}

	// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
	rev := make([]models.Chapter, len(chapters))
	for i := range chapters {
		rev[i] = chapters[len(chapters)-1-i]
	}
	return HandleView(c, views.Chapter(prevSlug, chapter.Slug, nextSlug, *media, images, *chapter, rev))
}

// HandleMediaChapterTOC handles TOC requests for media chapters
func HandleMediaChapterTOC(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Get media to construct full path
	media, err := models.GetMedia(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Media not found")
	}

	// Determine the actual chapter file path
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	toc := utils.GetTOC(chapterFilePath)
	c.Set("Content-Type", "text/html")
	return c.SendString(toc)
}

// HandleMediaChapterContent handles book content requests for media chapters
func HandleMediaChapterContent(c *fiber.Ctx) error {
	mangaSlug := string([]byte(c.Params("media")))
	chapterSlug := string([]byte(c.Params("chapter")))

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Get media to construct full path
	media, err := models.GetMedia(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Media not found")
	}

	// Determine the actual chapter file path
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	content := utils.GetBookContent(chapterFilePath, mangaSlug, chapterSlug)

	c.Set("Content-Type", "text/html")
	return c.SendString(content)
}

// HandleMediaChapterAsset handles asset requests from EPUB files with token validation
func HandleMediaChapterAsset(c *fiber.Ctx) error {
	token := c.Query("token")
	
	if token == "" {
		return handleErrorWithStatus(c, fmt.Errorf("token parameter is required"), fiber.StatusBadRequest)
	}

	// Validate and consume the token
	tokenInfo, err := utils.ValidateAndConsumeImageToken(token)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid or expired token: %w", err), fiber.StatusForbidden)
	}

	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	assetPath := c.Params("*")

	log.Debugf("Asset request: media=%s, chapter=%s, assetPath=%s", mangaSlug, chapterSlug, assetPath)

	// Verify token matches the requested resource
	if tokenInfo.MediaSlug != mangaSlug || tokenInfo.ChapterSlug != chapterSlug {
		return handleErrorWithStatus(c, fmt.Errorf("token does not match requested resource"), fiber.StatusForbidden)
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		log.Errorf("Chapter not found: %s/%s", mangaSlug, chapterSlug)
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Get media to construct full path
	media, err := models.GetMedia(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Media not found")
	}

	// Determine the actual chapter file path
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

	log.Debugf("Chapter file path: %s", chapterFilePath)

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapterFilePath)
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapterFilePath)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapterFilePath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening EPUB")
	}
	defer r.Close()

	// Get the OPF directory
	opfDir, err := utils.GetOPFDir(chapterFilePath)
	if err != nil {
		log.Errorf("Error getting OPF dir for %s: %v", chapterFilePath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error parsing EPUB")
	}

	// Block serving CSS files
	if strings.ToLower(filepath.Ext(assetPath)) == ".css" {
		log.Debugf("Blocking CSS asset request: %s", assetPath)
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	// Find the asset
	assetFullPath := filepath.Join(opfDir, assetPath)
	var file *zip.File
	for _, f := range r.File {
		if f.Name == assetFullPath {
			file = f
			break
		}
	}
	if file == nil {
		log.Errorf("Asset not found in EPUB: %s (tried %s)", assetPath, assetFullPath)
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening asset")
	}
	defer rc.Close()

	// Read the asset data
	assetData, err := io.ReadAll(rc)
	if err != nil {
		log.Errorf("Error reading asset %s: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading asset")
	}

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".css":
		c.Set("Content-Type", "text/css")
	case ".xhtml", ".html":
		c.Set("Content-Type", "text/html")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	// For image assets, apply compression based on user role
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" {
		// Get user role for compression quality
		userName, _ := c.Locals("user_name").(string)
		var quality int
		if userName != "" {
			user, err := models.FindUserByUsername(userName)
			if err == nil && user != nil {
				quality = models.GetCompressionQualityForRole(user.Role)
			} else {
				quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
			}
		} else {
			quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
		}

		// Decode the image
		imageReader := bytes.NewReader(assetData)
		img, _, err := image.Decode(imageReader)
		if err != nil {
			// If decoding fails, serve original data
			log.Debugf("Serving asset %s (original, decode failed)", assetPath)
			return c.Send(assetData)
		}

		// Encode all images as JPEG for better performance and consistent compression
		var buf bytes.Buffer
		// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
		jpegQuality := quality
		if jpegQuality < 1 {
			jpegQuality = 1
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
		if err != nil {
			// If encoding fails, serve original data
			log.Debugf("Serving asset %s (original, encode failed)", assetPath)
			return c.Send(assetData)
		}
		log.Debugf("Serving asset %s (compressed)", assetPath)
		return c.Send(buf.Bytes())
	} else {
		// For non-image assets, serve original data
		log.Debugf("Serving asset %s", assetPath)
		return c.Send(assetData)
	}
}

// HandleMarkRead marks a chapter as read for the logged-in user via HTMX
func HandleMarkRead(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.MarkChapterRead(userName, mangaSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// Return the inline eye toggle fragment so HTMX will swap the icon in-place.
	return HandleView(c, views.InlineEyeToggle(true, mangaSlug, chapterSlug))
}

// HandleMarkUnread unmarks a chapter as read for the logged-in user via HTMX
func HandleMarkUnread(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.UnmarkChapterRead(userName, mangaSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// Return the inline eye toggle fragment with read=false so HTMX swaps to the closed-eye.
	return HandleView(c, views.InlineEyeToggle(false, mangaSlug, chapterSlug))
}

// HandleUpdateMetadataMedia displays search results for updating a local media's metadata.
func HandleUpdateMetadataMedia(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	search := c.Query("search")

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		return handleError(c, fmt.Errorf("failed to get app config: %w", err))
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to get metadata provider: %w", err))
	}

	// Search using the provider
	results, err := provider.Search(search)
	if err != nil {
		return handleError(c, err)
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
		return handleError(c, fmt.Errorf("media slug can't be empty"))
	}

	existingMedia, err := models.GetMedia(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingMedia == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted"), fiber.StatusNotFound)
	}

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		return handleError(c, fmt.Errorf("failed to get app config: %w", err))
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to get metadata provider: %w", err))
	}

	// Fetch metadata using the provider
	meta, err := provider.GetMetadata(metadataID)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to fetch metadata: %w", err))
	}

	// Get cover URL and download/cache it
	coverURL := provider.GetCoverImageURL(meta)
	var cachedImageURL string
	if coverURL != "" {
		cachedImageURL, err = indexer.DownloadAndCacheImage(existingMedia.Slug, coverURL)
		if err != nil {
			log.Warnf("Failed to download cover art: %v", err)
			// Try local images as fallback
			cachedImageURL, _ = indexer.HandleLocalImages(existingMedia.Slug, existingMedia.Path)
		}
	}

	// Update media with metadata
	originalType := existingMedia.Type
	metadata.UpdateMedia(existingMedia, meta, cachedImageURL)

	// Check if the media is a webtoon by checking image dimensions, and overwrite type if detected
	detectedType := indexer.DetectWebtoonFromImages(existingMedia.Path, existingMedia.Slug)
	if detectedType != "" {
		if originalType == "media" && detectedType == "webtoon" {
			log.Infof("Overriding media type from 'media' to 'webtoon' for '%s' based on image aspect ratio", existingMedia.Slug)
		}
		existingMedia.SetType(detectedType)
	}

	// Persist tags
	if len(meta.Tags) > 0 {
		if err := models.SetTagsForMedia(existingMedia.Slug, meta.Tags); err != nil {
			log.Warnf("Failed to persist tags: %v", err)
		}
	}

	if err := models.UpdateMedia(existingMedia); err != nil {
		return handleError(c, err)
	}

	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleMediaSearch returns search results for the quick-search panel.
func HandleMediaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return handleError(c, err)
	}

	opts := models.SearchOptions{
		Filter:              searchParam,
		Page:                defaultPage,
		PageSize:            searchPageSize,
		SortBy:              "name",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
	}
	media, _, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return handleError(c, err)
	}

	if len(media) == 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMedias(media))
}

// HandleTags returns a JSON array of all known tags for client-side consumption
func HandleTags(c *fiber.Ctx) error {
	tags, err := models.GetAllTags()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tags)
}

// HandleTagsFragment returns an HTMX-ready fragment with tag checkboxes
func HandleTagsFragment(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the main series page
	if !IsHTMXRequest(c) {
		return c.Redirect("/series")
	}

	tags, err := models.GetAllTags()
	if err != nil {
		return handleError(c, err)
	}

	// Determine currently selected tags from the query (support repeated and comma-separated)
	var selectedTags []string
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		if valsMap, err := url.ParseQuery(raw); err == nil {
			if vals, ok := valsMap["tags"]; ok {
				for _, v := range vals {
					for _, t := range strings.Split(v, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							selectedTags = append(selectedTags, t)
						}
					}
				}
			}
		}
	}
	// Render fragment directly without layout wrapper
	return HandleView(c, views.TagsFragment(tags, selectedTags))
}

// templEscape provides a minimal HTML escape for values inserted into the fragment
func templEscape(s string) string {
	r := s
	r = strings.ReplaceAll(r, "&", "&amp;")
	r = strings.ReplaceAll(r, "<", "&lt;")
	r = strings.ReplaceAll(r, ">", "&gt;")
	r = strings.ReplaceAll(r, "\"", "&quot;")
	return r
}

// HandleMediaVote handles a user's upvote/downvote for a media via HTMX.
// Expected form values: "value" = "1" or "-1". User must be authenticated.
func HandleMediaVote(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// parse value
	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}
	v, err := strconv.Atoi(valStr)
	if err != nil {
		return fiber.ErrBadRequest
	}

	// If value == 0, remove vote
	if v == 0 {
		if err := models.RemoveVote(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetVote(userName, mangaSlug, v); err != nil {
			return handleError(c, err)
		}
	}

	// Return updated fragment so HTMX can refresh the vote UI in-place.
	score, up, down, err := models.GetMediaVotes(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	userVote, _ := models.GetUserVoteForMedia(userName, mangaSlug)
	return HandleView(c, views.MediaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMediaVoteFragment returns the vote UI fragment for a media. If user is logged in,
// it will show their current selection highlighted.
func HandleMediaVoteFragment(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !IsHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	score, up, down, err := models.GetMediaVotes(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	userVote := 0
	if userName != "" {
		v, _ := models.GetUserVoteForMedia(userName, mangaSlug)
		userVote = v
	}
	return HandleView(c, views.MediaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMediaFavorite handles toggling a favorite for the logged-in user via HTMX.
// Expected form values: "value" = "1" to favorite or "0" to unfavorite.
func HandleMediaFavorite(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	if valStr == "0" {
		if err := models.RemoveFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	}

	// Return updated fragment so HTMX can refresh the favorite UI in-place.
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	isFav, _ := models.IsFavoriteForUser(userName, mangaSlug)
	return HandleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleMediaFavoriteFragment returns the favorite UI fragment for a media.
func HandleMediaFavoriteFragment(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !IsHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	isFav := false
	if userName != "" {
		f, _ := models.IsFavoriteForUser(userName, mangaSlug)
		isFav = f
	}
	return HandleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleManualEditMetadata handles manual metadata updates by moderators or admins
func HandleManualEditMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("media slug can't be empty"))
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingMedia == nil {
		return handleError(c, fmt.Errorf("media not found"))
	}

	// Parse form values
	name := c.FormValue("name")
	author := c.FormValue("author")
	description := c.FormValue("description")
	year := c.FormValue("year")
	originalLanguage := c.FormValue("original_language")
	mangaType := c.FormValue("manga_type")
	status := c.FormValue("status")
	contentRating := c.FormValue("content_rating")
	tagsInput := c.FormValue("tags")
	coverURL := c.FormValue("cover_url")

	// Update fields
	existingMedia.Name = name
	existingMedia.Author = author
	existingMedia.Description = description
	if year != "" {
		if yearInt, err := strconv.Atoi(year); err == nil {
			existingMedia.Year = yearInt
		}
	} else {
		existingMedia.Year = 0
	}
	existingMedia.OriginalLanguage = originalLanguage
	if mangaType != "" {
		existingMedia.Type = mangaType
	}
	if status != "" {
		existingMedia.Status = status
	}
	if contentRating != "" {
		existingMedia.ContentRating = contentRating
	}

	// Process tags (comma-separated list)
	var tags []string
	for _, tag := range strings.Split(tagsInput, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if err := models.SetTagsForMedia(existingMedia.Slug, tags); err != nil {
		return handleError(c, fmt.Errorf("failed to update tags: %w", err))
	}

	// Process cover art URL (download and cache)
	if coverURL != "" {
		cachedImageURL, err := indexer.DownloadAndCacheImage(existingMedia.Slug, coverURL)
		if err != nil {
			return handleError(c, fmt.Errorf("failed to download and cache cover art: %w", err))
		}
		existingMedia.CoverArtURL = cachedImageURL
	}

	// Update media in database
	if err := models.UpdateMedia(existingMedia); err != nil {
		return handleError(c, err)
	}

	// Return success response for HTMX
	redirectURL := fmt.Sprintf("/series/%s", existingMedia.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleRefreshMetadata refreshes media metadata and chapters without resetting creation date
func HandleRefreshMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("media slug can't be empty"))
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingMedia == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found"), fiber.StatusNotFound)
	}

	// Get the configured metadata provider
	config, err := models.GetAppConfig()
	if err != nil {
		log.Warnf("Failed to get app config: %v", err)
		return handleError(c, err)
	}
	provider, err := metadata.GetProviderFromConfig(&config)
	if err != nil {
		log.Warnf("Failed to get metadata provider: %v", err)
		return handleError(c, err)
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
			cachedImageURL, err = indexer.DownloadAndCacheImage(mangaSlug, coverURL)
			if err != nil {
				log.Warnf("Failed to download cover art during metadata refresh: %v", err)
				// Try to fall back to local images
				log.Debugf("Falling back to local images for poster generation for media '%s'", mangaSlug)
				cachedImageURL, _ = indexer.HandleLocalImages(mangaSlug, existingMedia.Path)
			}
		} else {
			// No cover URL from provider, try local images
			log.Debugf("No cover URL from provider for media '%s', trying local images", mangaSlug)
			cachedImageURL, _ = indexer.HandleLocalImages(mangaSlug, existingMedia.Path)
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
		detectedType := indexer.DetectWebtoonFromImages(existingMedia.Path, existingMedia.Slug)
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
			return handleError(c, fmt.Errorf("failed to update media metadata: %w", err))
		}
	} else {
		// No metadata match - delete and re-index with local metadata
		log.Debugf("No metadata match found for '%s' from %s. Re-indexing with local metadata and poster generation.", existingMedia.Name, provider.Name())
		
		// Delete the media (chapters and tags will be cascade deleted)
		if err := models.DeleteMedia(existingMedia.Slug); err != nil {
			log.Warnf("Failed to delete media '%s' for re-indexing: %v", mangaSlug, err)
			return handleError(c, err)
		}
		
		// Re-index using the standard indexer to get local metadata
		if _, err := indexer.IndexMedia(existingMedia.Path, existingMedia.LibrarySlug); err != nil {
			log.Warnf("Failed to re-index media '%s' with local metadata: %v", mangaSlug, err)
			return handleError(c, err)
		}
		
		redirectURL := fmt.Sprintf("/series/%s", mangaSlug)
		c.Set("HX-Redirect", redirectURL)
		return c.SendStatus(fiber.StatusOK)
	}

	// Re-index chapters (this will detect new/removed chapters without deleting the media)
	added, deleted, newChapterSlugs, _, err := indexer.IndexChapters(existingMedia.Slug, existingMedia.Path, false)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to index chapters: %w", err))
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
	redirectURL := fmt.Sprintf("/series/%s", mangaSlug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandlePosterChapterSelect renders a list of chapters to select from
func HandlePosterChapterSelect(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	
	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return handleError(c, fmt.Errorf("media not found"))
	}

	// Get all chapters
	chapters, err := models.GetChapters(mangaSlug)
	if err != nil || len(chapters) == 0 {
		return HandleView(c, views.EmptyState("No chapters found."))
	}

	return HandleView(c, views.PosterEditor(mangaSlug, chapters, "", 0, -1, "", 1))
}

// HandlePosterSelector renders the image selector for a chapter
func HandlePosterSelector(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Query("chapter", "")
	
	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return handleError(c, fmt.Errorf("media not found"))
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return HandleView(c, views.EmptyState(fmt.Sprintf("Error: %v", err)))
	}

	var chapterPage int
	chapterPageStr := c.Query("page", "1")
	if p, err := strconv.Atoi(chapterPageStr); err == nil {
		chapterPage = p
	} else {
		chapterPage = 1
	}
	if chapterSlug != "" {
		for i, ch := range chapters {
			if ch.Slug == chapterSlug {
				chapterPage = (i / 10) + 1
				break
			}
		}
	}

	// Get chapter file path
	var chapterPath string
	if chapterSlug != "" {
		// Look up chapter by slug to get the actual file
		chapter, err := models.GetChapter(mangaSlug, chapterSlug)
		if err != nil {
			return handleError(c, err)
		}
		if chapter == nil {
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		if chapter.File == "" {
			return HandleView(c, views.EmptyState(fmt.Sprintf("Error: chapter file not found")))
		}
		chapterPath = filepath.Join(media.Path, chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return HandleView(c, views.EmptyState(fmt.Sprintf("Error: %v", err)))
		}
	}

	// Get count of images in the chapter file
	imageCount, err := utils.CountImageFiles(chapterPath)
	if err != nil {
		return HandleView(c, views.EmptyState(fmt.Sprintf("Error counting images: %v", err)))
	}
	if imageCount == 0 {
		return HandleView(c, views.EmptyState("No images found in the chapter."))
	}

	return HandleView(c, views.PosterEditor(mangaSlug, chapters, chapterSlug, imageCount, -1, "", chapterPage))
}

// HandlePosterPreview renders a preview of a selected image with crop selector
func HandlePosterPreview(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Query("chapter", "")
	imageIndexStr := c.Query("index", "0")
	
	imageIndex := 0
	if idx, err := strconv.Atoi(imageIndexStr); err == nil {
		imageIndex = idx
	}

	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return handleError(c, fmt.Errorf("media not found"))
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return handleError(c, fmt.Errorf("error getting chapters: %v", err))
	}

	var chapterPage int
	chapterPageStr := c.Query("page", "1")
	if p, err := strconv.Atoi(chapterPageStr); err == nil {
		chapterPage = p
	} else {
		chapterPage = 1
	}
	for i, ch := range chapters {
		if ch.Slug == chapterSlug {
			chapterPage = (i / 10) + 1
			break
		}
	}

	// Get chapter file path
	var chapterPath string
	if chapterSlug != "" {
		// Look up chapter by slug to get the actual file
		chapter, err := models.GetChapter(mangaSlug, chapterSlug)
		if err != nil {
			return handleError(c, err)
		}
		if chapter == nil {
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		if chapter.File == "" {
			return handleErrorWithStatus(c, fmt.Errorf("chapter file not found"), fiber.StatusNotFound)
		}
		chapterPath = filepath.Join(media.Path, chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return handleError(c, fmt.Errorf("error: %v", err))
		}
	}

	// Extract and get the image data URI
	imageDataURI, err := utils.GetImageDataURIByIndex(chapterPath, imageIndex)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to load image: %w", err))
	}

	imageCount, err := utils.CountImageFiles(chapterPath)
	if err != nil {
		return handleError(c, fmt.Errorf("error counting images: %v", err))
	}

	return HandleView(c, views.PosterEditor(mangaSlug, chapters, chapterSlug, imageCount, imageIndex, imageDataURI, chapterPage))
}

// HandlePosterSet sets a custom poster image based on user selection or upload
func HandlePosterSet(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")

	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return handleError(c, fmt.Errorf("media not found"))
	}

	// Check for file upload
	if file, err := c.FormFile("poster"); err == nil {
		// Handle upload
		ext := filepath.Ext(file.Filename)
		cacheDir := utils.GetCacheDirectory()
		cachedPath := filepath.Join(cacheDir, fmt.Sprintf("%s%s", mangaSlug, ext))
		if err := c.SaveFile(file, cachedPath); err != nil {
			return handleError(c, fmt.Errorf("failed to save uploaded file: %w", err))
		}
		cachedImageURL := fmt.Sprintf("/api/images/%s%s?t=%d", mangaSlug, ext, time.Now().Unix())

		// Update media with new cover art URL
		media.CoverArtURL = cachedImageURL
		if err := models.UpdateMedia(media); err != nil {
			return handleError(c, fmt.Errorf("failed to update media: %w", err))
		}

		// Return success message
		successMsg := "Poster updated successfully!"
		return HandleView(c, views.SuccessAlert(successMsg))
	}

	// Existing logic for cropping from existing images
	chapterSlug := c.FormValue("chapter_slug")
	cropDataStr := c.FormValue("crop_data")
	imageIndexStr := c.FormValue("image_index")

	imageIndex := 0
	if idx, err := strconv.Atoi(imageIndexStr); err == nil {
		imageIndex = idx
	}

	// Get chapter file path
	var chapterPath string
	if chapterSlug != "" {
		// Look up chapter by slug to get the actual file
		chapter, err := models.GetChapter(mangaSlug, chapterSlug)
		if err != nil {
			return handleError(c, err)
		}
		if chapter == nil {
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		if chapter.File == "" {
			return handleErrorWithStatus(c, fmt.Errorf("chapter file not found"), fiber.StatusNotFound)
		}
		chapterPath = filepath.Join(media.Path, chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return handleError(c, fmt.Errorf("error: %v", err))
		}
	}

	// Parse crop data
	var cropData map[string]interface{}
	if err := json.Unmarshal([]byte(cropDataStr), &cropData); err != nil {
		cropData = map[string]interface{}{"x": 0, "y": 0, "width": 0, "height": 0}
	}

	// Extract crop from image and cache it
	cachedImageURL, err := utils.ExtractAndCacheImageWithCropByIndex(chapterPath, mangaSlug, imageIndex, cropData, models.GetProcessedImageQuality())
	if err != nil {
		return handleError(c, fmt.Errorf("failed to extract and cache image: %w", err))
	}

	// Update media with new cover art URL
	media.CoverArtURL = cachedImageURL
	if err := models.UpdateMedia(media); err != nil {
		return handleError(c, fmt.Errorf("failed to update media: %w", err))
	}

	// Return success message
	successMsg := fmt.Sprintf("Poster updated successfully!")
	return HandleView(c, views.SuccessAlert(successMsg))
}

// HandleDeleteMedia deletes a media and all associated data
func HandleDeleteMedia(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("media slug can't be empty"))
	}

	existingMedia, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingMedia == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found"), fiber.StatusNotFound)
	}

	// Delete the media (chapters and tags will be cascade deleted)
	if err := models.DeleteMedia(existingMedia.Slug); err != nil {
		log.Errorf("Failed to delete media '%s': %v", mangaSlug, err)
		return handleError(c, fmt.Errorf("failed to delete media: %w", err))
	}

	log.Infof("Successfully deleted media '%s'", mangaSlug)

	// Redirect to media list
	c.Set("HX-Redirect", "/series")
	return c.SendStatus(fiber.StatusOK)
}

// HandleMediaChapter handles displaying a chapter reader page.
func HandleMediaChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	media, err := models.GetMedia(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, media.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}
	if chapter == nil {
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
	}

	userName := GetUserContext(c)
	if userName != "" && !IsHTMXRequest(c) {
		_ = models.MarkChapterRead(userName, mangaSlug, chapterSlug)
	}

	return handleErrorWithStatus(c, fmt.Errorf("media chapter reading not implemented"), fiber.StatusNotImplemented)
}

// HandleMediaTOC handles getting the table of contents for a chapter.
func HandleMediaTOC(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	toc := utils.GetTOC(chapter.File)
	c.Set("Content-Type", "text/html")
	return c.SendString(toc)
}

// HandleMediaBookContent handles getting the book content for a chapter.
func HandleMediaBookContent(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	return handleErrorWithStatus(c, fmt.Errorf("media chapter content reading not implemented"), fiber.StatusNotImplemented)
}

// HandleMediaChapterTOCFragment handles TOC fragment requests for chapters.
func HandleMediaChapterTOCFragment(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	toc := utils.GetTOC(chapter.File)
	c.Set("Content-Type", "text/html")
	return c.SendString(toc)
}

// HandleMediaChapterReaderFragment handles reader fragment requests for chapters.
func HandleMediaChapterReaderFragment(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	return handleErrorWithStatus(c, fmt.Errorf("media chapter reader fragment not implemented"), fiber.StatusNotImplemented)
}

// HandleMediaAsset handles asset requests from EPUB files.
func HandleMediaAsset(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	assetPath := c.Params("*")

	log.Debugf("Asset request: media=%s, chapter=%s, assetPath=%s", mangaSlug, chapterSlug, assetPath)

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		log.Errorf("Chapter not found: %s/%s", mangaSlug, chapterSlug)
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapter.File)
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapter.File)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapter.File, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening EPUB")
	}
	defer r.Close()

	// Find the asset
	var file *zip.File
	for _, f := range r.File {
		if f.Name == assetPath {
			file = f
			break
		}
	}
	if file == nil {
		log.Errorf("Asset not found in EPUB: %s", assetPath)
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening asset")
	}
	defer rc.Close()

	// Read the asset data
	assetData, err := io.ReadAll(rc)
	if err != nil {
		log.Errorf("Error reading asset %s: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading asset")
	}

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".css":
		c.Set("Content-Type", "text/css")
	case ".xhtml", ".html":
		c.Set("Content-Type", "text/html")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	// For image assets, apply compression based on user role
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" {
		// Get user role for compression quality
		userName, _ := c.Locals("user_name").(string)
		var quality int
		if userName != "" {
			user, err := models.FindUserByUsername(userName)
			if err == nil && user != nil {
				quality = models.GetCompressionQualityForRole(user.Role)
			} else {
				quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
			}
		} else {
			quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
		}

		// Decode the image
		imageReader := bytes.NewReader(assetData)
		img, _, err := image.Decode(imageReader)
		if err != nil {
			// If decoding fails, serve original data
			log.Debugf("Serving asset %s (original, decode failed)", assetPath)
			return c.Send(assetData)
		}

		// Encode all images as JPEG for better performance and consistent compression
		var buf bytes.Buffer
		// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
		jpegQuality := quality
		if jpegQuality < 1 {
			jpegQuality = 1
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
		if err != nil {
			// If encoding fails, serve original data
			log.Debugf("Serving asset %s (original, encode failed)", assetPath)
			return c.Send(assetData)
		}
		log.Debugf("Serving asset %s (compressed)", assetPath)
		return c.Send(buf.Bytes())
	} else {
		// For non-image assets, serve original data
		log.Debugf("Serving asset %s", assetPath)
		return c.Send(assetData)
	}
}

func HandleMediaMarkRead(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.MarkChapterRead(userName, mangaSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// No content to return since hx-swap="none"
	return c.SendString("")
}

// ComicHandler processes requests to serve comic book pages based on the provided query parameters.
func ComicHandler(c *fiber.Ctx) error {
	token := c.Query("token")
	
	if token == "" {
		return handleErrorWithStatus(c, fmt.Errorf("token parameter is required"), fiber.StatusBadRequest)
	}

	// Validate and consume the token
	tokenInfo, err := utils.ValidateAndConsumeImageToken(token)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid or expired token: %w", err), fiber.StatusForbidden)
	}

	// Use the token info to get the media and chapter
	media, err := models.GetMedia(tokenInfo.MediaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if media == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	chapter, err := models.GetChapter(tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
	if err != nil {
		return handleError(c, err)
	}
	if chapter == nil {
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
	}

	// Determine the actual chapter file path
	// For single-file media (cbz/cbr), media.Path is the file itself
	// For directory-based media, we need to join path and chapter file
	filePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		filePath = filepath.Join(media.Path, chapter.File)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// If the path is a directory, serve images from within it
	if fileInfo.IsDir() {
		return serveImageFromDirectory(c, filePath, tokenInfo.Page)
	}

	lowerFileName := strings.ToLower(fileInfo.Name())

	// Serve the file based on its extension
	switch {
	case strings.HasSuffix(lowerFileName, ".jpg"), strings.HasSuffix(lowerFileName, ".jpeg"),
		strings.HasSuffix(lowerFileName, ".png"), strings.HasSuffix(lowerFileName, ".webp"),
		strings.HasSuffix(lowerFileName, ".gif"):
		// Get user role for compression quality
		userName, _ := c.Locals("user_name").(string)
		var quality int
		if userName != "" {
			user, err := models.FindUserByUsername(userName)
			if err == nil && user != nil {
				quality = models.GetCompressionQualityForRole(user.Role)
			} else {
				quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
			}
		} else {
			quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
		}

	// Load the image
	file, err := os.Open(filePath)
	if err != nil {
		// If loading fails, serve original file
		return c.SendFile(filePath)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		// If loading fails, serve original file
		return c.SendFile(filePath)
	}		// Encode all images as JPEG for better performance and consistent compression
		var buf bytes.Buffer
		// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
		jpegQuality := quality
		if jpegQuality < 1 {
			jpegQuality = 1
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
		c.Set("Content-Type", "image/jpeg")
		if err != nil {
			// If encoding fails, serve original
			return c.SendFile(filePath)
		}
		return c.Send(buf.Bytes())
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		return serveComicBookArchiveFromRAR(c, filePath, tokenInfo.Page)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		return serveComicBookArchiveFromZIP(c, filePath, tokenInfo.Page)
	default:
		return HandleView(c, views.Error("Unsupported file type"))
	}
}

// serveImageFromDirectory handles serving individual image files from a chapter directory.
func serveImageFromDirectory(c *fiber.Ctx, dirPath string, page int) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Filter for image files only
	var imageFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			lowerName := strings.ToLower(name)
			if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
				strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".gif") ||
				strings.HasSuffix(lowerName, ".webp") {
				imageFiles = append(imageFiles, name)
			}
		}
	}

	// Sort image files alphabetically for consistent ordering
	sort.Strings(imageFiles)

	if len(imageFiles) == 0 {
		return HandleView(c, views.Error("No images found in chapter directory"))
	}

	// Page numbers are 1-indexed
	if page > len(imageFiles) {
		return c.Status(fiber.StatusNotFound).SendString("Page not found")
	}

	imagePath := filepath.Join(dirPath, imageFiles[page-1])

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	var quality int
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			quality = models.GetCompressionQualityForRole(user.Role)
		} else {
			quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
		}
	} else {
		quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
	}

	// Load the image
	file, err := os.Open(imagePath)
	if err != nil {
		// If loading fails, serve original file
		return c.SendFile(imagePath)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		// If loading fails, serve original file
		return c.SendFile(imagePath)
	}

	// Encode all images as JPEG for better performance and consistent compression
	var buf bytes.Buffer
	// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	c.Set("Content-Type", "image/jpeg")
	if err != nil {
		// If encoding fails, serve original
		return c.SendFile(imagePath)
	}
	return c.Send(buf.Bytes())
}

// serveComicBookArchiveFromRAR handles serving images from a RAR archive.
func serveComicBookArchiveFromRAR(c *fiber.Ctx, filePath string, page int) error {
	rarFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open RAR file")
	}
	defer rarFile.Close()

	rarReader, err := rardecode.NewReader(rarFile, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create RAR reader")
	}

	currentPage := 0
	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to read archive entry")
		}

		if !header.IsDir && isImageFile(header.Name) {
			currentPage++
			if currentPage == page {
				// Read image data into memory for conversion
				imageData, err := io.ReadAll(rarReader)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image data")
				}

				// Get user role for compression quality
				userName, _ := c.Locals("user_name").(string)
				var quality int
				if userName != "" {
					user, err := models.FindUserByUsername(userName)
					if err == nil && user != nil {
						quality = models.GetCompressionQualityForRole(user.Role)
					} else {
						quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
					}
				} else {
					quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
				}

				// Create a reader for the image data to decode it
				imageReader := bytes.NewReader(imageData)

				// Decode the image
				img, _, err := image.Decode(imageReader)
				if err != nil {
					// If decoding fails, serve original data
					contentType := getContentType(header.Name)
					c.Set("Content-Type", contentType)
					return c.Send(imageData)
				}

				// Encode all images as JPEG for better performance and consistent compression
				var buf bytes.Buffer
				// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
				jpegQuality := quality
				if jpegQuality < 1 {
					jpegQuality = 1
				}
				err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
				if err != nil {
					// If encoding fails, serve original data
					contentType := getContentType(header.Name)
					c.Set("Content-Type", contentType)
					return c.Send(imageData)
				}
				c.Set("Content-Type", "image/jpeg")
				return c.Send(buf.Bytes())
			}
		}
	}

	return c.Status(fiber.StatusNotFound).SendString("Page not found in archive")
}

// serveComicBookArchiveFromZIP handles serving images from a ZIP archive.
func serveComicBookArchiveFromZIP(c *fiber.Ctx, filePath string, page int) error {
	zipFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open ZIP file")
	}
	defer zipFile.Close()

	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create ZIP reader")
	}
	defer zipReader.Close()

	var imageFiles []*zip.File
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() && isImageFile(file.Name) {
			imageFiles = append(imageFiles, file)
		}
	}

	if page > len(imageFiles) {
		return c.Status(fiber.StatusBadRequest).SendString("Page number out of range")
	}

	imageFile := imageFiles[page-1]

	// Try WebP conversion for better compression
	rc, err := imageFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image from archive")
	}
	defer rc.Close()

	// Read image data into memory for conversion
	imageData, err := io.ReadAll(rc)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image data")
	}

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	var quality int
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			quality = models.GetCompressionQualityForRole(user.Role)
		} else {
			quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
		}
	} else {
		quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
	}

	// Create a reader for the image data to decode it
	imageReader := bytes.NewReader(imageData)

	// Decode the image
	img, _, err := image.Decode(imageReader)
	if err != nil {
		// If decoding fails, serve original data
		contentType := getContentType(imageFile.Name)
		c.Set("Content-Type", contentType)
		return c.Send(imageData)
	}

	// Encode all images as JPEG for better performance and consistent compression
	var buf bytes.Buffer
	// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		// If encoding fails, serve original data
		contentType := getContentType(imageFile.Name)
		c.Set("Content-Type", contentType)
		return c.Send(imageData)
	}
	c.Set("Content-Type", "image/jpeg")
	return c.Send(buf.Bytes())
}

// isImageFile checks if a filename has an image extension.
func isImageFile(fileName string) bool {
	lowerName := strings.ToLower(fileName)
	return strings.HasSuffix(lowerName, ".jpg") ||
		strings.HasSuffix(lowerName, ".jpeg") ||
		strings.HasSuffix(lowerName, ".png") ||
		strings.HasSuffix(lowerName, ".webp") ||
		strings.HasSuffix(lowerName, ".gif") ||
		strings.HasSuffix(lowerName, ".bmp")
}

// getContentType determines the Content-Type header based on file extension.
func getContentType(fileName string) string {
	lowerName := strings.ToLower(fileName)
	switch {
	case strings.HasSuffix(lowerName, ".png"):
		return "image/png"
	case strings.HasSuffix(lowerName, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lowerName, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lowerName, ".bmp"):
		return "image/bmp"
	case strings.HasSuffix(lowerName, ".jpg"), strings.HasSuffix(lowerName, ".jpeg"):
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}
