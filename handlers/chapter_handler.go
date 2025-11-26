package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// isChapterAccessible checks if a chapter is accessible to the user
func isChapterAccessible(chapter *models.Chapter, userName string) bool {
	// If released_at is set, it's released
	if chapter.ReleasedAt != nil {
		return true
	}

	if userName == "" {
		// Anonymous user - only allow fully released chapters (after early access period)
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config: %v", err)
			return false
		}
		releaseTime := chapter.CreatedAt.Add(time.Duration(cfg.PremiumEarlyAccessDuration) * time.Second)
		return time.Now().After(releaseTime)
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return false
	}

	if user.Role == "premium" {
		// Premium users can access all chapters
		return true
	}

	// For other roles, allow after creation (no early access restriction)
	return time.Now().After(chapter.CreatedAt)
}

// ChapterData holds all data needed to render a chapter view
type ChapterData struct {
	Media     *models.Media
	Chapter   *models.Chapter
	Chapters  []models.Chapter
	PrevSlug  string
	NextSlug  string
	Images    []string
	TOC       string
	Content   string
	IsNovel   bool
}

// GetChapterData retrieves all data needed for displaying a chapter
func GetChapterData(mediaSlug, chapterSlug, userName string) (*ChapterData, error) {
	// Get media and chapters
	media, chapters, err := models.GetMediaAndChapters(mediaSlug)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, nil // Not found
	}

	// Check library access
	var hasAccess bool
	if userName == "" {
		// Anonymous user
		accessibleLibs, err := models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			return nil, err
		}
		for _, lib := range accessibleLibs {
			if lib == media.LibrarySlug {
				hasAccess = true
				break
			}
		}
	} else {
		hasAccess, err = models.UserHasLibraryAccess(userName, media.LibrarySlug)
		if err != nil {
			return nil, err
		}
	}
	if !hasAccess {
		return nil, nil // Access denied
	}

	// Get chapter
	chapter, err := models.GetChapter(mediaSlug, chapterSlug)
	if err != nil {
		return nil, err
	}
	if chapter == nil {
		return nil, nil // Not found
	}

	// Check if chapter is released or user has early access
	if !isChapterAccessible(chapter, userName) {
		return nil, fmt.Errorf("premium_required")
	}

	// Get adjacent chapters
	prevSlug, nextSlug, err := models.GetAdjacentChapters(chapter.Slug, mediaSlug)
	if err != nil {
		return nil, err
	}

	data := &ChapterData{
		Media:    media,
		Chapter:  chapter,
		Chapters: chapters,
		PrevSlug: prevSlug,
		NextSlug: nextSlug,
		IsNovel:  media.Type == "novel",
	}

	if data.IsNovel {
		// Handle novel-specific logic
		chapterFilePath := media.Path
		if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
			chapterFilePath = filepath.Join(media.Path, chapter.File)
		}

		// Check if file exists
		if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
			return nil, nil // File not found
		}

		data.TOC = utils.GetTOC(chapterFilePath)
		validityMinutes := models.GetImageTokenValidityMinutes()
		data.Content = utils.GetBookContentWithValidity(chapterFilePath, mediaSlug, chapterSlug, validityMinutes)
	} else {
		// Get images for comic
		images, err := models.GetChapterImages(media, chapter)
		if err != nil {
			return nil, err
		}
		data.Images = images
	}

	return data, nil
}

// MarkChapterReadIfNeeded marks a chapter as read for non-HTMX requests
func MarkChapterReadIfNeeded(userName, mediaSlug, chapterSlug string, isHTMX bool) error {
	if userName != "" && !isHTMX {
		return models.MarkChapterRead(userName, mediaSlug, chapterSlug)
	}
	return nil
}

// GetChapterAndMediaData retrieves chapter and media data with access check
func GetChapterAndMediaData(mediaSlug, chapterSlug, userName string) (*models.Chapter, *models.Media, error) {
	chapter, err := models.GetChapter(mediaSlug, chapterSlug)
	if err != nil {
		return nil, nil, err
	}
	if chapter == nil {
		return nil, nil, nil
	}

	// Check access
	hasAccess, err := models.UserHasLibraryAccess(userName, chapter.MediaSlug)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, nil
	}

	media, err := models.GetMedia(mediaSlug)
	if err != nil {
		return nil, nil, err
	}
	if media == nil {
		return nil, nil, nil
	}

	return chapter, media, nil
}

// HandleChapter shows a chapter reader with navigation and optional read tracking.
func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := string([]byte(c.Params("media")))
	chapterSlug := string([]byte(c.Params("chapter")))

	// Validate media slug to prevent malformed URLs
	if strings.ContainsAny(mangaSlug, "/,") {
		return handleErrorWithStatus(c, fmt.Errorf("invalid media slug"), fiber.StatusBadRequest)
	}

	userName := GetUserContext(c)

	// Get chapter data from service
	data, err := GetChapterData(mangaSlug, chapterSlug, userName)
	if err != nil {
		if err.Error() == "premium_required" {
			// Show notification
			c.Set("HX-Trigger", `{"showNotification": {"message": "This chapter is in premium early access. Please wait for it to be released or upgrade your account.", "status": "destructive"}}`)
			if IsHTMXRequest(c) {
				// For HTMX requests, just show notification without modifying the page
				return c.Status(fiber.StatusNoContent).SendString("")
			} else {
				// For regular requests, redirect to manga page
				return c.Redirect(fmt.Sprintf("/series/%s", mangaSlug), fiber.StatusSeeOther)
			}
		}
		return handleError(c, err)
	}
	if data == nil {
		if IsHTMXRequest(c) {
			c.Set("HX-Trigger", `{"showNotification": {"message": "Access denied: you don't have permission to view this chapter", "status": "destructive"}}`)
			c.Set("HX-Redirect", fmt.Sprintf("/series/%s", mangaSlug))
			return c.SendString("")
		}
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found or access denied"), fiber.StatusNotFound)
	}

	// Mark read if needed (fallback for non-HTMX requests)
	err = MarkChapterReadIfNeeded(userName, mangaSlug, chapterSlug, IsHTMXRequest(c))
	if err != nil {
		// Log error but don't fail the request
		log.Errorf("Failed to mark chapter read: %v", err)
	}

	// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
	rev := make([]models.Chapter, len(data.Chapters))
	for i := range data.Chapters {
		rev[i] = data.Chapters[len(data.Chapters)-1-i]
	}

	if data.IsNovel {
		return HandleView(c, views.NovelChapter(data.PrevSlug, data.Chapter.Slug, data.NextSlug, *data.Media, *data.Chapter, rev, data.TOC, data.Content))
	}

	return HandleView(c, views.Chapter(data.PrevSlug, data.Chapter.Slug, data.NextSlug, *data.Media, data.Images, *data.Chapter, rev))
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
		if IsHTMXRequest(c) {
			c.Set("HX-Trigger", `{"showNotification": {"message": "Access denied: you don't have permission to view this chapter", "status": "destructive"}}`)
			return c.Status(fiber.StatusForbidden).SendString("")
		}
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

	userName := GetUserContext(c)

	chapter, media, err := GetChapterAndMediaData(mangaSlug, chapterSlug, userName)
	if err != nil {
		log.Errorf("Failed to get chapter data %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil || media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
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

	validityMinutes := models.GetImageTokenValidityMinutes()
	content := utils.GetBookContentWithValidity(chapterFilePath, mangaSlug, chapterSlug, validityMinutes)

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

	userName := GetUserContext(c)

	chapter, media, err := GetChapterAndMediaData(mangaSlug, chapterSlug, userName)
	if err != nil {
		log.Errorf("Failed to get chapter data %s/%s: %v", mangaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil || media == nil {
		log.Errorf("Chapter not found: %s/%s", mangaSlug, chapterSlug)
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
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

// HandleUnmarkChapterPremium unmarks a chapter as premium by updating its created_at to release it immediately
// HandleUnmarkChapterPremium handles unmarking a chapter as premium (making it immediately available)
func HandleUnmarkChapterPremium(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	// Get the chapter to check if it exists
	chapter, err := models.GetChapter(mediaSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}
	if chapter == nil {
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
	}

	// Set released_at to now to mark as released
	if err := models.UpdateChapterReleasedAt(mediaSlug, chapterSlug, time.Now()); err != nil {
		return handleError(c, err)
	}

	// Redirect back to the media page to refresh the chapters list
	c.Set("HX-Redirect", fmt.Sprintf("/series/%s", mediaSlug))
	return c.SendStatus(fiber.StatusOK)
}