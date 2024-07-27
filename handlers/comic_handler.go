package handlers

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/nwaples/rardecode"
)

func ComicHandler(c *fiber.Ctx) error {
	// Query parameters for extracting comic images from chapters and pages
	mangaSlug := c.Query("manga")
	chapterSlug := c.Query("chapter")
	chapterPage := c.Query("page")

	if mangaSlug == "" || chapterSlug == "" || chapterPage == "" {
		return HandleView(c, views.Error("When requesting a manga, all parameters must be provided."))
	}

	mangaID, err := models.GetMangaIDBySlug(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapterID, err := models.GetChapterIDBySlug(chapterSlug, mangaID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	manga, err := models.GetManga(mangaID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapter, err := models.GetChapter(chapterID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	filePath := filepath.Join(manga.Path, chapter.File)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	lowerFileName := strings.ToLower(fileInfo.Name())

	// Serve the file based on its extension - cbr, cbz, zip/rar/rar5, 7zip, raw images, epub, pdf)
	switch {
	case strings.HasSuffix(lowerFileName, ".jpg"), strings.HasSuffix(lowerFileName, ".png"):
		return c.SendFile(filePath)
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		return serveComicBookArchiveFromRAR(c, filePath)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		return serveComicBookArchiveFromZIP(c, filePath)
	default:
		return HandleView(c, views.Error("Unsupported file type"))
	}
}

func serveComicBookArchiveFromRAR(c *fiber.Ctx, filePath string) error {
	// Get the page parameter from the query string
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Open the RAR archive
	rarFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open archive")
	}
	defer rarFile.Close()

	// Create a new RAR reader
	rarReader, err := rardecode.NewReader(rarFile, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create RAR reader")
	}

	// Track the current page and scan entries
	currentPage := 0
	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to read archive entry")
		}

		// Check if the entry is an image and matches the requested page
		if !header.IsDir && (strings.HasSuffix(strings.ToLower(header.Name), ".jpg") || strings.HasSuffix(strings.ToLower(header.Name), ".png")) {
			currentPage++
			if currentPage == page {
				// Set Content-Type header based on file extension
				contentType := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(header.Name), ".png") {
					contentType = "image/png"
				}
				c.Set("Content-Type", contentType)

				// Copy the image data to the response writer
				if _, err := io.Copy(c.Response().BodyWriter(), rarReader); err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
				}
				return nil
			}
		}
	}

	// If we've reached this point, the requested page was not found
	return c.Status(fiber.StatusNotFound).SendString("Page not found in archive")
}

// Serve images from ZIP archives using archive/zip
func serveComicBookArchiveFromZIP(c *fiber.Ctx, filePath string) error {
	// Get the page parameter from the query string
	pageStr := c.Query("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	// Open the ZIP archive
	zipFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open archive")
	}
	defer zipFile.Close()

	// Create a new ZIP reader
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create ZIP reader")
	}
	defer zipReader.Close()

	// Filter out only image files
	var imageFiles []*zip.File
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() && (strings.HasSuffix(strings.ToLower(file.Name), ".jpg") || strings.HasSuffix(strings.ToLower(file.Name), ".png")) {
			imageFiles = append(imageFiles, file)
		}
	}

	// Check if the page number is within the range of available images
	if page > len(imageFiles) {
		return c.Status(fiber.StatusBadRequest).SendString("Page number out of range")
	}

	// Get the specified image file
	imageFile := imageFiles[page-1]

	// Set Content-Type header based on file extension
	contentType := "image/jpeg" // Default to JPEG
	if strings.HasSuffix(strings.ToLower(imageFile.Name), ".png") {
		contentType = "image/png"
	}
	c.Set("Content-Type", contentType)

	// Open the file from the ZIP archive
	rc, err := imageFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image from archive")
	}
	defer rc.Close()

	// Copy the image data to the response writer
	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
	}

	return nil
}
