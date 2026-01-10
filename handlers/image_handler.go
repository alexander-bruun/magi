package handlers

import (
	"archive/zip"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/files"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// handleAvatarRequest serves avatar images with quality based on user role
func handleAvatarRequest(c *fiber.Ctx) error {
	return handleStoredImageRequest(c, "avatars")
}

// handlePosterRequest serves poster images with quality based on user role
func handlePosterRequest(c *fiber.Ctx) error {
	return handleStoredImageRequest(c, "posters")
}

// handleStoredImageRequest serves stored images with quality based on user role
func handleStoredImageRequest(c *fiber.Ctx, subDir string) error {
	if dataManager == nil {
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
	exists, err := dataManager.Exists(imagePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Cache error")
	}
	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	}

	// Load the image
	start := time.Now()
	reader, err := dataManager.LoadReader(imagePath)
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
		data, err := dataManager.Load(imagePath)
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
		data, err := dataManager.Load(imagePath)
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
		data, err := dataManager.Load(imagePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to load image")
		}
		return c.Send(data)
	}

	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Send(data)
}

// ImageHandler serves images for both comics and light novels using token-based authentication
func ImageHandler(c *fiber.Ctx) error {
	token := c.Query("token")
	log.Debugf("ImageHandler: received token %s", token)

	if token == "" {
		log.Errorf("ImageHandler: token parameter is required")
		return sendBadRequestError(c, ErrImageTokenRequired)
	}

	// Validate the token
	tokenInfo, err := files.ValidateImageToken(token)
	if err != nil {
		log.Errorf("ImageHandler: Token validation failed for token %s: %v", token, err)
		return sendForbiddenError(c, ErrImageTokenInvalid)
	}

	// Validate MediaSlug to prevent malformed tokens
	if strings.ContainsAny(tokenInfo.MediaSlug, "/,") {
		log.Errorf("ImageHandler: Invalid MediaSlug in token: %s", tokenInfo.MediaSlug)
		return sendForbiddenError(c, ErrImageTokenInvalid)
	}

	media, err := models.GetMedia(tokenInfo.MediaSlug)
	if err != nil {
		log.Errorf("ImageHandler: Failed to get media %s: %v", tokenInfo.MediaSlug, err)
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		log.Errorf("ImageHandler: Media not found for slug: %s", tokenInfo.MediaSlug)
		return sendNotFoundError(c, ErrImageNotFound)
	}

	var chapter *models.Chapter
	if tokenInfo.AssetPath != "" {
		log.Debugf("ImageHandler: Handling light novel asset: %s", tokenInfo.AssetPath)
		// Light novel asset: chapterSlug may be 0 or empty, but should be valid for asset lookup
		chapterSlug := tokenInfo.ChapterSlug
		if chapterSlug == "" || strings.ContainsAny(chapterSlug, "./ ") {
			// Fallback: try to extract from Referer if possible
			referer := c.Get("Referer")
			if referer != "" {
				parts := strings.Split(referer, "/series/")
				if len(parts) > 1 {
					slugParts := strings.Split(parts[1], "/")
					if len(slugParts) > 1 {
						chapterSlug = slugParts[1]
					}
				}
			}
		}
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, tokenInfo.LibrarySlug, chapterSlug)
		if err != nil {
			log.Errorf("ImageHandler: Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, chapterSlug, err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, chapterSlug)
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if isHTMXRequest(c) {
				triggerNotification(c, "Access denied: you don't have permission to view this chapter", "destructive")
				// Return 204 No Content to prevent navigation/swap but show notification
				return c.Status(fiber.StatusNoContent).SendString("")
			}
			return sendForbiddenError(c, ErrImageAccessDenied)
		}
		return serveLightNovelAsset(c, media, chapter, tokenInfo.AssetPath)
	} else {
		// Comic page
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, tokenInfo.LibrarySlug, tokenInfo.ChapterSlug)
		if err != nil {
			log.Errorf("ImageHandler: Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, tokenInfo.ChapterSlug, err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if isHTMXRequest(c) {
				triggerNotification(c, "Access denied: you don't have permission to view this chapter", "destructive")
				// Return 204 No Content to prevent navigation/swap but show notification
				return c.Status(fiber.StatusNoContent).SendString("")
			}
			return sendForbiddenError(c, ErrImageAccessDenied)
		}
		return serveComicPage(c, chapter, tokenInfo.Page)
	}
}

// serveComicPage serves a comic page image
func serveComicPage(c *fiber.Ctx, chapter *models.Chapter, page int) error {
	start := time.Now()
	// log.Infof("serveComicPage: serving page %d for media %s chapter %s", page, media.Slug, chapter.Slug)
	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	filePath := filepath.Join(library.Folders[0], chapter.File)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return sendNotFoundError(c, ErrImageNotFound)
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
			return sendInternalServerError(c, ErrImageProcessingFailed, err)
		}
		c.Set("Content-Type", contentType)
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		imageLoadDuration.WithLabelValues("cbz").Observe(time.Since(start).Seconds())
		imageBytes, contentType, err := ServeComicArchiveFromZIP(filePath, page)
		if err != nil {
			return sendInternalServerError(c, ErrImageProcessingFailed, err)
		}
		c.Set("Content-Type", contentType)
		return c.Send(imageBytes)
	default:
		return sendBadRequestError(c, ErrImageUnsupportedType)
	}
}

// serveLightNovelAsset serves a light novel asset from an EPUB file
func serveLightNovelAsset(c *fiber.Ctx, media *models.Media, chapter *models.Chapter, assetPath string) error {
	start := time.Now()
	log.Debugf("serveLightNovelAsset: serving asset %s for media %s chapter %s", assetPath, media.Slug, chapter.Slug)
	// Determine the actual chapter file path
	library, err := models.GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	chapterFilePath := filepath.Join(library.Folders[0], chapter.File)

	log.Debugf("Light novel asset request: media=%s, chapter=%s, asset=%s, file=%s", media.Slug, chapter.Slug, assetPath, chapterFilePath)

	log.Debugf("Serving asset: %s\n", assetPath)

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapterFilePath)
		return sendNotFoundError(c, ErrImageNotFound)
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapterFilePath)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapterFilePath, err)
		return sendInternalServerError(c, ErrImageProcessingFailed, err)
	}
	defer r.Close()

	// Get the OPF directory
	opfDir, err := files.GetOPFDir(chapterFilePath)
	if err != nil {
		log.Errorf("Error getting OPF dir for %s: %v", chapterFilePath, err)
		return sendInternalServerError(c, ErrImageProcessingFailed, err)
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
		return sendNotFoundError(c, ErrImageNotFound)
	}

	log.Debugf("Asset found in EPUB: %s", assetFullPath)

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return sendInternalServerError(c, ErrImageProcessingFailed, err)
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
		return sendInternalServerError(c, ErrImageProcessingFailed, err)
	}
	metricExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(assetPath), "."))
	imageLoadDuration.WithLabelValues(metricExt).Observe(time.Since(start).Seconds())
	return nil
}

// serveImageFromDirectoryImageHandler handles serving individual image files from a chapter directory.
func serveImageFromDirectoryImageHandler(c *fiber.Ctx, dirPath string, page int) error {
	start := time.Now()
	imagePath, err := GetImagesFromDirectory(dirPath, page)
	if err != nil {
		return sendNotFoundError(c, ErrImageNotFound)
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
