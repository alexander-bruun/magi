package files

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	"strings"

	"github.com/alexander-bruun/magi/utils/store"
	"github.com/gofiber/fiber/v3/log"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/tiff" // Register TIFF format
	_ "golang.org/x/image/webp" // Register WebP format
)

// slugKey is the HMAC key used for image URL slug generation and verification.
// Derived from ImageAccessSecret (stored in DB), so it is shared across all prefork children.
// Must be set via SetSlugKey before generating or decrypting slugs.
var slugKey []byte

// SetSlugKey derives the slug HMAC key from the ImageAccessSecret.
// Call this once during startup after the DB config is loaded.
func SetSlugKey(imageAccessSecret string) {
	h := sha256.Sum256([]byte("magi-slug-v1:" + imageAccessSecret))
	slugKey = h[:]
}

// generateKeystream produces a deterministic byte stream for XOR obfuscation.
func generateKeystream(mediaSlug, chapterSlug string, length int) []byte {
	stream := make([]byte, 0, length+32)
	for counter := 0; len(stream) < length; counter++ {
		mac := hmac.New(sha256.New, slugKey)
		mac.Write([]byte(fmt.Sprintf("ks\x00%s\x00%s\x00%d", mediaSlug, chapterSlug, counter)))
		stream = append(stream, mac.Sum(nil)...)
	}
	return stream[:length]
}

// computeTag generates an 8-byte HMAC authentication tag for a payload.
func computeTag(mediaSlug, chapterSlug string, payload []byte) []byte {
	mac := hmac.New(sha256.New, slugKey)
	mac.Write([]byte("tag\x00" + mediaSlug + "\x00" + chapterSlug + "\x00"))
	mac.Write(payload)
	return mac.Sum(nil)[:8]
}

// GeneratePageSlug creates a URL-safe slug that hides the page number.
// The slug encodes the library slug + page number, XORed with an HMAC keystream
// and authenticated with an HMAC tag. Deterministic for the same inputs + key.
func GeneratePageSlug(mediaSlug, librarySlug, chapterSlug string, page int) string {
	payload := []byte(fmt.Sprintf("p\x00%s\x00%d", librarySlug, page))
	ks := generateKeystream(mediaSlug, chapterSlug, len(payload))
	encrypted := make([]byte, len(payload))
	for i := range payload {
		encrypted[i] = payload[i] ^ ks[i]
	}
	tag := computeTag(mediaSlug, chapterSlug, payload)
	return base64.RawURLEncoding.EncodeToString(append(encrypted, tag...))
}

// GenerateAssetSlug creates a URL-safe slug that hides an EPUB asset path.
func GenerateAssetSlug(mediaSlug, librarySlug, chapterSlug, assetPath string) string {
	payload := []byte(fmt.Sprintf("a\x00%s\x00%s", librarySlug, assetPath))
	ks := generateKeystream(mediaSlug, chapterSlug, len(payload))
	encrypted := make([]byte, len(payload))
	for i := range payload {
		encrypted[i] = payload[i] ^ ks[i]
	}
	tag := computeTag(mediaSlug, chapterSlug, payload)
	return base64.RawURLEncoding.EncodeToString(append(encrypted, tag...))
}

// DecryptSlug decrypts a slug and returns the type ("page" or "asset"), library slug, and value.
func DecryptSlug(mediaSlug, chapterSlug, slug string) (slugType, librarySlug, value string, err error) {
	data, err := base64.RawURLEncoding.DecodeString(slug)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid slug encoding")
	}
	if len(data) < 13 { // minimum: type(1) + \x00(1) + lib(1) + \x00(1) + val(1) + tag(8)
		return "", "", "", fmt.Errorf("slug too short")
	}

	encrypted := data[:len(data)-8]
	tag := data[len(data)-8:]

	// Decrypt via XOR
	ks := generateKeystream(mediaSlug, chapterSlug, len(encrypted))
	payload := make([]byte, len(encrypted))
	for i := range encrypted {
		payload[i] = encrypted[i] ^ ks[i]
	}

	// Verify authentication tag
	expectedTag := computeTag(mediaSlug, chapterSlug, payload)
	if !hmac.Equal(tag, expectedTag) {
		return "", "", "", fmt.Errorf("invalid slug")
	}

	// Parse: "type\x00librarySlug\x00value"
	parts := strings.SplitN(string(payload), "\x00", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid slug format")
	}

	switch parts[0] {
	case "p":
		return "page", parts[1], parts[2], nil
	case "a":
		return "asset", parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("unknown slug type")
	}
}

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

type ThumbnailSize struct {
	Name   string
	Width  int
	Height int
}

var allSizes = []ThumbnailSize{
	{"", targetWidth, targetHeight},           // full
	{"_thumb", thumbWidth, thumbHeight},       // thumbnail
	{"_small", smallWidth, smallHeight},       // small
	{"_tiny", tinyWidth, tinyHeight},          // tiny
	{"_display", displayWidth, displayHeight}, // display
}

var standardSizes = []ThumbnailSize{
	{"", targetWidth, targetHeight},           // full
	{"_thumb", thumbWidth, thumbHeight},       // thumbnail
	{"_small", smallWidth, smallHeight},       // small
	{"_tiny", tinyWidth, tinyHeight},          // tiny
	{"_display", displayWidth, displayHeight}, // display
}

// saveOriginal saves the original image to the data backend
func saveOriginal(img image.Image, baseName string, dataBackend *store.FileStore, useWebp bool, originalFormat string) error {
	var format string
	if useWebp {
		format = "webp"
	} else {
		format = originalFormat
	}
	path := fmt.Sprintf("posters/%s_original.%s", baseName, format)
	data, err := EncodeImageToBytes(img, format, 100)
	if err != nil {
		return err
	}
	return dataBackend.Save(path, data)
}

// generateAndSaveThumbnails generates and saves multiple thumbnail sizes
func generateAndSaveThumbnails(img image.Image, baseName string, dataBackend *store.FileStore, useWebp bool, sizes []ThumbnailSize, originalFormat string) error {
	for _, size := range sizes {
		resized := resizeAndCrop(img, size.Width, size.Height)
		var format string
		if useWebp {
			format = "webp"
		} else {
			format = originalFormat
		}
		path := fmt.Sprintf("posters/%s%s.%s", baseName, size.Name, format)
		data, err := EncodeImageToBytes(resized, format, 100)
		if err != nil {
			return err
		}
		if err := dataBackend.Save(path, data); err != nil {
			return err
		}
	}
	return nil
}

// generatePosterURL generates the URL for the poster image
func generatePosterURL(slug string, useWebp bool) string {
	format := "jpg"
	if useWebp {
		format = "webp"
	}
	return fmt.Sprintf("/api/posters/%s.%s", slug, format)
}

// DownloadImageWithThumbnails downloads an image and creates multiple sizes for better performance
func DownloadImageWithThumbnails(fileName, fileUrl string, dataBackend *store.FileStore, useWebp bool) error {

	// Determine file name and extension
	fileNameWithExtension := getFileNameWithExtension(fileName, fileUrl)
	baseName := strings.TrimSuffix(fileNameWithExtension, filepath.Ext(fileNameWithExtension))

	img, format, err := fetchImage(fileUrl)
	if err != nil {
		return err
	}

	if err := saveOriginal(img, baseName, dataBackend, useWebp, format); err != nil {
		return err
	}

	return generateAndSaveThumbnails(img, baseName, dataBackend, useWebp, allSizes, format)
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
func ProcessImageWithCrop(fromPath, toPath string, cropData map[string]any, quality int) error {
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
func applyCropAndResize(img image.Image, cropData map[string]any, targetWidth, targetHeight int) image.Image {
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
func getFloat64(m map[string]any, key string) float64 {
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
	switch {
	case strings.HasSuffix(filePath, ".jpg"), strings.HasSuffix(filePath, ".jpeg"):
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()
		return jpeg.Encode(file, img, &jpeg.Options{Quality: quality})
	case strings.HasSuffix(filePath, ".png"):
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()
		return png.Encode(file, img)
	case strings.HasSuffix(filePath, ".gif"):
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()
		return gif.Encode(file, img, nil)
	case strings.HasSuffix(filePath, ".webp"):
		// Use EncodeImageToBytes for WebP (available in extended build)
		data, err := EncodeImageToBytes(img, "webp", quality)
		if err != nil {
			return fmt.Errorf("failed to encode WebP: %w", err)
		}
		return os.WriteFile(filePath, data, 0644)
	default:
		return fmt.Errorf("unsupported file format: %s", filePath)
	}
}

// GenerateRandomSecret generates a random 32-byte secret encoded as hex
func GenerateRandomSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ExtractPosterImage extracts a poster-sized image from any supported file type (image or archive)
// and caches it with proper processing. For archives, it extracts the first image.
// For tall images (webtoons), it crops from the top to capture the cover/title area.
// Creates multiple sizes: original, full (400x600), thumbnail (200x300), and small (100x150).
// Returns the cached image URL path.
func ExtractPosterImage(filePath, slug string, dataBackend *store.FileStore, useWebp bool) (string, error) {

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
	if err := saveOriginal(img, slug, dataBackend, useWebp, "jpeg"); err != nil {
		return "", err
	}

	// Generate thumbnails
	if err := generateAndSaveThumbnails(img, slug, dataBackend, useWebp, standardSizes, "jpeg"); err != nil {
		return "", err
	}

	log.Debugf("Successfully processed and cached poster images for media '%s'", slug)
	return generatePosterURL(slug, useWebp), nil
}

// ProcessLocalImageWithThumbnails processes a local image file and creates multiple cached sizes
// Creates: original, full (400x600), thumbnail (200x300), and small (100x150) versions
// Returns the URL path for the full-size image
func ProcessLocalImageWithThumbnails(imagePath, slug string, dataBackend *store.FileStore, useWebp bool) (string, error) {

	// Load the image
	img, err := OpenImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}

	// Save original (unprocessed) for potential future use
	if err := saveOriginal(img, slug, dataBackend, useWebp, "jpeg"); err != nil {
		return "", err
	}

	// Generate thumbnails
	if err := generateAndSaveThumbnails(img, slug, dataBackend, useWebp, standardSizes, "jpeg"); err != nil {
		return "", err
	}

	return generatePosterURL(slug, useWebp), nil
}

// DownloadPosterImage downloads an image from a URL and creates multiple cached sizes
// Creates: original, full (400x600), thumbnail (200x300), and small (100x150) versions
// Returns the URL path for the full-size image
func DownloadPosterImage(imageURL, slug string, dataBackend *store.FileStore, useWebp bool) (string, error) {

	// Download and decode the image
	img, format, err := fetchImage(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image from %s: %w", imageURL, err)
	}

	// Save original (unprocessed) for potential future use
	if err := saveOriginal(img, slug, dataBackend, useWebp, format); err != nil {
		return "", err
	}

	// Generate thumbnails
	if err := generateAndSaveThumbnails(img, slug, dataBackend, useWebp, standardSizes, format); err != nil {
		return "", err
	}

	return generatePosterURL(slug, useWebp), nil
}

// GenerateThumbnails generates thumbnail and small versions from a cached full-size image
func GenerateThumbnails(fullImagePath, slug string, dataBackend *store.FileStore) error {
	// Load the full-size image from cache
	data, err := dataBackend.Load(fullImagePath)
	if err != nil {
		return fmt.Errorf("failed to load image from cache: %w", err)
	}

	// Decode the image
	reader := bytes.NewReader(data)
	img, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Determine format from the full image path extension
	ext := strings.ToLower(filepath.Ext(fullImagePath))
	format := "jpeg"
	useWebp := false
	if ext == ".webp" {
		format = "webp"
		useWebp = true
	}

	sizes := []ThumbnailSize{{"_thumb", thumbWidth, thumbHeight}, {"_small", smallWidth, smallHeight}, {"_tiny", tinyWidth, tinyHeight}, {"_display", displayWidth, displayHeight}}
	return generateAndSaveThumbnails(img, slug, dataBackend, useWebp, sizes, format)
}
