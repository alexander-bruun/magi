package utils

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
)

const (
	targetWidth  = 400
	targetHeight = 600
)

// DownloadImage downloads an image from the specified URL, saves it in the original and resized formats.
func DownloadImage(downloadDir, fileName, fileUrl string) error {
	if err := ensureDirExists(downloadDir); err != nil {
		return err
	}

	// Determine file name and extension
	fileNameWithExtension := getFileNameWithExtension(fileName, fileUrl)
	originalFilePath := filepath.Join(downloadDir, strings.TrimSuffix(fileNameWithExtension, filepath.Ext(fileNameWithExtension))+"_original"+filepath.Ext(fileNameWithExtension))

	img, format, err := fetchImage(fileUrl)
	if err != nil {
		return err
	}

	if err := saveImage(originalFilePath, img, format); err != nil {
		return err
	}

	resizedImg := resizeAndCrop(img, targetWidth, targetHeight)
	resizedFilePath := filepath.Join(downloadDir, fileNameWithExtension)
	return saveImage(resizedFilePath, resizedImg, "jpeg")
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
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch image: %v", err)
	}
	defer resp.Body.Close()

	img, format, err := image.Decode(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %v", err)
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
		return jpeg.Encode(file, img, nil)
	case "png":
		return png.Encode(file, img)
	case "gif":
		return gif.Encode(file, img, nil)
	default:
		return fmt.Errorf("unsupported image format: %s", format)
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
		return jpeg.Encode(file, img, nil)
	case strings.HasSuffix(filePath, ".png"):
		return png.Encode(file, img)
	case strings.HasSuffix(filePath, ".gif"):
		return gif.Encode(file, img, nil)
	default:
		return fmt.Errorf("unsupported file format: %s", filePath)
	}
}
