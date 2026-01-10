package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/files"
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

	// Get the library for the first chapter
	library, err := models.GetLibrary(chapters[0].LibrarySlug)
	if err != nil {
		return "", fmt.Errorf("failed to get library '%s': %w", chapters[0].LibrarySlug, err)
	}

	// Use the first library folder as root
	if len(library.Folders) == 0 {
		return "", fmt.Errorf("library '%s' has no folders configured", chapters[0].LibrarySlug)
	}

	// Construct the full path using the chapter's file path
	chapterPath := filepath.Join(library.Folders[0], chapters[0].File)
	return chapterPath, nil
}

// HandlePosterChapterSelect renders a list of chapters to select from
func HandlePosterChapterSelect(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")

	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Get all chapters
	chapters, err := models.GetChapters(mangaSlug)
	if err != nil || len(chapters) == 0 {
		return handleView(c, views.EmptyState("No chapters found."))
	}

	return handleView(c, views.PosterEditor(mangaSlug, chapters, "", 0, -1, "", 1))
}

// HandlePosterSelector renders the image selector for a chapter
func HandlePosterSelector(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	chapterSlug := c.Query("chapter", "")

	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return handleView(c, views.EmptyState(fmt.Sprintf("Error: %v", err)))
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
		chapter, err := models.GetChapter(mangaSlug, "", chapterSlug)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
		if chapter == nil {
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		if chapter.File == "" {
			return handleView(c, views.EmptyState("Error: chapter file not found"))
		}
		// Get the library for the chapter
		library, err := models.GetLibrary(chapter.LibrarySlug)
		if err != nil {
			return handleView(c, views.EmptyState(fmt.Sprintf("Error: failed to get library '%s': %v", chapter.LibrarySlug, err)))
		}
		if len(library.Folders) == 0 {
			return handleView(c, views.EmptyState(fmt.Sprintf("Error: library '%s' has no folders configured", chapter.LibrarySlug)))
		}
		// Construct the full path using the chapter's file path
		chapterPath = filepath.Join(library.Folders[0], chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return handleView(c, views.EmptyState(fmt.Sprintf("Error: %v", err)))
		}
	}

	// Get count of images in the chapter file
	imageCount, err := files.CountImageFiles(chapterPath)
	if err != nil {
		return handleView(c, views.EmptyState(fmt.Sprintf("Error counting images: %v", err)))
	}
	if imageCount == 0 {
		return handleView(c, views.EmptyState("No images found in the chapter."))
	}

	return handleView(c, views.PosterEditor(mangaSlug, chapters, chapterSlug, imageCount, -1, "", chapterPage))
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
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
		chapter, err := models.GetChapter(mangaSlug, "", chapterSlug)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
		if chapter == nil {
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		if chapter.File == "" {
			return sendNotFoundError(c, ErrChapterFileReadFailed)
		}
		// Get the library for the chapter
		library, err := models.GetLibrary(chapter.LibrarySlug)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, fmt.Errorf("failed to get library '%s': %w", chapter.LibrarySlug, err))
		}
		if len(library.Folders) == 0 {
			return sendInternalServerError(c, ErrPosterProcessingFailed, fmt.Errorf("library '%s' has no folders configured", chapter.LibrarySlug))
		}
		// Construct the full path using the chapter's file path
		chapterPath = filepath.Join(library.Folders[0], chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
	}

	// Extract and get the image data URI
	imageDataURI, err := files.GetImageDataURIByIndex(chapterPath, imageIndex)
	if err != nil {
		return sendInternalServerError(c, ErrPosterProcessingFailed, err)
	}

	imageCount, err := files.CountImageFiles(chapterPath)
	if err != nil {
		return sendInternalServerError(c, ErrPosterProcessingFailed, err)
	}

	return handleView(c, views.PosterEditor(mangaSlug, chapters, chapterSlug, imageCount, imageIndex, imageDataURI, chapterPage))
}

// HandlePosterSet sets a custom poster image based on user selection or upload
func HandlePosterSet(c *fiber.Ctx) error {
	if dataManager == nil {
		return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("cache not initialized"))
	}

	mangaSlug := c.Params("media")

	media, err := models.GetMediaUnfiltered(mangaSlug)
	if err != nil || media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	dataDir := files.GetDataDirectory()
	postersDir := filepath.Join(dataDir, "posters")
	posterQuality := 100 // Use full quality for manually uploaded/cropped posters

	// Check for file upload
	if file, err := c.FormFile("poster"); err == nil {
		// Handle upload
		if err := os.MkdirAll(postersDir, 0755); err != nil {
			return sendInternalServerError(c, ErrPosterSaveFailed, err)
		}

		// Save uploaded file temporarily
		tempPath := filepath.Join(postersDir, fmt.Sprintf("temp_%s_%d", mangaSlug, time.Now().Unix()))
		if err := c.SaveFile(file, tempPath); err != nil {
			return sendInternalServerError(c, ErrPosterUploadFailed, err)
		}
		defer os.Remove(tempPath) // Clean up temp file

		// Load and convert to appropriate format
		img, err := files.OpenImage(tempPath)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}

		format := "webp"
		cachePath := fmt.Sprintf("posters/%s.%s", mangaSlug, format)

		imageData, err := files.EncodeImageToBytes(img, format, posterQuality)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
		if err := dataManager.Save(cachePath, imageData); err != nil {
			return sendInternalServerError(c, ErrPosterSaveFailed, err)
		}

		// Generate thumbnails
		if err := files.GenerateThumbnails(cachePath, mangaSlug, dataManager.Backend()); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Warning: failed to generate thumbnails: %v\n", err)
		}

		storedImageURL := fmt.Sprintf("/api/posters/%s.%s", mangaSlug, format)

		// Update media with new cover art URL
		media.CoverArtURL = storedImageURL
		if err := models.UpdateMedia(media); err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Return success message
		successMsg := "Poster updated successfully!"
		return handleView(c, views.SuccessAlert(successMsg))
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
		chapter, err := models.GetChapter(mangaSlug, "", chapterSlug)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
		if chapter == nil {
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		if chapter.File == "" {
			return sendNotFoundError(c, ErrChapterFileReadFailed)
		}
		// Get the library for the chapter
		library, err := models.GetLibrary(chapter.LibrarySlug)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, fmt.Errorf("failed to get library '%s': %w", chapter.LibrarySlug, err))
		}
		if len(library.Folders) == 0 {
			return sendInternalServerError(c, ErrPosterProcessingFailed, fmt.Errorf("library '%s' has no folders configured", chapter.LibrarySlug))
		}
		// Construct the full path using the chapter's file path
		chapterPath = filepath.Join(library.Folders[0], chapter.File)
	} else {
		// Fallback to first chapter if not specified
		var err error
		chapterPath, err = getFirstChapterFilePath(media)
		if err != nil {
			return sendInternalServerError(c, ErrPosterProcessingFailed, err)
		}
	}

	// Parse crop data
	var cropData map[string]any
	if err := json.Unmarshal([]byte(cropDataStr), &cropData); err != nil {
		cropData = map[string]any{"x": 0, "y": 0, "width": 0, "height": 0}
	}

	// Extract crop from image and cache it
	storedImageURL, err := files.ExtractAndStoreImageWithCropByIndex(chapterPath, mangaSlug, imageIndex, cropData, true, posterQuality)
	if err != nil {
		return sendInternalServerError(c, ErrPosterProcessingFailed, err)
	}

	// Generate thumbnails
	fullImagePath := fmt.Sprintf("posters/%s.webp", mangaSlug)
	if err := files.GenerateThumbnails(fullImagePath, mangaSlug, dataManager.Backend()); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to generate thumbnails: %v\n", err)
	}

	// Update media with new cover art URL
	media.CoverArtURL = storedImageURL
	if err := models.UpdateMedia(media); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Return success message
	successMsg := "Poster updated successfully!"
	return handleView(c, views.SuccessAlert(successMsg))
}
