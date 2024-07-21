package utils

import (
	"fmt"
	"image"
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

func DownloadImage(downloadDir, fileName, fileUrl string) error {
	// Check if the download directory exists
	_, err := os.Stat(downloadDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("download directory does not exist: %s", downloadDir)
	}

	// Get the file name from the URL
	fileNameFromUrl := filepath.Base(fileUrl)

	// Determine the file extension from the URL
	fileExtension := filepath.Ext(fileNameFromUrl)

	// If no file type provided, keep the original file type
	if !strings.Contains(fileName, ".") {
		fileName += fileExtension
	}

	// Construct the full file path for the original image
	originalFilePath := filepath.Join(downloadDir, strings.TrimSuffix(fileName, fileExtension)+"_original"+fileExtension)

	// Get the data
	resp, err := http.Get(fileUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Decode the image
	img, format, err := image.Decode(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Save the original image
	outFile, err := os.Create(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for original image: %v", err)
	}
	defer outFile.Close()

	switch strings.ToLower(format) {
	case "jpeg":
		err = jpeg.Encode(outFile, img, nil)
	case "png":
		err = png.Encode(outFile, img)
	default:
		return fmt.Errorf("unsupported image format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to save original image: %v", err)
	}

	// Resize and crop the image
	resizedImg := resizeAndCrop(img, targetWidth, targetHeight)

	// Construct the full file path for the resized image
	resizedFilePath := filepath.Join(downloadDir, fileName)

	// Save the resized and cropped image
	resizedFile, err := os.Create(resizedFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for resized image: %v", err)
	}
	defer resizedFile.Close()

	err = jpeg.Encode(resizedFile, resizedImg, nil)
	if err != nil {
		return fmt.Errorf("failed to save resized image: %v", err)
	}

	return nil
}

func resizeAndCrop(img image.Image, targetWidth, targetHeight int) image.Image {
	// Resize while preserving aspect ratio
	resizedImg := resize.Resize(uint(targetWidth), 0, img, resize.Lanczos3)
	// Calculate cropping bounds
	width := resizedImg.Bounds().Dx()
	height := resizedImg.Bounds().Dy()

	if height < targetHeight {
		resizedImg = resize.Resize(0, uint(targetHeight), img, resize.Lanczos3)
		width = resizedImg.Bounds().Dx()
		height = resizedImg.Bounds().Dy()
	}

	// Center crop
	cropX := (width - targetWidth) / 2
	cropY := (height - targetHeight) / 2

	return cropImage(resizedImg, cropX, cropY, targetWidth, targetHeight)
}

// This will obviously cause manga posters with off aspect ratios to get significantly cropped, but this is accpeted to have a uniform poster size.
func cropImage(img image.Image, x, y, width, height int) image.Image {
	rect := image.Rect(x, y, x+width, y+height)
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)
	return croppedImg
}
