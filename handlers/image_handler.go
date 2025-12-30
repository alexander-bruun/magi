package handlers

import (
	"archive/zip"
	"bytes"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/chai2010/webp"
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

	// Check if image compression is disabled
	cfg, err := models.GetAppConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get config")
	}
	if cfg.DisableWebpConversion {
		// Serve original image without compression or conversion
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
	var buf bytes.Buffer
	// WebP quality is 0-100
	webpQuality := float32(quality)
	if webpQuality < 0 {
		webpQuality = 0
	}
	if webpQuality > 100 {
	}
	err = webp.Encode(&buf, img, &webp.Options{Quality: webpQuality})
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
	return c.Send(buf.Bytes())
}

// handleImageRequest serves images with quality based on user role
func handleImageRequest(c *fiber.Ctx) error {
	if dataManager == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Cache not initialized")
	}

	// Get the requested path (remove /api/images/ or /api/posters/ prefix)
	imagePath := ""
	if strings.HasPrefix(c.Path(), "/api/images/") {
		imagePath = filepath.Join("images", strings.TrimPrefix(c.Path(), "/api/images/"))
	} else if strings.HasPrefix(c.Path(), "/api/posters/") {
		imagePath = filepath.Join("posters", strings.TrimPrefix(c.Path(), "/api/posters/"))
	} else if strings.HasPrefix(c.Path(), "/api/avatars/") {
		imagePath = filepath.Join("avatars", strings.TrimPrefix(c.Path(), "/api/avatars/"))
	}

	if imagePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image path")
	}

	// Check if the file exists in data manager
	exists, err := dataManager.Exists(imagePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Cache error")
	}
	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
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

	// Check if image compression is disabled
	cfg, err := models.GetAppConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get config")
	}
	if cfg.DisableWebpConversion {
		// Serve original image without compression or conversion
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

	// Load the image
	start := time.Now()
	reader, err := dataManager.LoadReader(imagePath)
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
	var buf bytes.Buffer
	// WebP quality is 0-100
	webpQuality := float32(quality)
	if webpQuality < 0 {
		webpQuality = 0
	}
	if webpQuality > 100 {
	}
	err = webp.Encode(&buf, img, &webp.Options{Quality: webpQuality})
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
	return c.Send(buf.Bytes())
}

// ImageHandler serves images for both comics and light novels using token-based authentication
func ImageHandler(c *fiber.Ctx) error {
	// log.Infof("ImageHandler called with token: %s", c.Query("token"))
	// log.Infof("ImageHandler: request from IP %s, User-Agent: %s, Referer: %s", c.IP(), c.Get("User-Agent"), c.Get("Referer"))

	token := c.Query("token")
	log.Debugf("ImageHandler: received token %s", token)

	if token == "" {
		log.Errorf("ImageHandler: token parameter is required")
		return sendBadRequestError(c, ErrImageTokenRequired)
	}

	// log.Infof("ImageHandler: validating token %s", token)
	// Validate the token
	tokenInfo, err := utils.ValidateImageToken(token)
	if err != nil {
		log.Errorf("ImageHandler: Token validation failed for token %s: %v", token, err)
		return sendForbiddenError(c, ErrImageTokenInvalid)
	}

	// log.Infof("ImageHandler: token %s validated successfully, consuming", token)
	// Consume the token
	utils.ConsumeImageToken(token)
	// log.Infof("ImageHandler: token %s consumed", token)

	// Validate MediaSlug to prevent malformed tokens
	if strings.ContainsAny(tokenInfo.MediaSlug, "/,") {
		log.Errorf("ImageHandler: Invalid MediaSlug in token: %s", tokenInfo.MediaSlug)
		return sendForbiddenError(c, ErrImageTokenInvalid)
	}
	// log.Infof("ImageHandler: MediaSlug validated: %s", tokenInfo.MediaSlug)

	media, err := models.GetMedia(tokenInfo.MediaSlug)
	if err != nil {
		log.Errorf("ImageHandler: Failed to get media %s: %v", tokenInfo.MediaSlug, err)
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		log.Errorf("ImageHandler: Media not found for slug: %s", tokenInfo.MediaSlug)
		return sendNotFoundError(c, ErrImageNotFound)
	}
	// log.Infof("ImageHandler: Media found: %s", media.Slug)

	var chapter *models.Chapter
	if tokenInfo.AssetPath != "" {
		log.Debugf("ImageHandler: Handling light novel asset: %s", tokenInfo.AssetPath)
		// Light novel asset: chapterSlug may be 0 or empty, but should be valid for asset lookup
		chapterSlug := tokenInfo.ChapterSlug
		if chapterSlug == "" || strings.ContainsAny(chapterSlug, "./ ") {
			// log.Infof("ImageHandler: ChapterSlug empty or invalid, checking Referer")
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
		// log.Infof("ImageHandler: Using chapterSlug: %s", chapterSlug)
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, chapterSlug)
		if err != nil {
			log.Errorf("ImageHandler: Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, chapterSlug, err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, chapterSlug)
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		// log.Infof("ImageHandler: Chapter found: %s", chapter.Slug)
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if IsHTMXRequest(c) {
				triggerNotification(c, "Access denied: you don't have permission to view this chapter", "destructive")
				// Return 204 No Content to prevent navigation/swap but show notification
				return c.Status(fiber.StatusNoContent).SendString("")
			}
			return sendForbiddenError(c, ErrImageAccessDenied)
		}
		// log.Infof("ImageHandler: Access granted, serving light novel asset")
		return serveLightNovelAsset(c, media, chapter, tokenInfo.AssetPath)
	} else {
		// log.Infof("ImageHandler: Handling comic page: %d", tokenInfo.Page)
		// Comic page
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
		if err != nil {
			log.Errorf("ImageHandler: Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, tokenInfo.ChapterSlug, err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
			return sendNotFoundError(c, ErrChapterNotFound)
		}
		// log.Debugf("ImageHandler: Chapter found: %s", chapter.Slug)
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if IsHTMXRequest(c) {
				triggerNotification(c, "Access denied: you don't have permission to view this chapter", "destructive")
				// Return 204 No Content to prevent navigation/swap but show notification
				return c.Status(fiber.StatusNoContent).SendString("")
			}
			return sendForbiddenError(c, ErrImageAccessDenied)
		}
		// log.Debugf("ImageHandler: Access granted, serving comic page")
		return serveComicPage(c, media, chapter, tokenInfo.Page)
	}
}

// serveComicPage serves a comic page image
func serveComicPage(c *fiber.Ctx, media *models.Media, chapter *models.Chapter, page int) error {
	start := time.Now()
	// log.Infof("serveComicPage: serving page %d for media %s chapter %s", page, media.Slug, chapter.Slug)
	// Determine the actual chapter file path
	// For single-file media (cbz/cbr), media.Path is the file itself
	// For directory-based media, we need to join path and chapter file
	filePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		filePath = filepath.Join(media.Path, chapter.File)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return sendNotFoundError(c, ErrImageNotFound)
	}

	// Get app config for compression settings
	cfg, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
		imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())
		return c.SendFile(filePath)
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		imageLoadDuration.WithLabelValues("cbr").Observe(time.Since(start).Seconds())
		imageBytes, contentType, err := ServeComicArchiveFromRAR(filePath, page, quality, cfg.DisableWebpConversion)
		if err != nil {
			return sendInternalServerError(c, ErrImageProcessingFailed, err)
		}
		c.Set("Content-Type", contentType)
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		imageLoadDuration.WithLabelValues("cbz").Observe(time.Since(start).Seconds())
		imageBytes, contentType, err := ServeComicArchiveFromZIP(filePath, page, quality, cfg.DisableWebpConversion)
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
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

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
	opfDir, err := utils.GetOPFDir(chapterFilePath)
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

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	quality := GetCompressionQualityForUser(userName)

	// Process image for serving
	imageBytes, err := ProcessImageForServing(imagePath, quality)
	c.Set("Content-Type", "image/webp")
	var ext string
	if err != nil {
		// If encoding fails, serve original
		ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
		imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())
		return c.SendFile(imagePath)
	}
	ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), ".")) // Use original file extension for metrics
	imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())
	return c.Send(imageBytes)
}
