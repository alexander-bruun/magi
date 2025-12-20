package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/nwaples/rardecode"
)

// ImageServeData holds data needed for serving images
type ImageServeData struct {
	FilePath string
	IsDir    bool
	UserRole string
}

// GetImageServeData retrieves data needed for serving comic images
func GetImageServeData(mediaSlug, chapterSlug string) (*ImageServeData, error) {
	media, err := models.GetMedia(mediaSlug)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, nil // Not found
	}

	chapter, err := models.GetChapter(mediaSlug, chapterSlug)
	if err != nil {
		return nil, err
	}
	if chapter == nil {
		return nil, nil // Not found
	}

	// Determine the actual chapter file path
	filePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		filePath = filepath.Join(media.Path, chapter.File)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return &ImageServeData{
		FilePath: filePath,
		IsDir:    fileInfo.IsDir(),
	}, nil
}

// GetCompressionQualityForUser gets the compression quality for a user
func GetCompressionQualityForUser(userName string) int {
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			return models.GetCompressionQualityForRole(user.Role)
		} else {
			return models.GetCompressionQualityForRole("reader")
		}
	}
	return models.GetCompressionQualityForRole("anonymous")
}

// ProcessImageForServing processes an image for serving with compression
func ProcessImageForServing(filePath string, quality int) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetImagesFromDirectory gets image files from a directory
func GetImagesFromDirectory(dirPath string, page int) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

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

	sort.Strings(imageFiles)

	if page < 1 || page > len(imageFiles) {
		return "", fmt.Errorf("page %d out of range", page)
	}

	return filepath.Join(dirPath, imageFiles[page-1]), nil
}

// ServeComicArchiveFromZIP serves an image from a ZIP archive
func ServeComicArchiveFromZIP(filePath string, page int, quality int) ([]byte, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Get sorted list of image files
	var imageFiles []string
	for _, f := range r.File {
		lowerName := strings.ToLower(f.Name)
		if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
			strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".gif") ||
			strings.HasSuffix(lowerName, ".webp") {
			imageFiles = append(imageFiles, f.Name)
		}
	}

	sort.Strings(imageFiles)

	if page < 1 || page > len(imageFiles) {
		return nil, fmt.Errorf("page %d out of range", page)
	}

	file := r.File[page-1]
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	img, _, err := image.Decode(rc)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ServeComicArchiveFromRAR serves an image from a RAR archive
func ServeComicArchiveFromRAR(filePath string, page int, quality int) ([]byte, error) {
	r, err := rardecode.OpenReader(filePath, "")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Get sorted list of image files
	var imageFiles []*rardecode.FileHeader
	for {
		header, err := r.Next()
		if err != nil {
			break
		}
		lowerName := strings.ToLower(header.Name)
		if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
			strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".gif") ||
			strings.HasSuffix(lowerName, ".webp") {
			imageFiles = append(imageFiles, header)
		}
	}

	sort.Slice(imageFiles, func(i, j int) bool {
		return imageFiles[i].Name < imageFiles[j].Name
	})

	if page < 1 || page > len(imageFiles) {
		return nil, fmt.Errorf("page %d out of range", page)
	}

	// Skip to the desired file
	r, err = rardecode.OpenReader(filePath, "")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for i := 0; i < page; i++ {
		_, err := r.Next()
		if err != nil {
			return nil, err
		}
	}

	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ComicHandler processes requests to serve comic book pages based on the provided query parameters.
func ComicHandler(c *fiber.Ctx) error {
	token := c.Query("token")
	
	if token == "" {
		return sendBadRequestError(c, ErrComicTokenRequired)
	}

	// Validate the token
	tokenInfo, err := utils.ValidateImageToken(token)
	if err != nil {
		return sendForbiddenError(c, ErrComicTokenInvalid)
	}

	// Consume the token after the response is sent
	defer utils.ConsumeImageToken(token)

	// Get image serve data from service
	imageData, err := GetImageServeData(tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if imageData == nil {
		return sendNotFoundError(c, ErrComicNotFound)
	}

	// If the path is a directory, serve images from within it
	if imageData.IsDir {
		return serveImageFromDirectory(c, imageData.FilePath, tokenInfo.Page)
	}

	lowerFileName := strings.ToLower(filepath.Base(imageData.FilePath))

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	quality := GetCompressionQualityForUser(userName)

	// Serve the file based on its extension
	switch {
	case strings.HasSuffix(lowerFileName, ".jpg"), strings.HasSuffix(lowerFileName, ".jpeg"),
		strings.HasSuffix(lowerFileName, ".png"), strings.HasSuffix(lowerFileName, ".webp"),
		strings.HasSuffix(lowerFileName, ".gif"):
		// Process image for serving
		imageBytes, err := ProcessImageForServing(imageData.FilePath, quality)
		c.Set("Content-Type", "image/jpeg")
		if err != nil {
			// If encoding fails, serve original
			return c.SendFile(imageData.FilePath)
		}
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		imageBytes, err := ServeComicArchiveFromRAR(imageData.FilePath, tokenInfo.Page, quality)
		if err != nil {
			return HandleView(c, views.Error(err.Error()))
		}
		c.Set("Content-Type", "image/jpeg")
		return c.Send(imageBytes)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		imageBytes, err := ServeComicArchiveFromZIP(imageData.FilePath, tokenInfo.Page, quality)
		if err != nil {
			return HandleView(c, views.Error(err.Error()))
		}
		c.Set("Content-Type", "image/jpeg")
		return c.Send(imageBytes)
	default:
		return HandleView(c, views.Error("Unsupported file type"))
	}
}

// serveImageFromDirectory handles serving individual image files from a chapter directory.
func serveImageFromDirectory(c *fiber.Ctx, dirPath string, page int) error {
	imagePath, err := GetImagesFromDirectory(dirPath, page)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	quality := GetCompressionQualityForUser(userName)

	// Process image for serving
	imageBytes, err := ProcessImageForServing(imagePath, quality)
	c.Set("Content-Type", "image/jpeg")
	if err != nil {
		// If encoding fails, serve original
		return c.SendFile(imagePath)
	}
	return c.Send(imageBytes)
}