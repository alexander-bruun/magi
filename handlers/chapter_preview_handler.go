package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// HandleChapterPreview generates and serves a preview thumbnail for a chapter.
// The preview is a smart crop of the most visually interesting page in the chapter.
// Previews are cached in the file store under "previews/{mediaSlug}_{chapterSlug}.jpg".
// Route: GET /api/chapter-preview/:media/:chapter
func HandleChapterPreview(c fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	if mediaSlug == "" || chapterSlug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("invalid parameters")
	}

	// Cache key in file store
	cachePath := fmt.Sprintf("previews/%s_%s.jpg", mediaSlug, chapterSlug)

	// Check cache first
	if fileStore != nil {
		exists, err := fileStore.Exists(cachePath)
		if err == nil && exists {
			data, err := fileStore.Load(cachePath)
			if err == nil {
				c.Set("Content-Type", "image/jpeg")
				c.Set("Cache-Control", "public, max-age=604800, immutable")
				return c.Send(data)
			}
		}
	}

	// Look up the chapter — try without library slug first (matches any library)
	chapter, err := models.GetChapter(mediaSlug, "", chapterSlug)
	if err != nil {
		log.Errorf("HandleChapterPreview: failed to get chapter %s/%s: %v", mediaSlug, chapterSlug, err)
		return c.Status(fiber.StatusNotFound).SendString("chapter not found")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("chapter not found")
	}

	// Check library access
	hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
	if err != nil || !hasAccess {
		return c.Status(fiber.StatusForbidden).SendString("access denied")
	}

	// Get the library to build the file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("library error")
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	// Check that the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("chapter file not found")
	}

	// Check file extension — skip EPUBs (novels don't have visual previews)
	if strings.HasSuffix(strings.ToLower(chapterFilePath), ".epub") {
		return c.Status(fiber.StatusNotFound).SendString("no preview for novels")
	}

	// Generate the preview
	start := time.Now()
	previewData, err := files.GenerateChapterPreview(chapterFilePath)
	if err != nil {
		log.Errorf("HandleChapterPreview: failed to generate preview for %s/%s: %v", mediaSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("preview generation failed")
	}
	log.Debugf("Generated chapter preview for %s/%s in %v", mediaSlug, chapterSlug, time.Since(start))

	// Cache the result
	if fileStore != nil {
		if err := fileStore.Save(cachePath, previewData); err != nil {
			log.Warnf("HandleChapterPreview: failed to cache preview: %v", err)
		}
	}

	c.Set("Content-Type", "image/jpeg")
	c.Set("Cache-Control", "public, max-age=604800, immutable")
	return c.Send(previewData)
}

// HandleChapterListPanel returns the HTML for the sliding chapter list panel.
// Route: GET /api/chapter-panel/:media/:chapter
func HandleChapterListPanel(c fiber.Ctx) error {
	mediaSlug := c.Params("media")
	currentChapterSlug := c.Params("chapter")

	if mediaSlug == "" || currentChapterSlug == "" {
		return SendBadRequestError(c, "invalid parameters")
	}

	userName := GetUserContext(c)

	// Get media
	contentRatingLimit := GetContentRatingLimit(userName)
	media, err := models.GetMediaWithContentLimit(mediaSlug, contentRatingLimit)
	if err != nil || media == nil {
		return SendNotFoundError(c, "media not found")
	}

	// Get all chapters
	chapters, err := models.GetChapters(mediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get the current chapter to filter by library
	currentChapter, err := models.GetChapter(mediaSlug, "", currentChapterSlug)
	if err != nil || currentChapter == nil {
		return SendNotFoundError(c, "chapter not found")
	}

	// Filter to same library
	var filteredChapters []models.Chapter
	for _, ch := range chapters {
		if ch.LibrarySlug == currentChapter.LibrarySlug {
			filteredChapters = append(filteredChapters, ch)
		}
	}

	return renderComponent(c, views.ChapterListPanel(*media, currentChapterSlug, filteredChapters))
}
