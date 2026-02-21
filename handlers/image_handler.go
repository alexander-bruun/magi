package handlers

import (
	"archive/zip"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/files"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// handleAvatarRequest serves avatar images with quality based on user role
func handleAvatarRequest(c fiber.Ctx) error {
	return handleStoredImageRequest(c, "avatars")
}

// handlePosterRequest serves poster images with quality based on user role
func handlePosterRequest(c fiber.Ctx) error {
	return handleStoredImageRequest(c, "posters")
}

// handleStoredImageRequest serves stored images with quality based on user role
func handleStoredImageRequest(c fiber.Ctx, subDir string) error {
	if fileStore == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Data manager not initialized")
	}

	// Get the requested path (remove /api/{subDir}/ prefix)
	imagePath := filepath.Join(subDir, strings.TrimPrefix(c.Path(), "/api/"+subDir+"/"))

	if imagePath == "" || imagePath == subDir+"/" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image path")
	}

	// Check for size parameter and modify path accordingly
	size := c.Query("size")
	if size != "" {
		// Remove extension and add size suffix
		ext := filepath.Ext(imagePath)
		base := strings.TrimSuffix(imagePath, ext)
		switch size {
		case "small":
			imagePath = base + "_small" + ext
		case "thumb":
			imagePath = base + "_thumb" + ext
		case "tiny":
			imagePath = base + "_tiny" + ext
		case "display":
			imagePath = base + "_display" + ext
		}
	}

	// Check if the file exists in data manager
	exists, err := fileStore.Exists(imagePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Cache error")
	}
	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	}

	// Load the image
	start := time.Now()
	reader, err := fileStore.LoadReader(imagePath)
	if err != nil {
		// If loading fails, try to serve without compression
		switch strings.ToLower(filepath.Ext(imagePath)) {
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".gif":
			c.Set("Content-Type", "image/gif")
		case ".webp":
			c.Set("Content-Type", "image/webp")
		default:
			c.Set("Content-Type", "application/octet-stream")
		}
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		data, err := fileStore.Load(imagePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to load image")
		}
		return c.Send(data)
	}
	defer reader.Close()
	img, _, err := image.Decode(reader)
	if err != nil {
		// If loading fails, serve original file
		switch strings.ToLower(filepath.Ext(imagePath)) {
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".gif":
			c.Set("Content-Type", "image/gif")
		case ".webp":
			c.Set("Content-Type", "image/webp")
		default:
			c.Set("Content-Type", "application/octet-stream")
		}
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		data, err := fileStore.Load(imagePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to load image")
		}
		return c.Send(data)
	}

	// Record load time
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
	if ext == "" {
		ext = "unknown"
	}
	log.Debugf("Recording image load time for %s: %v", ext, time.Since(start).Seconds())
	imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())

	// Encode all images as WebP for better compression
	data, err := files.EncodeImageToBytes(img, "webp", 100)
	c.Set("Content-Type", "image/webp")

	if err != nil {
		// If encoding fails, serve original file
		switch strings.ToLower(filepath.Ext(imagePath)) {
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".gif":
			c.Set("Content-Type", "image/gif")
		case ".webp":
			c.Set("Content-Type", "image/webp")
		default:
			c.Set("Content-Type", "application/octet-stream")
		}
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		data, err := fileStore.Load(imagePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to load image")
		}
		return c.Send(data)
	}

	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Send(data)
}

// ChapterImageHandler serves chapter images (comics and EPUB assets) via encrypted slug URLs.
// Route: /series/:media/:chapter/:slug
// The slug is decrypted to determine the content type (page or asset), library, and target.
func ChapterImageHandler(c fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")
	slug := c.Params("slug")

	if mediaSlug == "" || chapterSlug == "" || slug == "" {
		return SendBadRequestError(c, "invalid image URL")
	}

	// Decrypt the slug to get type, library slug, and value
	slugType, librarySlug, value, err := files.DecryptSlug(mediaSlug, chapterSlug, slug)
	if err != nil {
		log.Debugf("ChapterImageHandler: slug decryption failed: %v", err)
		return SendNotFoundError(c, ErrImageNotFound)
	}

	switch slugType {
	case "page":
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return SendBadRequestError(c, "invalid page")
		}

		chapter, err := models.GetChapter(mediaSlug, librarySlug, chapterSlug)
		if err != nil {
			log.Errorf("ChapterImageHandler: Failed to get chapter %s/%s: %v", mediaSlug, chapterSlug, err)
			return SendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			return SendNotFoundError(c, ErrChapterNotFound)
		}

		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			return SendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			return SendForbiddenError(c, ErrImageAccessDenied)
		}

		return serveComicPage(c, chapter, page)

	case "asset":
		media, err := models.GetMedia(mediaSlug)
		if err != nil || media == nil {
			return SendNotFoundError(c, ErrImageNotFound)
		}

		chapter, err := models.GetChapter(mediaSlug, librarySlug, chapterSlug)
		if err != nil {
			log.Errorf("ChapterImageHandler: Failed to get chapter %s/%s: %v", mediaSlug, chapterSlug, err)
			return SendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			return SendNotFoundError(c, ErrChapterNotFound)
		}

		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			return SendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			return SendForbiddenError(c, ErrImageAccessDenied)
		}

		return serveLightNovelAsset(c, media, chapter, value)

	default:
		return SendBadRequestError(c, "invalid image URL")
	}
}

// serveComicPage serves a comic page image
func serveComicPage(c fiber.Ctx, chapter *models.Chapter, page int) error {
	start := time.Now()
	// log.Infof("serveComicPage: serving page %d for media %s chapter %s", page, media.Slug, chapter.Slug)
	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	filePath := filepath.Join(library.Folders[0], chapter.File)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return SendNotFoundError(c, ErrImageNotFound)
	}

	// If the path is a directory, serve images from within it
	if fileInfo.IsDir() {
		imageLoadDuration.WithLabelValues("dir").Observe(time.Since(start).Seconds())
		return serveImageFromDirectoryImageHandler(c, filePath, page)
	}

	lowerFileName := strings.ToLower(fileInfo.Name())

	// Serve the file based on its extension
	switch {
	case strings.HasSuffix(lowerFileName, ".jpg"), strings.HasSuffix(lowerFileName, ".jpeg"),
		strings.HasSuffix(lowerFileName, ".png"), strings.HasSuffix(lowerFileName, ".webp"),
		strings.HasSuffix(lowerFileName, ".gif"):
		// Serve raw image bytes
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".gif":
			c.Set("Content-Type", "image/gif")
		case ".webp":
			c.Set("Content-Type", "image/webp")
		default:
			c.Set("Content-Type", "application/octet-stream")
		}
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
		imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())
		return c.SendFile(filePath)
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		imageLoadDuration.WithLabelValues("cbr").Observe(time.Since(start).Seconds())
		imageBytes, contentType, err := ServeComicArchiveFromRAR(filePath, page)
		if err != nil {
			return SendInternalServerError(c, ErrImageProcessingFailed, err)
		}
		c.Set("Content-Type", contentType)
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		imageLoadDuration.WithLabelValues("cbz").Observe(time.Since(start).Seconds())
		imageBytes, contentType, err := ServeComicArchiveFromZIP(filePath, page)
		if err != nil {
			return SendInternalServerError(c, ErrImageProcessingFailed, err)
		}
		c.Set("Content-Type", contentType)
		return c.Send(imageBytes)
	default:
		return SendBadRequestError(c, ErrImageUnsupportedType)
	}
}

// serveLightNovelAsset serves a light novel asset from an EPUB file
func serveLightNovelAsset(c fiber.Ctx, media *models.Media, chapter *models.Chapter, assetPath string) error {
	start := time.Now()
	log.Debugf("serveLightNovelAsset: serving asset %s for media %s chapter %s", assetPath, media.Slug, chapter.Slug)
	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	log.Debugf("Light novel asset request: media=%s, chapter=%s, asset=%s, file=%s", media.Slug, chapter.Slug, assetPath, chapterFilePath)

	log.Debugf("Serving asset: %s\n", assetPath)

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapterFilePath)
		return SendNotFoundError(c, ErrImageNotFound)
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapterFilePath)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapterFilePath, err)
		return SendInternalServerError(c, ErrImageProcessingFailed, err)
	}
	defer r.Close()

	// Get the OPF directory
	opfDir, err := files.GetOPFDir(chapterFilePath)
	if err != nil {
		log.Errorf("Error getting OPF dir for %s: %v", chapterFilePath, err)
		return SendInternalServerError(c, ErrImageProcessingFailed, err)
	}

	log.Debugf("OPF dir: %s, requested asset: %s", opfDir, assetPath)

	// Find the asset
	assetFullPath := filepath.Join(opfDir, assetPath)
	log.Debugf("Looking for asset at: %s", assetFullPath)

	var file *zip.File
	for _, f := range r.File {
		log.Debugf("EPUB file: %s", f.Name)
		if f.Name == assetFullPath {
			file = f
			break
		}
	}
	if file == nil {
		log.Errorf("Asset not found in EPUB: %s (looked for %s)", assetPath, assetFullPath)
		return SendNotFoundError(c, ErrImageNotFound)
	}

	log.Debugf("Asset found in EPUB: %s", assetFullPath)

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return SendInternalServerError(c, ErrImageProcessingFailed, err)
	}
	defer rc.Close()

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/webp")
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

	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		log.Errorf("Error writing asset %s to response: %v", assetPath, err)
		return SendInternalServerError(c, ErrImageProcessingFailed, err)
	}
	metricExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(assetPath), "."))
	imageLoadDuration.WithLabelValues(metricExt).Observe(time.Since(start).Seconds())
	return nil
}

// serveImageFromDirectoryImageHandler handles serving individual image files from a chapter directory.
func serveImageFromDirectoryImageHandler(c fiber.Ctx, dirPath string, page int) error {
	start := time.Now()
	imagePath, err := GetImagesFromDirectory(dirPath, page)
	if err != nil {
		return SendNotFoundError(c, ErrImageNotFound)
	}

	// Serve raw image bytes
	switch strings.ToLower(filepath.Ext(imagePath)) {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".webp":
		c.Set("Content-Type", "image/webp")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}
	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
	imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())
	return c.SendFile(imagePath)
}
