package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
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
		postersDir := filepath.Join(cacheDir, "posters")
		if err := os.MkdirAll(postersDir, 0755); err != nil {
			return handleError(c, fmt.Errorf("failed to create posters directory: %w", err))
		}
		cachedPath := filepath.Join(postersDir, fmt.Sprintf("%s%s", mangaSlug, ext))
		if err := c.SaveFile(file, cachedPath); err != nil {
			return handleError(c, fmt.Errorf("failed to save uploaded file: %w", err))
		}
		cachedImageURL := fmt.Sprintf("/api/posters/%s%s?t=%d", mangaSlug, ext, time.Now().Unix())

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