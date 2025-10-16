package handlers

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/nwaples/rardecode"
)

// ComicHandler processes requests to serve comic book pages based on the provided query parameters.
func ComicHandler(c *fiber.Ctx) error {
	mangaSlug := c.Query("manga")
	chapterSlug := c.Query("chapter")
	chapterPage := c.Query("page")

	if mangaSlug == "" || chapterSlug == "" || chapterPage == "" {
		return HandleView(c, views.Error("When requesting a manga, all parameters must be provided"))
	}

	manga, err := models.GetManga(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}
	if manga == nil {
		return HandleView(c, views.Error("Manga not found or access restricted based on content rating settings."))
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Determine the actual chapter file path
	// For single-file manga (cbz/cbr), manga.Path is the file itself
	// For directory-based manga, we need to join path and chapter file
	filePath := manga.Path
	if fileInfo, err := os.Stat(manga.Path); err == nil && fileInfo.IsDir() {
		filePath = filepath.Join(manga.Path, chapter.File)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// If the path is a directory, serve images from within it
	if fileInfo.IsDir() {
		return serveImageFromDirectory(c, filePath)
	}

	lowerFileName := strings.ToLower(fileInfo.Name())

	// Serve the file based on its extension
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

// serveImageFromDirectory handles serving individual image files from a chapter directory.
func serveImageFromDirectory(c *fiber.Ctx, dirPath string) error {
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

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
	return c.SendFile(imagePath)
}

// serveComicBookArchiveFromRAR handles serving images from a RAR archive.
func serveComicBookArchiveFromRAR(c *fiber.Ctx, filePath string) error {
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

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

		if !header.IsDir && (strings.HasSuffix(strings.ToLower(header.Name), ".jpg") || strings.HasSuffix(strings.ToLower(header.Name), ".png")) {
			currentPage++
			if currentPage == page {
				contentType := getContentType(header.Name)
				c.Set("Content-Type", contentType)

				if _, err := io.Copy(c.Response().BodyWriter(), rarReader); err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
				}
				return nil
			}
		}
	}

	return c.Status(fiber.StatusNotFound).SendString("Page not found in archive")
}

// serveComicBookArchiveFromZIP handles serving images from a ZIP archive.
func serveComicBookArchiveFromZIP(c *fiber.Ctx, filePath string) error {
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

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
		if !file.FileInfo().IsDir() && (strings.HasSuffix(strings.ToLower(file.Name), ".jpg") || strings.HasSuffix(strings.ToLower(file.Name), ".png")) {
			imageFiles = append(imageFiles, file)
		}
	}

	if page > len(imageFiles) {
		return c.Status(fiber.StatusBadRequest).SendString("Page number out of range")
	}

	imageFile := imageFiles[page-1]
	contentType := getContentType(imageFile.Name)
	c.Set("Content-Type", contentType)

	rc, err := imageFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image from archive")
	}
	defer rc.Close()

	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
	}

	return nil
}

// getContentType determines the Content-Type header based on file extension.
func getContentType(fileName string) string {
	if strings.HasSuffix(strings.ToLower(fileName), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}
