package utils

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/nwaples/rardecode"
)

// CountImageFiles counts the number of image files in an archive (zip, cbz, rar, or cbr) or directory.
func CountImageFiles(archiveFilePath string) (int, error) {
	// Check if it's a directory first
	fileInfo, err := os.Stat(archiveFilePath)
	if err != nil {
		log.Debugf("CountImageFiles: stat failed for %s: %v", archiveFilePath, err)
		return 0, err
	}
	
	if fileInfo.IsDir() {
		count, err := countImageFilesInDirectory(archiveFilePath)
		log.Debugf("CountImageFiles: directory %s has %d images", archiveFilePath, count)
		return count, err
	}
	
	// Handle archive files
	lowerPath := strings.ToLower(archiveFilePath)
	if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
		count, err := countImageFilesInZip(archiveFilePath)
		log.Debugf("CountImageFiles: zip %s has %d images, err=%v", archiveFilePath, count, err)
		return count, err
	} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
		count, err := countImageFilesInRar(archiveFilePath)
		log.Debugf("CountImageFiles: rar %s has %d images, err=%v", archiveFilePath, count, err)
		return count, err
	} else {
		log.Debugf("CountImageFiles: unsupported file type: %s", archiveFilePath)
		return 0, fmt.Errorf("unsupported file type: %s", archiveFilePath)
	}
}

// countImageFilesInDirectory counts image files in a directory (for chapter folders).
func countImageFilesInDirectory(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && isImageFile(entry.Name()) {
			count++
		}
	}
	return count, nil
}

// countImageFilesInZip counts the number of image files in a zip archive.
func countImageFilesInZip(zipFilePath string) (int, error) {
	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return 0, err
	}
	defer zipFile.Close()

	imageCount := 0
	for _, file := range zipFile.File {
		if isImageFile(file.Name) {
			imageCount++
		}
	}
	return imageCount, nil
}

// countImageFilesInRar counts the number of image files in a rar archive.
func countImageFilesInRar(rarFilePath string) (int, error) {
	rarFile, err := os.Open(rarFilePath)
	if err != nil {
		return 0, err
	}
	defer rarFile.Close()

	rarReader, err := rardecode.NewReader(rarFile, "")
	if err != nil {
		return 0, err
	}

	imageCount := 0
	for {
		header, err := rarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		if isImageFile(header.Name) {
			imageCount++
		}
	}
	return imageCount, nil
}

// ExtractFirstImage extracts the first image from an archive and saves it to the output folder.
func ExtractFirstImage(archivePath, outputFolder string) error {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".zip", ".cbz":
		return extractFirstImageFromZip(archivePath, outputFolder)
	case ".rar", ".cbr":
		return extractFirstImageFromRar(archivePath, outputFolder)
	default:
		return fmt.Errorf("unsupported archive format: %s", ext)
	}
}

func extractFirstImageFromZip(zipPath, outputFolder string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !strings.Contains(file.Name, "..") {
			if isImageFile(file.Name) {
				return extractZipFile(file, outputFolder)
			}
		}
	}
	return fmt.Errorf("no image file found in the archive")
}

func extractFirstImageFromRar(rarPath, outputFolder string) error {
	file, err := os.Open(rarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return err
	}

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if isImageFile(header.Name) {
			return extractRarFile(reader, header.Name, outputFolder)
		}
	}
	return fmt.Errorf("no image file found in the archive")
}

func extractZipFile(file *zip.File, outputFolder string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	outputPath := filepath.Join(outputFolder, file.Name)
	if !strings.HasPrefix(filepath.Clean(outputPath), filepath.Clean(outputFolder)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", outputPath)
	}
	dst, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func extractRarFile(reader io.Reader, fileName, outputFolder string) error {
	outputPath := filepath.Join(outputFolder, filepath.Base(fileName))
	dst, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, reader)
	return err
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// isImageFile checks if a file is an image based on its extension.
// ExtractAndCacheFirstImage extracts the first image from an archive and caches it with proper resizing.
// For tall images (webtoons), it crops from the top to capture the cover/title area.
// Returns the cached image URL path.
func ExtractAndCacheFirstImage(archivePath, slug, cacheDir string) (string, error) {
	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "magi-extract-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the first image
	if err := ExtractFirstImage(archivePath, tempDir); err != nil {
		return "", fmt.Errorf("failed to extract first image: %w", err)
	}

	// Find the extracted image file
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp directory: %w", err)
	}

	var extractedImagePath string
	for _, entry := range entries {
		if !entry.IsDir() && isImageFile(entry.Name()) {
			extractedImagePath = filepath.Join(tempDir, entry.Name())
			break
		}
	}

	if extractedImagePath == "" {
		return "", fmt.Errorf("no image file found after extraction")
	}

	// Process and cache the image, cropping from the top for tall webtoon pages
	fileExt := filepath.Ext(extractedImagePath)[1:]
	originalFile := filepath.Join(cacheDir, fmt.Sprintf("%s_original.%s", slug, fileExt))
	croppedFile := filepath.Join(cacheDir, fmt.Sprintf("%s.%s", slug, fileExt))

	if err := CopyFile(extractedImagePath, originalFile); err != nil {
		return "", fmt.Errorf("failed to copy image to cache: %w", err)
	}

	if err := ProcessImageWithTopCrop(originalFile, croppedFile); err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	return fmt.Sprintf("/api/images/%s.%s", slug, fileExt), nil
}

// ListImagesInManga returns a list of image paths/URIs from a manga file or directory.
// For directories, it returns the file paths of images.
// For archive files (.cbz, .cbr, .zip, .rar), it returns data URIs of the images.
func ListImagesInManga(mangaPath string) ([]string, error) {
	fileInfo, err := os.Stat(mangaPath)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		// List images in directory
		return listImagesInDirectory(mangaPath)
	}

	// Handle archive files
	lowerPath := strings.ToLower(mangaPath)
	if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
		return listImagesInZip(mangaPath)
	} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
		return listImagesInRar(mangaPath)
	}

	return nil, fmt.Errorf("unsupported file type: %s", mangaPath)
}

func listImagesInDirectory(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var images []string
	for _, entry := range entries {
		if !entry.IsDir() && isImageFile(entry.Name()) {
			images = append(images, filepath.Join(dirPath, entry.Name()))
		}
	}
	return images, nil
}

func listImagesInZip(zipPath string) ([]string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var images []string
	for _, file := range reader.File {
		if isImageFile(file.Name) {
			images = append(images, file.Name)
		}
	}
	return images, nil
}

func listImagesInRar(rarPath string) ([]string, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return nil, err
	}

	var images []string
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if isImageFile(header.Name) {
			images = append(images, header.Name)
		}
	}
	return images, nil
}

// ExtractAndCacheImageWithCrop extracts an image (from a file path or archive) and caches it with optional cropping.
func ExtractAndCacheImageWithCrop(imagePath string, slug string, cropData map[string]interface{}) (string, error) {
	cacheDir := GetCacheDirectory()

	// Extract image to temp location
	tempDir, err := os.MkdirTemp("", "magi-poster-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// For directory-based images, copy directly
	if fileInfo, err := os.Stat(imagePath); err == nil && !fileInfo.IsDir() {
		lowerPath := strings.ToLower(imagePath)
		if !strings.HasSuffix(lowerPath, ".cbz") && !strings.HasSuffix(lowerPath, ".cbr") &&
			!strings.HasSuffix(lowerPath, ".zip") && !strings.HasSuffix(lowerPath, ".rar") {
			// It's a direct image file
			return processCroppedImage(imagePath, slug, cacheDir, cropData)
		}
	}

	// For archive files, we'd need to extract them
	// For now, return error as the listing should give direct paths for directories
	return "", fmt.Errorf("archive image extraction not yet supported for custom posters")
}

func processCroppedImage(imagePath, slug, cacheDir string, cropData map[string]interface{}) (string, error) {
	fileExt := filepath.Ext(imagePath)[1:]
	originalFile := filepath.Join(cacheDir, fmt.Sprintf("%s_original.%s", slug, fileExt))
	croppedFile := filepath.Join(cacheDir, fmt.Sprintf("%s.%s", slug, fileExt))

	// Copy original
	if err := CopyFile(imagePath, originalFile); err != nil {
		return "", fmt.Errorf("failed to copy image: %w", err)
	}

	// Apply cropping if provided
	if err := ProcessImageWithCrop(originalFile, croppedFile, cropData); err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	return fmt.Sprintf("/api/images/%s.%s", slug, fileExt), nil
}

// GetCacheDirectory returns the cache directory path
func GetCacheDirectory() string {
	return "./cache"
}

// GetImageDataURIByIndex extracts an image at the given index and returns a data URI
func GetImageDataURIByIndex(mangaPath string, imageIndex int) (string, error) {
	fileInfo, err := os.Stat(mangaPath)
	if err != nil {
		return "", err
	}

	if fileInfo.IsDir() {
		// For directory-based manga, read images and encode the one at imageIndex
		entries, err := os.ReadDir(mangaPath)
		if err != nil {
			return "", err
		}

		imageCount := 0
		for _, entry := range entries {
			if !entry.IsDir() && isImageFile(entry.Name()) {
				if imageCount == imageIndex {
					imagePath := filepath.Join(mangaPath, entry.Name())
					return imageFileToDataURI(imagePath)
				}
				imageCount++
			}
		}
		return "", fmt.Errorf("image index out of range")
	}

	// For archive files, extract and encode
	lowerPath := strings.ToLower(mangaPath)
	if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
		return getImageFromZipAsDataURI(mangaPath, imageIndex)
	} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
		return getImageFromRarAsDataURI(mangaPath, imageIndex)
	}

	return "", fmt.Errorf("unsupported file type")
}

// imageFileToDataURI reads an image file and encodes it as a data URI
func imageFileToDataURI(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

// getImageFromZipAsDataURI extracts an image from a zip archive and returns as data URI
func getImageFromZipAsDataURI(zipPath string, imageIndex int) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	imageCount := 0
	for _, file := range reader.File {
		if isImageFile(file.Name) {
			if imageCount == imageIndex {
				// Extract and encode this image
				src, err := file.Open()
				if err != nil {
					return "", err
				}
				defer src.Close()

				data, err := io.ReadAll(src)
				if err != nil {
					return "", err
				}

				ext := strings.ToLower(filepath.Ext(file.Name))
				mimeType := "image/jpeg"
				switch ext {
				case ".png":
					mimeType = "image/png"
				case ".gif":
					mimeType = "image/gif"
				case ".webp":
					mimeType = "image/webp"
				}

				encoded := base64.StdEncoding.EncodeToString(data)
				return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
			}
			imageCount++
		}
	}
	return "", fmt.Errorf("image index out of range")
}

// getImageFromRarAsDataURI extracts an image from a rar archive and returns as data URI
func getImageFromRarAsDataURI(rarPath string, imageIndex int) (string, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return "", err
	}

	imageCount := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if isImageFile(header.Name) {
			if imageCount == imageIndex {
				data, err := io.ReadAll(reader)
				if err != nil {
					return "", err
				}

				ext := strings.ToLower(filepath.Ext(header.Name))
				mimeType := "image/jpeg"
				switch ext {
				case ".png":
					mimeType = "image/png"
				case ".gif":
					mimeType = "image/gif"
				case ".webp":
					mimeType = "image/webp"
				}

				encoded := base64.StdEncoding.EncodeToString(data)
				return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
			}
			imageCount++
		}
	}
	return "", fmt.Errorf("image index out of range")
}

// ExtractAndCacheImageWithCropByIndex extracts an image by index with cropping
func ExtractAndCacheImageWithCropByIndex(mangaPath, slug string, imageIndex int, cropData map[string]interface{}) (string, error) {
	cacheDir := GetCacheDirectory()
	tempDir, err := os.MkdirTemp("", "magi-poster-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fileInfo, err := os.Stat(mangaPath)
	if err != nil {
		return "", err
	}

	var imagePath string

	if fileInfo.IsDir() {
		// For directory, find the image at the index
		entries, err := os.ReadDir(mangaPath)
		if err != nil {
			return "", err
		}

		imageCount := 0
		for _, entry := range entries {
			if !entry.IsDir() && isImageFile(entry.Name()) {
				if imageCount == imageIndex {
					imagePath = filepath.Join(mangaPath, entry.Name())
					break
				}
				imageCount++
			}
		}
		if imagePath == "" {
			return "", fmt.Errorf("image index out of range")
		}
	} else {
		// For archive files, extract to temp
		lowerPath := strings.ToLower(mangaPath)
		if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
			imagePath, err = extractImageFromZipToPath(mangaPath, tempDir, imageIndex)
			if err != nil {
				return "", err
			}
		} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
			imagePath, err = extractImageFromRarToPath(mangaPath, tempDir, imageIndex)
			if err != nil {
				return "", err
			}
		}
	}

	return processCroppedImage(imagePath, slug, cacheDir, cropData)
}

// extractImageFromZipToPath extracts a specific image from a zip archive
func extractImageFromZipToPath(zipPath, outputDir string, imageIndex int) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	imageCount := 0
	for _, file := range reader.File {
		if isImageFile(file.Name) {
			if imageCount == imageIndex {
				src, err := file.Open()
				if err != nil {
					return "", err
				}
				defer src.Close()

				outputPath := filepath.Join(outputDir, filepath.Base(file.Name))
				dst, err := os.Create(outputPath)
				if err != nil {
					return "", err
				}
				_, err = io.Copy(dst, src)
				dst.Close()
				return outputPath, err
			}
			imageCount++
		}
	}
	return "", fmt.Errorf("image index out of range")
}

// extractImageFromRarToPath extracts a specific image from a rar archive
func extractImageFromRarToPath(rarPath, outputDir string, imageIndex int) (string, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return "", err
	}

	imageCount := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if isImageFile(header.Name) {
			if imageCount == imageIndex {
				outputPath := filepath.Join(outputDir, filepath.Base(header.Name))
				dst, err := os.Create(outputPath)
				if err != nil {
					return "", err
				}
				_, err = io.Copy(dst, reader)
				dst.Close()
				return outputPath, err
			}
			imageCount++
		}
	}
	return "", fmt.Errorf("image index out of range")
}

// Helper function to extract zip file and return path
func extractZipFileToPath(file *zip.File, outputFolder string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	outputPath := filepath.Join(outputFolder, filepath.Base(file.Name))
	if !strings.HasPrefix(filepath.Clean(outputPath), filepath.Clean(outputFolder)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path: %s", outputPath)
	}
	dst, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return outputPath, err
}

// GetMiddleImageDimensions gets the width and height of the middle image in the first chapter.
// Returns width, height, and error.
func GetMiddleImageDimensions(chapterPath string) (int, int, error) {
	fileInfo, err := os.Stat(chapterPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to stat path: %w", err)
	}

	var images []string

	if fileInfo.IsDir() {
		// Get images from directory
		entries, err := os.ReadDir(chapterPath)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && isImageFile(entry.Name()) {
				images = append(images, filepath.Join(chapterPath, entry.Name()))
			}
		}
	} else {
		// Handle archive files
		lowerPath := strings.ToLower(chapterPath)
		if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
			list, err := listImagesInZip(chapterPath)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to list images in zip: %w", err)
			}
			images = list
		} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
			list, err := listImagesInRar(chapterPath)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to list images in rar: %w", err)
			}
			images = list
		} else {
			return 0, 0, fmt.Errorf("unsupported file type: %s", chapterPath)
		}
	}

	if len(images) == 0 {
		return 0, 0, fmt.Errorf("no images found in chapter")
	}

	// Get the middle image index
	middleIdx := len(images) / 2

	// Extract middle image to temporary location
	tempDir, err := os.MkdirTemp("", "magi-dims-")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var imagePath string

	if fileInfo.IsDir() {
		imagePath = images[middleIdx]
	} else {
		// Extract image from archive
		lowerPath := strings.ToLower(chapterPath)
		if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
			extractedPath, err := extractImageFromZipToPath(chapterPath, tempDir, middleIdx)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to extract image from zip: %w", err)
			}
			imagePath = extractedPath
		} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
			extractedPath, err := extractImageFromRarToPath(chapterPath, tempDir, middleIdx)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to extract image from rar: %w", err)
			}
			imagePath = extractedPath
		}
	}

	// Open and decode the image to get dimensions
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return width, height, nil
}

// IsWebtoonByAspectRatio checks if an image's aspect ratio suggests it's a webtoon.
// Returns true if height is >= 3 times the width.
func IsWebtoonByAspectRatio(width, height int) bool {
	if width <= 0 {
		return false
	}
	return height >= width*3
}

func isImageFile(fileName string) bool {
	lastDotIndex := strings.LastIndex(fileName, ".")
	if lastDotIndex == -1 {
		return false
	}
	ext := strings.ToLower(fileName[lastDotIndex+1:])
	switch ext {
	case "jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp":
		return true
	default:
		return false
	}
}

