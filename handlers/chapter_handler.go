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

	"github.com/gofiber/fiber/v2/log"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/sync"
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
)

// isChapterAccessible checks if a chapter is accessible to the user
func isChapterAccessible(chapter *models.Chapter, userName string) bool {
	// If released_at is set, it's released
	if chapter.ReleasedAt != nil {
		return true
	}

	if userName == "" {
		// Anonymous user
		if !chapter.IsPremium {
			// Non-premium chapters are accessible to everyone
			return true
		}

		// For premium chapters, check if anonymous role has premium chapter access
		hasAccess, err := models.RoleHasAccess("anonymous")
		if err != nil {
			log.Errorf("Failed to check premium chapter access for anonymous role: %v", err)
			return false
		}
		log.Debugf("Chapter %s is premium, anonymous access: %v", chapter.Slug, hasAccess)
		return hasAccess
	}

	// For logged-in users
	if !chapter.IsPremium {
		// Non-premium chapters are accessible to everyone
		return true
	}

	// For premium chapters, check if user has premium chapter access via permissions
	hasAccess, err := models.UserHasPremiumChapterAccess(userName)
	if err != nil {
		log.Errorf("Failed to check premium chapter access for user %s: %v", userName, err)
		return false
	}
	return hasAccess
}

// ChapterData holds all data needed to render a chapter view
type ChapterData struct {
	Media    *models.Media
	Chapter  *models.Chapter
	Chapters []models.Chapter
	PrevID   string
	NextID   string
	Images   []string
	TOC      string
	Content  string
	IsNovel  bool
}

// GetChapterData retrieves all data needed for displaying a chapter
func GetChapterData(hash, userName string) (*ChapterData, error) {
	// Determine content rating limit based on user
	cfg, err := models.GetAppConfig()
	if err != nil {
		return nil, err
	}
	contentRatingLimit := cfg.ContentRatingLimit
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			contentRatingLimit = 3 // Admins can see all content
		}
	}

	// Get chapter by ID
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		log.Errorf("GetChapterData: Failed to get chapter %s: %v", hash, err)
		return nil, err
	}
	if chapter == nil {
		log.Warnf("GetChapterData: Chapter %s not found", hash)
		return nil, nil // Not found
	}

	// Get media with user-specific limit
	media, err := models.GetMediaWithContentLimit(chapter.MediaSlug, contentRatingLimit)
	if err != nil {
		log.Errorf("GetChapterData: Failed to get media %s: %v", chapter.MediaSlug, err)
		return nil, err
	}
	if media == nil {
		log.Warnf("GetChapterData: Media %s not found", chapter.MediaSlug)
		return nil, nil // Not found
	}

	// Check library access
	var hasAccess bool
	if userName == "" {
		// Anonymous user
		hasAccess, err = models.AnonymousHasLibraryAccess(chapter.LibrarySlug)
		if err != nil {
			log.Errorf("GetChapterData: Failed to check anonymous access for library %s: %v", chapter.LibrarySlug, err)
			return nil, err
		}
	} else {
		hasAccess, err = models.UserHasLibraryAccess(userName, chapter.LibrarySlug)
		if err != nil {
			log.Errorf("GetChapterData: Failed to check user access for user %s library %s: %v", userName, chapter.LibrarySlug, err)
			return nil, err
		}
	}
	if !hasAccess {
		log.Warnf("GetChapterData: Access denied for user %s to library %s", userName, chapter.LibrarySlug)
		return nil, nil // Access denied
	}

	// Get all chapters for the media
	chapters, err := models.GetChapters(chapter.MediaSlug)
	if err != nil {
		log.Errorf("GetChapterData: Failed to get chapters for media %s: %v", chapter.MediaSlug, err)
		return nil, err
	}

	// Filter chapters to only those from the same library
	var filteredChapters []models.Chapter
	for _, c := range chapters {
		if c.LibrarySlug == chapter.LibrarySlug {
			filteredChapters = append(filteredChapters, c)
		}
	}
	chapters = filteredChapters

	// Check if chapter is released or user has early access
	if !isChapterAccessible(chapter, userName) {
		return nil, fmt.Errorf("premium_required")
	}

	// Get adjacent chapters
	prevID, nextID, err := models.GetAdjacentChapters(chapters, chapter.ID, userName)
	if err != nil {
		return nil, err
	}

	// Determine content type based on file extension
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get library '%s': %w", chapter.LibrarySlug, err)
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	// Check file extension to determine if it's a novel (.epub) or comic (.cbz, .cbr, or folder with images)
	isNovel := false
	if fileInfo, err := os.Stat(chapterFilePath); err == nil {
		if fileInfo.IsDir() {
			// Folder with images - treat as comic
			isNovel = false
		} else {
			// Check file extension
			ext := strings.ToLower(filepath.Ext(chapterFilePath))
			isNovel = ext == ".epub"
		}
	} else {
		// File doesn't exist, default to comic behavior
		isNovel = false
	}

	data := &ChapterData{
		Media:    media,
		Chapter:  chapter,
		Chapters: chapters,
		PrevID:   prevID,
		NextID:   nextID,
		IsNovel:  isNovel,
	}

	if data.IsNovel {

		data.TOC = files.GetTOC(chapterFilePath)
		validityMinutes := models.GetImageTokenValidityMinutes()
		data.Content = files.GetBookContentWithValidity(chapterFilePath, chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug, validityMinutes)
	} else {
		// Check if chapter file path exists
		if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
			// Chapter file missing, delete the chapter
			log.Warnf("Chapter file '%s' for media '%s' chapter '%s' does not exist, deleting chapter", chapterFilePath, chapter.MediaSlug, chapter.Slug)
			// First delete notifications to avoid foreign key issues
			if err := models.DeleteNotificationsForChapter(chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug); err != nil {
				log.Errorf("Failed to delete notifications for chapter '%s': %v", chapter.Slug, err)
			}
			if delErr := models.DeleteChapter(chapter.MediaSlug, chapter.Slug, chapter.LibrarySlug); delErr != nil {
				log.Errorf("Failed to delete missing chapter '%s' for media '%s': %v", chapter.Slug, chapter.MediaSlug, delErr)
			}
			return nil, fmt.Errorf("chapter_not_found")
		}

		// Get images for comic
		images, err := models.GetChapterImages(media, chapter)
		if err != nil {
			return nil, err
		}
		data.Images = images
	}

	return data, nil
}

// GetChapterAndMediaData retrieves chapter and media data with access check
func GetChapterAndMediaData(mediaSlug, librarySlug, chapterSlug, userName string) (*models.Chapter, *models.Media, error) {
	// Determine content rating limit based on user
	cfg, err := models.GetAppConfig()
	if err != nil {
		return nil, nil, err
	}
	contentRatingLimit := cfg.ContentRatingLimit
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			contentRatingLimit = 3 // Admins can see all content
		}
	}

	chapter, err := models.GetChapter(mediaSlug, librarySlug, chapterSlug)
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

	media, err := models.GetMediaWithContentLimit(mediaSlug, contentRatingLimit)
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
	hash := string([]byte(c.Params("hash")))

	// Validate hash to prevent malformed URLs
	if strings.ContainsAny(hash, "/,") {
		log.Warnf("Invalid hash detected: %s", hash)
		return SendBadRequestError(c, ErrInvalidMediaSlug)
	}

	userName := GetUserContext(c)

	// Get user role for conditional rendering
	userRole := ""
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			userRole = user.Role
		}
	}

	// Get chapter data from service
	data, err := GetChapterData(hash, userName)
	if err != nil {
		if err.Error() == "premium_required" {
			// Show notification
			triggerNotification(c, "This chapter is in premium early access. Please wait for it to be released or upgrade your account.", "destructive")
			if isHTMXRequest(c) {
				// For HTMX requests, just show notification without modifying the page
				return c.Status(fiber.StatusNoContent).SendString("")
			} else {
				// For regular requests, redirect to home page
				return c.Redirect("/", fiber.StatusSeeOther)
			}
		}
		if err.Error() == "chapter_not_found" {
			if isHTMXRequest(c) {
				triggerNotification(c, "This chapter is no longer available and has been removed.", "warning")
				// Return 204 No Content to prevent navigation/swap but show notification
				return c.Status(fiber.StatusNoContent).SendString("")
			}
			return SendNotFoundError(c, ErrChapterRemoved)
		}
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if data == nil {
		if isHTMXRequest(c) {
			triggerNotification(c, "Chapter or media not found.", "warning")
			// Return 204 No Content to prevent navigation/swap but show notification
			return c.Status(fiber.StatusNoContent).SendString("")
		}
		return SendNotFoundError(c, "Chapter or media not found.")
	}

	// Fetch comments for the chapter
	comments, err := models.GetCommentsByTargetAndMedia("chapter", data.Chapter.Slug, data.Media.Slug)
	if err != nil {
		log.Errorf("Failed to fetch comments for chapter %s: %v", data.Chapter.ID, err)
		comments = []models.Comment{} // Initialize empty slice on error
	}

	// Mark read if needed
	if userName != "" {
		err = models.MarkChapterRead(userName, data.Media.Slug, data.Chapter.LibrarySlug, data.Chapter.Slug)
		if err != nil {
			// Log error but don't fail the request
			log.Errorf("Failed to mark chapter read: %v", err)
		}
		sync.SyncReadingProgressForUser(userName, data.Media.Slug, data.Chapter.LibrarySlug, data.Chapter.Slug)
	}

	// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
	rev := make([]models.Chapter, len(data.Chapters))
	for i := range data.Chapters {
		rev[i] = data.Chapters[len(data.Chapters)-1-i]
	}

	if data.IsNovel {
		return handleView(c, views.NovelChapter(data.PrevID, data.Chapter.Slug, data.NextID, *data.Media, *data.Chapter, rev, data.TOC, data.Content))
	}

	return handleView(c, views.Chapter(data.PrevID, data.Chapter.Slug, data.NextID, *data.Media, data.Images, *data.Chapter, data.Chapters, comments, userRole, userName))
}

// HandleMediaChapterTOC handles TOC requests for media chapters
func HandleMediaChapterTOC(c *fiber.Ctx) error {
	hash := c.Params("hash")

	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		log.Errorf("Failed to get chapter %s: %v", hash, err)
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return SendNotFoundError(c, ErrChapterNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if !hasAccess {
		if isHTMXRequest(c) {
			triggerNotification(c, "Access denied: you don't have permission to view this chapter", "destructive")
			return c.Status(fiber.StatusForbidden).SendString("")
		}
		return SendForbiddenError(c, ErrChapterAccessDenied)
	}

	// Get media to construct full path
	media, err := models.GetMedia(chapter.MediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		return SendNotFoundError(c, ErrMediaNotFound)
	}

	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	// Check if the file exists and is an EPUB
	if fileInfo, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		return SendNotFoundError(c, ErrEPUBNotFound)
	} else if !fileInfo.IsDir() && strings.ToLower(filepath.Ext(chapterFilePath)) != ".epub" {
		return SendBadRequestError(c, "TOC only available for EPUB files")
	}

	toc := files.GetTOC(chapterFilePath)
	c.Set("Content-Type", "text/html")
	return c.SendString(toc)
}

// HandleMediaChapterContent handles book content requests for media chapters
func HandleMediaChapterContent(c *fiber.Ctx) error {
	hash := string([]byte(c.Params("hash")))

	userName := GetUserContext(c)

	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		log.Errorf("Failed to get chapter %s: %v", hash, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check access
	hasAccess, err := models.UserHasLibraryAccess(userName, chapter.LibrarySlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if !hasAccess {
		return c.Status(fiber.StatusForbidden).SendString("Access denied")
	}

	media, err := models.GetMedia(chapter.MediaSlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Media not found")
	}

	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	// Check if the file exists and is an EPUB
	if fileInfo, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		return SendNotFoundError(c, ErrEPUBNotFound)
	} else if !fileInfo.IsDir() && strings.ToLower(filepath.Ext(chapterFilePath)) != ".epub" {
		return c.Status(fiber.StatusBadRequest).SendString("Content only available for EPUB files")
	}

	validityMinutes := models.GetImageTokenValidityMinutes()
	content := files.GetBookContentWithValidity(chapterFilePath, chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug, validityMinutes)

	c.Set("Content-Type", "text/html")
	return c.SendString(content)
}

// HandleMediaChapterAsset handles asset requests from EPUB files with token validation
func HandleMediaChapterAsset(c *fiber.Ctx) error {
	token := c.Query("token")

	if token == "" {
		return SendBadRequestError(c, ErrImageTokenRequired)
	}

	// Validate the token
	tokenInfo, err := files.ValidateImageToken(token)
	if err != nil {
		return SendForbiddenError(c, "Invalid or expired token")
	}

	// Consume the token after the response is sent
	defer files.ConsumeImageToken(token)

	hash := c.Params("hash")
	assetPath := c.Params("*")

	log.Debugf("Asset request: hash=%s, assetPath=%s", hash, assetPath)

	// Get chapter to verify
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return SendNotFoundError(c, ErrChapterNotFound)
	}

	// Verify token matches the requested resource
	if tokenInfo.MediaSlug != chapter.MediaSlug || tokenInfo.LibrarySlug != chapter.LibrarySlug || tokenInfo.ChapterSlug != chapter.Slug {
		return SendForbiddenError(c, "Token does not match requested resource")
	}

	// Get media
	media, err := models.GetMedia(chapter.MediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		return SendNotFoundError(c, ErrMediaNotFound)
	}

	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

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
	opfDir, err := files.GetOPFDir(chapterFilePath)
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
		// Always encode to WebP at 100% quality
		quality := 100

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
		jpegQuality := max(quality, 1)
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
	hash := c.Params("hash")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return SendNotFoundError(c, ErrChapterNotFound)
	}

	if err := models.MarkChapterRead(userName, chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug); err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	sync.SyncReadingProgressForUser(userName, chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug)
	// Return the inline eye toggle fragment so HTMX will swap the icon in-place.
	return handleView(c, views.InlineEyeToggle(true, chapter.ID))
}

// HandleMarkUnread unmarks a chapter as read for the logged-in user via HTMX
func HandleMarkUnread(c *fiber.Ctx) error {
	hash := c.Params("hash")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return SendNotFoundError(c, ErrChapterNotFound)
	}

	if err := models.UnmarkChapterRead(userName, chapter.MediaSlug, chapter.LibrarySlug, chapter.Slug); err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	// Return the inline eye toggle fragment with read=false so HTMX swaps to the closed-eye.
	return handleView(c, views.InlineEyeToggle(false, chapter.ID))
}

// HandleUnmarkChapterPremium unmarks a chapter as premium by updating its created_at to release it immediately
// HandleUnmarkChapterPremium handles unmarking a chapter as premium (making it immediately available)
func HandleUnmarkChapterPremium(c *fiber.Ctx) error {
	hash := c.Params("hash")

	// Get the chapter to check if it exists
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return SendNotFoundError(c, ErrChapterNotFound)
	}

	// Set released_at to now to mark as released
	if err := models.UpdateChapterReleasedAt(chapter.MediaSlug, chapter.Slug, chapter.LibrarySlug, time.Now()); err != nil {
		return SendInternalServerError(c, ErrChapterReleaseFailed, err)
	}

	// Redirect back to the media page to refresh the chapters list
	c.Set("HX-Redirect", fmt.Sprintf("/series/%s", chapter.MediaSlug))
	return c.SendStatus(fiber.StatusOK)
}
