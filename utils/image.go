package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/webp" // Register WebP format
)

const (
	targetWidth  = 400
	targetHeight = 600
	jpegQuality  = 85 // Increased from default ~75 to 85 for better quality/size balance

	// Thumbnail sizes for different use cases
	thumbWidth  = 200
	thumbHeight = 300
	smallWidth  = 100
	smallHeight = 150
)

// DownloadImageWithThumbnails downloads an image and creates multiple sizes for better performance
func DownloadImageWithThumbnails(downloadDir, fileName, fileUrl string) error {
	if err := ensureDirExists(downloadDir); err != nil {
		return err
	}

	// Determine file name and extension
	fileNameWithExtension := getFileNameWithExtension(fileName, fileUrl)
	baseName := strings.TrimSuffix(fileNameWithExtension, filepath.Ext(fileNameWithExtension))
	
	img, format, err := fetchImage(fileUrl)
	if err != nil {
		return err
	}

	// Save original (unprocessed) for potential future use
	originalFilePath := filepath.Join(downloadDir, baseName+"_original"+filepath.Ext(fileNameWithExtension))
	if err := saveImage(originalFilePath, img, format); err != nil {
		return err
	}

	// Generate full-size version (400x600)
	fullImg := resizeAndCrop(img, targetWidth, targetHeight)
	fullFilePath := filepath.Join(downloadDir, fileNameWithExtension)
	if err := saveImage(fullFilePath, fullImg, "jpeg"); err != nil {
		return err
	}

	// Generate thumbnail version (200x300) for listings
	thumbImg := resizeAndCrop(img, thumbWidth, thumbHeight)
	thumbFilePath := filepath.Join(downloadDir, baseName+"_thumb.jpg")
	if err := saveImage(thumbFilePath, thumbImg, "jpeg"); err != nil {
		return err
	}

	// Generate small version (100x150) for compact views
	smallImg := resizeAndCrop(img, smallWidth, smallHeight)
	smallFilePath := filepath.Join(downloadDir, baseName+"_small.jpg")
	return saveImage(smallFilePath, smallImg, "jpeg")
}

// ensureDirExists checks if the directory exists; if not, returns an error.
func ensureDirExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}
	return nil
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
func saveImage(filePath string, img image.Image, format string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality})
	case "png":
		return png.Encode(file, img)
	case "gif":
		return gif.Encode(file, img, nil)
	case "webp":
		return webp.Encode(file, img, &webp.Options{Quality: float32(jpegQuality)})
	default:
		// Unknown format - save as progressive JPEG
		return jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality})
	}
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
func ProcessImage(fromPath, toPath string) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := openImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := resizeAndCrop(img, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg)
}

// ProcessImageWithTopCrop processes an image by extracting the top portion and resizing it.
// It takes the top square (or near-square) section of the image, useful for cover/poster areas.
func ProcessImageWithTopCrop(fromPath, toPath string) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := openImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := cropFromTopAndResize(img, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg)
}

// ProcessImageWithCrop processes an image by applying user-defined cropping and resizing
func ProcessImageWithCrop(fromPath, toPath string, cropData map[string]interface{}) error {
	if err := checkFileExists(fromPath); err != nil {
		return err
	}

	img, err := openImage(fromPath)
	if err != nil {
		return err
	}

	processedImg := applyCropAndResize(img, cropData, targetWidth, targetHeight)
	return saveProcessedImage(toPath, processedImg)
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
func openImage(path string) (image.Image, error) {
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
func saveProcessedImage(filePath string, img image.Image) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	switch {
	case strings.HasSuffix(filePath, ".jpg"), strings.HasSuffix(filePath, ".jpeg"):
		return jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality})
	case strings.HasSuffix(filePath, ".png"):
		return png.Encode(file, img)
	case strings.HasSuffix(filePath, ".gif"):
		return gif.Encode(file, img, nil)
	default:
		return fmt.Errorf("unsupported file format: %s", filePath)
	}
}

// ConvertToWebP converts an image file to WebP format in memory
func ConvertToWebP(sourcePath string) ([]byte, error) {
	// Load the source image
	img, err := openImage(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load image for WebP conversion: %w", err)
	}

	// Encode to WebP in memory
	var buf bytes.Buffer
	err = webp.Encode(&buf, img, &webp.Options{Quality: 75})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image to WebP: %w", err)
	}

	return buf.Bytes(), nil
}

// ConvertImageToWebP converts an image.Image to WebP format in memory
func ConvertImageToWebP(img image.Image) ([]byte, error) {
	// Encode to WebP in memory
	var buf bytes.Buffer
	err := webp.Encode(&buf, img, &webp.Options{Quality: 75})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image to WebP: %w", err)
	}

	return buf.Bytes(), nil
}
