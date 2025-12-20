package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/filestore"
	"github.com/chai2010/webp"
	"github.com/gofiber/fiber/v2/log"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/tiff" // Register TIFF format
	_ "golang.org/x/image/webp" // Register WebP format
)

const (
	targetWidth  = 400
	targetHeight = 600

	// Thumbnail sizes for different use cases
	thumbWidth  = 200
	thumbHeight = 300
	smallWidth  = 100
	smallHeight = 150
	tinyWidth   = 60
	tinyHeight  = 90

	// Display size for highlight posters (matches CSS dimensions)
	displayWidth  = 144
	displayHeight = 216
)

// DownloadImageWithThumbnails downloads an image and creates multiple sizes for better performance
func DownloadImageWithThumbnails(fileName, fileUrl string, cacheBackend filestore.CacheBackend, quality int) error {

	// Determine file name and extension
	fileNameWithExtension := getFileNameWithExtension(fileName, fileUrl)
	baseName := strings.TrimSuffix(fileNameWithExtension, filepath.Ext(fileNameWithExtension))

	img, format, err := fetchImage(fileUrl)
	if err != nil {
		return err
	}

	// Save original (unprocessed) for potential future use
	originalPath := fmt.Sprintf("posters/%s_original%s", baseName, filepath.Ext(fileNameWithExtension))
	originalData, err := EncodeImageToBytes(img, format, quality)
	if err != nil {
		return err
	}
	if err := cacheBackend.Save(originalPath, originalData); err != nil {
		return err
	}

	// Generate full-size version (400x600)
	fullImg := resizeAndCrop(img, targetWidth, targetHeight)
	fullPath := fmt.Sprintf("posters/%s.webp", baseName)
	fullData, err := EncodeImageToBytes(fullImg, "webp", quality)
	if err != nil {
		return err
	}
	if err := cacheBackend.Save(fullPath, fullData); err != nil {
		return err
	}

	// Generate thumbnail version (200x300) for listings
	thumbImg := resizeAndCrop(img, thumbWidth, thumbHeight)
	thumbPath := fmt.Sprintf("posters/%s_thumb.webp", baseName)
	thumbData, err := EncodeImageToBytes(thumbImg, "webp", quality)
	if err != nil {
		return err
	}
	if err := cacheBackend.Save(thumbPath, thumbData); err != nil {
		return err
	}

	// Generate small version (100x150) for compact views
	smallImg := resizeAndCrop(img, smallWidth, smallHeight)
	smallPath := fmt.Sprintf("posters/%s_small.webp", baseName)
	smallData, err := EncodeImageToBytes(smallImg, "webp", quality)
	if err != nil {
		return err
	}
	if err := cacheBackend.Save(smallPath, smallData); err != nil {
		return err
	}

	// Generate tiny version (60x90) for very small displays
	tinyImg := resizeAndCrop(img, tinyWidth, tinyHeight)
	tinyPath := fmt.Sprintf("posters/%s_tiny.webp", baseName)
	tinyData, err := EncodeImageToBytes(tinyImg, "webp", quality)
	if err != nil {
		return err
	}
	if err := cacheBackend.Save(tinyPath, tinyData); err != nil {
		return err
	}

	// Generate display version (144x216) for highlight posters
	displayImg := resizeAndCrop(img, displayWidth, displayHeight)
	displayPath := fmt.Sprintf("posters/%s_display.webp", baseName)
	displayData, err := EncodeImageToBytes(displayImg, "webp", quality)
	if err != nil {
		return err
	}
	return cacheBackend.Save(displayPath, displayData)
}

// ensureDirExists ensures the directory exists, creating it if necessary.
func ensureDirExists(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// getFileNameWithExtension returns the file name with an extension if not already present.
func getFileNameWithExtension(fileName, fileUrl string) string {
	if !strings.Contains(fileName, ".") {
		fileExtension := filepath.Ext(filepath.Base(fileUrl))
		fileName += fileExtension
	}
	return fileName
}

// fetchImage downloads and decodes an image from the URL.
func fetchImage(url string) (image.Image, string, error) {
	// Create request with proper headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}

	// Add user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch image: HTTP %d", resp.StatusCode)
	}

	// Read the response body into memory first to allow multiple decode attempts
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %v", err)
	}

	img, format, err := image.Decode(strings.NewReader(string(data)))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image (format detection failed): %v", err)
	}
	return img, format, nil
}

// saveImage encodes and saves an image to the specified path.
// SaveImage saves an image to the given path with the specified format and quality
// SaveImage saves an image to a file path with the specified format and quality
func SaveImage(filePath string, img image.Image, format string, quality int) error {
	data, err := EncodeImageToBytes(img, format, quality)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// EncodeImageToBytes encodes an image to bytes in the specified format
func EncodeImageToBytes(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
		jpegQuality := quality
		if jpegQuality < 1 {
			jpegQuality = 1
		}
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return nil, err
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case "webp":
		// WebP quality is 0-100, lossy
		webpQuality := float32(quality)
		if webpQuality < 0 {
			webpQuality = 0
		}
		if webpQuality > 100 {
			webpQuality = 100
		}
		if err := webp.Encode(&buf, img, &webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
	default:
		// Unknown format - save as WebP
		webpQuality := float32(quality)
		if webpQuality < 0 {
			webpQuality = 0
		}
		if webpQuality > 100 {
			webpQuality = 100
		}
		if err := webp.Encode(&buf, img, &webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// resizeAndCrop resizes and crops an image to the target dimensions.
func resizeAndCrop(img image.Image, width, height int) image.Image {
	resizedImg := resize.Resize(uint(width), 0, img, resize.Lanczos3)
	if resizedImg.Bounds().Dy() < height {
		resizedImg = resize.Resize(0, uint(height), img, resize.Lanczos3)
	}

	cropX, cropY := calculateCropOffset(resizedImg, width, height)
	return cropImage(resizedImg, cropX, cropY, width, height)
}

// calculateCropOffset calculates the offset for cropping an image.
func calculateCropOffset(img image.Image, targetWidth, targetHeight int) (int, int) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	cropX, cropY := (width-targetWidth)/2, (height-targetHeight)/2
	return cropX, cropY
}

// cropImage crops the image to the specified dimensions.
func cropImage(img image.Image, x, y, width, height int) image.Image {
	rect := image.Rect(x, y, x+width, y+height)
	return img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)
}

// ProcessImage processes an image by resizing and cropping it, then saving it to a new file.
func ProcessImage(fromPath, toPath string, quality int) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := OpenImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := resizeAndCrop(img, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg, quality)
}

// ProcessImageWithTopCrop processes an image by extracting the top portion and resizing it.
// It takes the top square (or near-square) section of the image, useful for cover/poster areas.
func ProcessImageWithTopCrop(fromPath, toPath string, quality int) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := OpenImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := cropFromTopAndResize(img, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg, quality)
}

// ProcessImageWithCrop processes an image by applying user-defined cropping and resizing
func ProcessImageWithCrop(fromPath, toPath string, cropData map[string]interface{}, quality int) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := OpenImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := applyCropAndResize(img, cropData, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg, quality)
}

// applyCropAndResize applies user-defined crop coordinates and resizes the image
func applyCropAndResize(img image.Image, cropData map[string]interface{}, targetWidth, targetHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Extract crop coordinates from map
	x := int(getFloat64(cropData, "x"))
	y := int(getFloat64(cropData, "y"))
	cropWidth := int(getFloat64(cropData, "width"))
	cropHeight := int(getFloat64(cropData, "height"))

	// Validate coordinates
	if cropWidth <= 0 {
		cropWidth = width
	}
	if cropHeight <= 0 {
		cropHeight = height
	}

	// Clamp to image bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+cropWidth > width {
		cropWidth = width - x
	}
	if y+cropHeight > height {
		cropHeight = height - y
	}

	// Apply crop
	if cropWidth > 0 && cropHeight > 0 {
		croppedRect := image.Rect(x, y, x+cropWidth, y+cropHeight)
		img = img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(croppedRect)
	}

	// Resize to target dimensions
	resizedImg := resize.Resize(uint(targetWidth), 0, img, resize.Lanczos3)
	if resizedImg.Bounds().Dy() < targetHeight {
		resizedImg = resize.Resize(0, uint(targetHeight), img, resize.Lanczos3)
	}

	cropX, cropY := calculateCropOffset(resizedImg, targetWidth, targetHeight)
	return cropImage(resizedImg, cropX, cropY, targetWidth, targetHeight)
}

// getFloat64 safely retrieves a float64 value from a map
func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// cropFromTopAndResize crops a poster-sized region from the top of the image and then resizes it.
// This is useful for extracting cover/title areas from tall webtoon pages.
// Poster aspect ratio is 2:3 (width:height)
func cropFromTopAndResize(img image.Image, targetWidth, targetHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate poster-sized crop from the top
	// Poster aspect ratio is 2:3 (width:height)
	posterAspectRatio := 2.0 / 3.0
	cropWidth := width
	cropHeight := int(float64(cropWidth) / posterAspectRatio)

	// If the calculated crop height exceeds image height, adjust width to fit
	if cropHeight > height {
		cropHeight = height
		cropWidth = int(float64(cropHeight) * posterAspectRatio)
	}

	// Crop from the top (y=0)
	croppedRect := image.Rect(0, 0, cropWidth, cropHeight)
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(croppedRect)

	// Now resize and center crop to target dimensions
	resizedImg := resize.Resize(uint(targetWidth), 0, croppedImg, resize.Lanczos3)
	if resizedImg.Bounds().Dy() < targetHeight {
		resizedImg = resize.Resize(0, uint(targetHeight), croppedImg, resize.Lanczos3)
	}

	cropX, cropY := calculateCropOffset(resizedImg, targetWidth, targetHeight)
	return cropImage(resizedImg, cropX, cropY, targetWidth, targetHeight)
}

// checkFileExists checks if a file exists.
func checkFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", path)
	}
	return nil
}

// openImage opens and decodes an image file.
// OpenImage opens and decodes an image from the given path
func OpenImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	return img, nil
}

// saveProcessedImage encodes and saves a processed image to the specified path.
func saveProcessedImage(filePath string, img image.Image, quality int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	switch {
	case strings.HasSuffix(filePath, ".jpg"), strings.HasSuffix(filePath, ".jpeg"):
		return jpeg.Encode(file, img, &jpeg.Options{Quality: quality})
	case strings.HasSuffix(filePath, ".png"):
		return png.Encode(file, img)
	case strings.HasSuffix(filePath, ".gif"):
		return gif.Encode(file, img, nil)
	default:
		return fmt.Errorf("unsupported file format: %s", filePath)
	}
}

// GenerateSignedImageURL generates a signed URL for image access with expiration
func GenerateSignedImageURL(baseURL, secret, mediaSlug, chapterSlug string, page int, expiration time.Duration) string {
	expires := time.Now().Add(expiration).Unix()
	data := fmt.Sprintf("%s:%s:%d:%d", mediaSlug, chapterSlug, page, expires)

	// Create HMAC signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))

	// Build the signed URL
	return fmt.Sprintf("%s?media=%s&chapter=%s&page=%d&expires=%d&signature=%s",
		baseURL, mediaSlug, chapterSlug, page, expires, signature)
}

// ValidateImageSignature validates the signature and expiration of an image access request
func ValidateImageSignature(secret, mediaSlug, chapterSlug, pageStr, expiresStr, signatureStr string) error {
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return fmt.Errorf("invalid page number")
	}

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expiration time")
	}

	// Check if expired
	if time.Now().Unix() > expires {
		return fmt.Errorf("signature expired")
	}

	// Recreate the data string
	data := fmt.Sprintf("%s:%s:%d:%d", mediaSlug, chapterSlug, page, expires)

	// Verify signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signatureStr), []byte(expectedSignature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// GenerateRandomSecret generates a random 32-byte secret encoded as hex
func GenerateRandomSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ImageAccessToken represents a one-time use token for image access
type ImageAccessToken struct {
	MediaSlug   string
	ChapterSlug string
	Page        int
	AssetPath   string // For light novel assets
	ExpiresAt   time.Time
	UsedCount   int
}

// Global token store
var tokens sync.Map // map[string]*ImageAccessToken

// GenerateImageAccessToken generates a one-time use token for image access
func GenerateImageAccessToken(mediaSlug, chapterSlug string, page int) string {
	return GenerateImageAccessTokenWithAssetAndValidity(mediaSlug, chapterSlug, page, "", 5) // default 5 minutes
}

// GenerateImageAccessTokenWithValidity generates a one-time use token for image access with custom validity
func GenerateImageAccessTokenWithValidity(mediaSlug, chapterSlug string, page int, validityMinutes int) string {
	return GenerateImageAccessTokenWithAssetAndValidity(mediaSlug, chapterSlug, page, "", validityMinutes)
}

// GenerateImageAccessTokenWithAsset generates a one-time use token for image access with optional asset path
func GenerateImageAccessTokenWithAsset(mediaSlug, chapterSlug string, page int, assetPath string) string {
	return GenerateImageAccessTokenWithAssetAndValidity(mediaSlug, chapterSlug, page, assetPath, 5) // default 5 minutes
}

// GenerateImageAccessTokenWithAssetAndValidity generates a one-time use token for image access with optional asset path and custom validity
func GenerateImageAccessTokenWithAssetAndValidity(mediaSlug, chapterSlug string, page int, assetPath string, validityMinutes int) string {
	log.Debugf("Generating token for mediaSlug=%s, chapterSlug=%s, assetPath=%s", mediaSlug, chapterSlug, assetPath)
	tokenBytes := make([]byte, 32)
	var token string
	if _, err := rand.Read(tokenBytes); err == nil {
		token = hex.EncodeToString(tokenBytes)
	} else {
		// Fallback to UUID if crypto/rand fails
		uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", time.Now().UnixNano(), page, len(mediaSlug), len(chapterSlug), time.Now().UnixNano())
		token = uuid
		log.Errorf("crypto/rand failed for token generation, using UUID fallback: %s", token)
	}

	// Store the token
	tokens.Store(token, &ImageAccessToken{
		MediaSlug:   mediaSlug,
		ChapterSlug: chapterSlug,
		Page:        page,
		AssetPath:   assetPath,
		ExpiresAt:   time.Now().Add(time.Duration(validityMinutes) * time.Minute),
	})
	log.Debugf("Stored token %s: media=%s, chapter=%s, page=%d, asset=%s", token, mediaSlug, chapterSlug, page, assetPath)

	return token
}

// ValidateImageToken validates a token (reusable)
func ValidateImageToken(token string) (*ImageAccessToken, error) {
	// log.Infof("ValidateImageToken: attempting to validate token %s", token)
	val, ok := tokens.Load(token)
	if !ok {
		return nil, fmt.Errorf("Token %s is invalid: not found", token)
	}
	tokenInfo := val.(*ImageAccessToken)

	// log.Infof("Retrieved token %s: media=%s, chapter=%s, page=%d, asset=%s", token, tokenInfo.MediaSlug, tokenInfo.ChapterSlug, tokenInfo.Page, tokenInfo.AssetPath)

	// Check expiration
	if time.Now().After(tokenInfo.ExpiresAt) {
		tokens.Delete(token)
		return nil, fmt.Errorf("Token %s is invalid: expired at %v (now: %v)", token, tokenInfo.ExpiresAt, time.Now())
	}

	log.Debugf("Token %s validated successfully", token)

	return tokenInfo, nil
}

// ConsumeImageToken consumes a validated token
func ConsumeImageToken(token string) {
	tokens.Delete(token)
	// log.Infof("Token %s consumed", token)
}

// CleanupExpiredTokens removes expired tokens from the store
func CleanupExpiredTokens() {
	now := time.Now()
	tokens.Range(func(key, value interface{}) bool {
		token := key.(string)
		info := value.(*ImageAccessToken)
		if now.After(info.ExpiresAt) {
			tokens.Delete(token)
			log.Debugf("Expired token: %s", token)
		}
		return true
	})
}

// ExtractPosterImage extracts a poster-sized image from any supported file type (image or archive)
// and caches it with proper processing. For archives, it extracts the first image.
// For tall images (webtoons), it crops from the top to capture the cover/title area.
// Creates multiple sizes: original, full (400x600), thumbnail (200x300), and small (100x150).
// Returns the cached image URL path.
func ExtractPosterImage(filePath, slug string, cacheBackend filestore.CacheBackend, quality int) (string, error) {

	log.Debugf("Extracting poster image from '%s' for media '%s'", filePath, slug)

	var img image.Image
	var err error

	// Check if it's a regular image file
	if isImageFile(filepath.Base(filePath)) {
		// Process the image directly
		img, err = OpenImage(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to open image: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(filePath), ".zip") || strings.HasSuffix(strings.ToLower(filePath), ".cbz") ||
		strings.HasSuffix(strings.ToLower(filePath), ".rar") || strings.HasSuffix(strings.ToLower(filePath), ".cbr") ||
		strings.HasSuffix(strings.ToLower(filePath), ".epub") {

		// Create a temporary directory for extraction
		tempDir, err := os.MkdirTemp("", "magi-poster-extract-")
		if err != nil {
			return "", fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		// Extract the first image
		if err := ExtractFirstImage(filePath, tempDir); err != nil {
			// If archive is invalid or has no images, log and skip rather than failing
			if strings.Contains(err.Error(), "invalid or corrupt") ||
				strings.Contains(err.Error(), "no image files found") {
				log.Debugf("Skipping invalid or empty archive '%s' for media '%s': %v", filePath, slug, err)
				return "", nil
			}
			log.Errorf("Failed to extract first image from '%s' for media '%s': %v", filePath, slug, err)
			return "", fmt.Errorf("failed to extract first image: %w", err)
		}

		log.Debugf("Successfully extracted image from archive '%s' for media '%s'", filePath, slug)

		// Find the extracted image file (search recursively)
		var extractedImagePath string
		err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && isImageFile(d.Name()) {
				extractedImagePath = path
				log.Debugf("Found extracted image '%s' for media '%s'", d.Name(), slug)
				return fs.SkipAll // Stop after finding the first image
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to walk temp directory: %w", err)
		}

		if extractedImagePath == "" {
			log.Debugf("No image file found after extraction from '%s' for media '%s'", filePath, slug)
			return "", fmt.Errorf("no image file found after extraction")
		}

		// Load the extracted image
		img, err = OpenImage(extractedImagePath)
		if err != nil {
			return "", fmt.Errorf("failed to open extracted image: %w", err)
		}
	} else {
		return "", fmt.Errorf("unsupported file type for poster extraction: %s", filePath)
	}

	// Save original (unprocessed) for potential future use
	originalPath := fmt.Sprintf("posters/%s_original.webp", slug)
	originalData, err := EncodeImageToBytes(img, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode original image: %w", err)
	}
	if err := cacheBackend.Save(originalPath, originalData); err != nil {
		return "", fmt.Errorf("failed to save original image: %w", err)
	}

	// Generate full-size version (400x600)
	fullImg := resizeAndCrop(img, targetWidth, targetHeight)
	fullPath := fmt.Sprintf("posters/%s.webp", slug)
	fullData, err := EncodeImageToBytes(fullImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode full-size image: %w", err)
	}
	if err := cacheBackend.Save(fullPath, fullData); err != nil {
		return "", fmt.Errorf("failed to save full-size image: %w", err)
	}

	// Generate thumbnail version (200x300) for listings
	thumbImg := resizeAndCrop(img, thumbWidth, thumbHeight)
	thumbPath := fmt.Sprintf("posters/%s_thumb.webp", slug)
	thumbData, err := EncodeImageToBytes(thumbImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode thumbnail image: %w", err)
	}
	if err := cacheBackend.Save(thumbPath, thumbData); err != nil {
		return "", fmt.Errorf("failed to save thumbnail image: %w", err)
	}

	// Generate small version (100x150) for compact views
	smallImg := resizeAndCrop(img, smallWidth, smallHeight)
	smallPath := fmt.Sprintf("posters/%s_small.webp", slug)
	smallData, err := EncodeImageToBytes(smallImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to save small image: %w", err)
	}
	if err := cacheBackend.Save(smallPath, smallData); err != nil {
		return "", fmt.Errorf("failed to save small image: %w", err)
	}

	log.Debugf("Successfully processed and cached poster images for media '%s'", slug)
	return fmt.Sprintf("/api/posters/%s.webp?v=%s", slug, GenerateRandomString(8)), nil
}

// ProcessLocalImageWithThumbnails processes a local image file and creates multiple cached sizes
// Creates: original, full (400x600), thumbnail (200x300), and small (100x150) versions
// Returns the URL path for the full-size image
func ProcessLocalImageWithThumbnails(imagePath, slug string, cacheBackend filestore.CacheBackend, quality int) (string, error) {

	// Load the image
	img, err := OpenImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}

	// Save original (unprocessed) for potential future use
	originalPath := fmt.Sprintf("posters/%s_original.webp", slug)
	originalData, err := EncodeImageToBytes(img, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode original image: %w", err)
	}
	if err := cacheBackend.Save(originalPath, originalData); err != nil {
		return "", fmt.Errorf("failed to save original image: %w", err)
	}

	// Generate full-size version (400x600)
	fullImg := resizeAndCrop(img, targetWidth, targetHeight)
	fullPath := fmt.Sprintf("posters/%s.webp", slug)
	fullData, err := EncodeImageToBytes(fullImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode full-size image: %w", err)
	}
	if err := cacheBackend.Save(fullPath, fullData); err != nil {
		return "", fmt.Errorf("failed to save full-size image: %w", err)
	}

	// Generate thumbnail version (200x300) for listings
	thumbImg := resizeAndCrop(img, thumbWidth, thumbHeight)
	thumbPath := fmt.Sprintf("posters/%s_thumb.webp", slug)
	thumbData, err := EncodeImageToBytes(thumbImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to encode thumbnail image: %w", err)
	}
	if err := cacheBackend.Save(thumbPath, thumbData); err != nil {
		return "", fmt.Errorf("failed to save thumbnail image: %w", err)
	}

	// Generate small version (100x150) for compact views
	smallImg := resizeAndCrop(img, smallWidth, smallHeight)
	smallPath := fmt.Sprintf("posters/%s_small.webp", slug)
	smallData, err := EncodeImageToBytes(smallImg, "webp", quality)
	if err != nil {
		return "", fmt.Errorf("failed to save small image: %w", err)
	}
	if err := cacheBackend.Save(smallPath, smallData); err != nil {
		return "", fmt.Errorf("failed to save small image: %w", err)
	}

	return fmt.Sprintf("/api/posters/%s.webp?v=%s", slug, GenerateRandomString(8)), nil
}

// GenerateThumbnails generates thumbnail and small versions from a cached full-size image
func GenerateThumbnails(fullImagePath, slug string, cacheBackend filestore.CacheBackend, quality int) error {
	// Load the full-size image from cache
	data, err := cacheBackend.Load(fullImagePath)
	if err != nil {
		return fmt.Errorf("failed to load image from cache: %w", err)
	}

	// Decode the image
	reader := bytes.NewReader(data)
	img, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Generate thumbnail version (200x300) for listings
	thumbImg := resizeAndCrop(img, thumbWidth, thumbHeight)
	thumbPath := fmt.Sprintf("posters/%s_thumb.webp", slug)
	thumbData, err := EncodeImageToBytes(thumbImg, "webp", quality)
	if err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}
	if err := cacheBackend.Save(thumbPath, thumbData); err != nil {
		return fmt.Errorf("failed to save thumbnail: %w", err)
	}

	// Generate small version (100x150) for compact views
	smallImg := resizeAndCrop(img, smallWidth, smallHeight)
	smallPath := fmt.Sprintf("posters/%s_small.webp", slug)
	smallData, err := EncodeImageToBytes(smallImg, "webp", quality)
	if err != nil {
		return fmt.Errorf("failed to encode small image: %w", err)
	}
	return cacheBackend.Save(smallPath, smallData)
}
