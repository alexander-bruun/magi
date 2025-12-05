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
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// handleAvatarRequest serves avatar images with quality based on user role
func handleAvatarRequest(c *fiber.Ctx) error {
	return handleCachedImageRequest(c, "avatars")
}

// handlePosterRequest serves poster images with quality based on user role
func handlePosterRequest(c *fiber.Ctx) error {
	return handleCachedImageRequest(c, "posters")
}

// handleCachedImageRequest serves cached images with quality based on user role
func handleCachedImageRequest(c *fiber.Ctx, subDir string) error {
	// Get the requested path (remove /api/{subDir}/ prefix)
	imagePath := filepath.Join(subDir, strings.TrimPrefix(c.Path(), "/api/"+subDir+"/"))

	if imagePath == "" || imagePath == subDir+"/" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image path")
	}

	// Construct full file path
	fullPath := filepath.Join(savedCacheDirectory, imagePath)

	// Check if the original file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
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

	// Load the image
	start := time.Now()
	file, err := os.Open(fullPath)
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
		return c.SendFile(fullPath)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
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
		return c.SendFile(fullPath)
	}

	// Record load time
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
	if ext == "" {
		ext = "unknown"
	}
	log.Debugf("Recording image load time for %s: %v", ext, time.Since(start).Seconds())
	imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())

	// Encode all images as JPEG for better performance and consistent compression
	var buf bytes.Buffer
	// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	c.Set("Content-Type", "image/jpeg")

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
		return c.SendFile(fullPath)
	}

	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Send(buf.Bytes())
}

// handleImageRequest serves images with quality based on user role
func handleImageRequest(c *fiber.Ctx) error {
	// Determine cache directory based on route
	var cacheDir string
	if strings.HasPrefix(c.Path(), "/api/posters/") {
		cacheDir = savedCacheDirectory
	} else if strings.HasPrefix(c.Path(), "/api/avatars/") {
		cacheDir = savedCacheDirectory
	} else if strings.HasPrefix(c.Path(), "/api/images/") {
		// /api/images/* should not serve cached images
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	} else {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid route")
	}
	
	// Get the requested path (remove /api/images/ or /api/posters/ prefix)
	imagePath := ""
	if strings.HasPrefix(c.Path(), "/api/images/") {
		imagePath = strings.TrimPrefix(c.Path(), "/api/images/")
	} else if strings.HasPrefix(c.Path(), "/api/posters/") {
		imagePath = filepath.Join("posters", strings.TrimPrefix(c.Path(), "/api/posters/"))
	} else if strings.HasPrefix(c.Path(), "/api/avatars/") {
		imagePath = filepath.Join("avatars", strings.TrimPrefix(c.Path(), "/api/avatars/"))
	}
	
	if imagePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image path")
	}

	// Construct full file path
	fullPath := filepath.Join(cacheDir, imagePath)

	// Check if the original file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
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

	// Load the image
	start := time.Now()
	file, err := os.Open(fullPath)
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
		return c.SendFile(fullPath)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
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
		return c.SendFile(fullPath)
	}

	// Record load time
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
	if ext == "" {
		ext = "unknown"
	}
	log.Debugf("Recording image load time for %s: %v", ext, time.Since(start).Seconds())
	imageLoadDuration.WithLabelValues(ext).Observe(time.Since(start).Seconds())

	// Encode all images as JPEG for better performance and consistent compression
	var buf bytes.Buffer
	// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	c.Set("Content-Type", "image/jpeg")

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
		return c.SendFile(fullPath)
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
		return handleErrorWithStatus(c, fmt.Errorf("token parameter is required"), fiber.StatusBadRequest)
	}

	// log.Infof("ImageHandler: validating token %s", token)
	// Validate the token
	tokenInfo, err := utils.ValidateImageToken(token)
	if err != nil {
		log.Errorf("ImageHandler: Token validation failed for token %s: %v", token, err)
		return handleErrorWithStatus(c, fmt.Errorf("invalid or expired token: %w", err), fiber.StatusForbidden)
	}

	// log.Infof("ImageHandler: token %s validated successfully, consuming", token)
	// Consume the token
	utils.ConsumeImageToken(token)
	// log.Infof("ImageHandler: token %s consumed", token)

	// Validate MediaSlug to prevent malformed tokens
	if strings.ContainsAny(tokenInfo.MediaSlug, "/,") {
		log.Errorf("ImageHandler: Invalid MediaSlug in token: %s", tokenInfo.MediaSlug)
		return handleErrorWithStatus(c, fmt.Errorf("invalid token"), fiber.StatusForbidden)
	}
	// log.Infof("ImageHandler: MediaSlug validated: %s", tokenInfo.MediaSlug)

	media, err := models.GetMedia(tokenInfo.MediaSlug)
	if err != nil {
		log.Errorf("ImageHandler: Failed to get media %s: %v", tokenInfo.MediaSlug, err)
		return handleError(c, err)
	}
	if media == nil {
		log.Errorf("ImageHandler: Media not found for slug: %s", tokenInfo.MediaSlug)
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}
	// log.Infof("ImageHandler: Media found: %s", media.Slug)
	
	var chapter *models.Chapter
	if tokenInfo.AssetPath != "" {
		log.Infof("ImageHandler: Handling light novel asset: %s", tokenInfo.AssetPath)
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
			return handleError(c, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, chapterSlug)
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		// log.Infof("ImageHandler: Chapter found: %s", chapter.Slug)
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return handleError(c, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if IsHTMXRequest(c) {
				c.Set("HX-Trigger", `{"showNotification": {"message": "Access denied: you don't have permission to view this chapter", "status": "destructive"}}`)
				return c.Status(fiber.StatusForbidden).SendString("")
			}
			return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
		}
		// log.Infof("ImageHandler: Access granted, serving light novel asset")
		return serveLightNovelAsset(c, media, chapter, tokenInfo.AssetPath)
	} else {
		// log.Infof("ImageHandler: Handling comic page: %d", tokenInfo.Page)
		// Comic page
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
		if err != nil {
			log.Errorf("ImageHandler: Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, tokenInfo.ChapterSlug, err)
			return handleError(c, err)
		}
		if chapter == nil {
			log.Errorf("ImageHandler: Chapter not found: %s/%s", tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		// log.Debugf("ImageHandler: Chapter found: %s", chapter.Slug)
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			log.Errorf("ImageHandler: Error checking access: %v", err)
			return handleError(c, err)
		}
		if !hasAccess {
			log.Errorf("ImageHandler: Access denied for chapter %s", chapter.Slug)
			if IsHTMXRequest(c) {
				c.Set("HX-Trigger", `{"showNotification": {"message": "Access denied: you don't have permission to view this chapter", "status": "destructive"}}`)
				return c.Status(fiber.StatusForbidden).SendString("")
			}
			return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
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
		return handleErrorWithStatus(c, fmt.Errorf("file not found"), fiber.StatusNotFound)
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
		imageBytes, err := ServeComicArchiveFromRAR(filePath, page, 95) // Default quality
		if err != nil {
			return handleErrorWithStatus(c, fmt.Errorf("failed to serve archive: %w", err), fiber.StatusInternalServerError)
		}
		c.Set("Content-Type", "image/jpeg")
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		imageLoadDuration.WithLabelValues("cbz").Observe(time.Since(start).Seconds())
		imageBytes, err := ServeComicArchiveFromZIP(filePath, page, 95) // Default quality
		if err != nil {
			return handleErrorWithStatus(c, fmt.Errorf("failed to serve archive: %w", err), fiber.StatusInternalServerError)
		}
		c.Set("Content-Type", "image/jpeg")
		return c.Send(imageBytes)
	default:
		return handleErrorWithStatus(c, fmt.Errorf("unsupported file type"), fiber.StatusBadRequest)
	}
}

// serveLightNovelAsset serves a light novel asset from an EPUB file
func serveLightNovelAsset(c *fiber.Ctx, media *models.Media, chapter *models.Chapter, assetPath string) error {
	start := time.Now()
	log.Infof("serveLightNovelAsset: serving asset %s for media %s chapter %s", assetPath, media.Slug, chapter.Slug)
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
		return handleErrorWithStatus(c, fmt.Errorf("EPUB file not found"), fiber.StatusNotFound)
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapterFilePath)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapterFilePath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error opening EPUB: %w", err), fiber.StatusInternalServerError)
	}
	defer r.Close()

	// Get the OPF directory
	opfDir, err := utils.GetOPFDir(chapterFilePath)
	if err != nil {
		log.Errorf("Error getting OPF dir for %s: %v", chapterFilePath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error parsing EPUB: %w", err), fiber.StatusInternalServerError)
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
		return handleErrorWithStatus(c, fmt.Errorf("asset not found"), fiber.StatusNotFound)
	}

	log.Debugf("Asset found in EPUB: %s", assetFullPath)

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error opening asset: %w", err), fiber.StatusInternalServerError)
	}
	defer rc.Close()

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

	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		log.Errorf("Error writing asset %s to response: %v", assetPath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error writing asset: %w", err), fiber.StatusInternalServerError)
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
		return handleErrorWithStatus(c, fmt.Errorf("image not found: %w", err), fiber.StatusNotFound)
	}

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	quality := GetCompressionQualityForUser(userName)

	// Process image for serving
	imageBytes, err := ProcessImageForServing(imagePath, quality)
	c.Set("Content-Type", "image/jpeg")
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