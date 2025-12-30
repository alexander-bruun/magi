package utils

import (
	"archive/zip"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/nwaples/rardecode/v2"
)

var dataDirectory = "./data"

// ArchiveIterator provides a common interface for iterating through archive files
type ArchiveIterator interface {
	Next() (name string, reader io.ReadCloser, err error)
	Close() error
}

// SetDataDirectory sets the data directory path
func SetDataDirectory(dir string) {
	dataDirectory = dir
}

// zipIterator implements ArchiveIterator for ZIP files
type zipIterator struct {
	reader *zip.ReadCloser
	files  []*zip.File
	index  int
}

func (z *zipIterator) Next() (string, io.ReadCloser, error) {
	if z.index >= len(z.files) {
		return "", nil, io.EOF
	}
	file := z.files[z.index]
	z.index++
	reader, err := file.Open()
	return file.Name, reader, err
}

func (z *zipIterator) Close() error {
	return z.reader.Close()
}

// rarIterator implements ArchiveIterator for RAR files
type rarIterator struct {
	reader *rardecode.Reader
	file   *os.File
}

func (r *rarIterator) Next() (string, io.ReadCloser, error) {
	header, err := r.reader.Next()
	if err != nil {
		return "", nil, err
	}
	// For RAR, we return a wrapper that implements ReadCloser
	return header.Name, &rarReadCloser{reader: r.reader}, nil
}

func (r *rarIterator) Close() error {
	return r.file.Close()
}

// rarReadCloser wraps the RAR reader to implement io.ReadCloser
type rarReadCloser struct {
	reader *rardecode.Reader
}

func (r *rarReadCloser) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *rarReadCloser) Close() error {
	// RAR reader doesn't need explicit closing per file
	return nil
}

// epubIterator implements ArchiveIterator for EPUB files with prioritized image directories
type epubIterator struct {
	reader      *zip.ReadCloser
	files       []*zip.File
	index       int
	prioritized []string
}

func (e *epubIterator) Next() (string, io.ReadCloser, error) {
	if e.index >= len(e.files) {
		return "", nil, io.EOF
	}
	file := e.files[e.index]
	e.index++
	reader, err := file.Open()
	return file.Name, reader, err
}

func (e *epubIterator) Close() error {
	return e.reader.Close()
}

// newEpubIterator creates an EPUB iterator with prioritized image directories
func newEpubIterator(reader *zip.ReadCloser) *epubIterator {
	// For EPUB files, prioritize images in common image directories
	imageDirs := []string{"OEBPS/Images/", "Images/", "images/", "OEBPS/images/"}

	// Separate prioritized and non-prioritized files
	var prioritized []*zip.File
	var others []*zip.File

	for _, file := range reader.File {
		isPrioritized := false
		for _, dir := range imageDirs {
			if strings.HasPrefix(file.Name, dir) {
				prioritized = append(prioritized, file)
				isPrioritized = true
				break
			}
		}
		if !isPrioritized {
			others = append(others, file)
		}
	}

	// Combine prioritized files first, then others
	allFiles := append(prioritized, others...)

	return &epubIterator{
		reader: reader,
		files:  allFiles,
		index:  0,
	}
}

// extractFirstImageFromArchive extracts the first image from any archive using the iterator
func extractFirstImageFromArchive(iter ArchiveIterator, outputFolder string) error {
	defer iter.Close()

	for {
		name, reader, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading archive: %w", err)
		}

		if isImageFile(name) {
			// Extract the file
			outputPath, err := safeJoinPath(outputFolder, name)
			if err != nil {
				reader.Close()
				return fmt.Errorf("invalid archive path: %w", err)
			}

			// Create directory if needed
			dir := filepath.Dir(outputPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				reader.Close()
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}

			dst, err := os.Create(outputPath)
			if err != nil {
				reader.Close()
				return err
			}

			_, err = io.Copy(dst, reader)
			dst.Close()
			reader.Close()
			return err
		}
		reader.Close()
	}

	return fmt.Errorf("no image files found in archive")
}

// isSafeArchivePath checks whether the provided path is safe for extraction (no directory traversal, not absolute).
func isSafeArchivePath(name string) bool {
	// Reject absolute paths
	if filepath.IsAbs(name) {
		return false
	}
	// Clean the path and ensure it does not start with ".." or contain ".." in any segment
	cleaned := filepath.Clean(name)
	if strings.Contains(cleaned, ".."+string(os.PathSeparator)) || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/..") || strings.Contains(cleaned, "\\..") {
		return false
	}
	// Optionally, disallow names starting with path separator (defense-in-depth)
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return false
	}
	return true
}

// safeJoinPath safely joins a base directory with a file path from an archive.
// It validates that the resulting path stays within the base directory to prevent directory traversal attacks.
func safeJoinPath(baseDir, archivePath string) (string, error) {
	if !isSafeArchivePath(archivePath) {
		return "", fmt.Errorf("unsafe archive path: %s", archivePath)
	}

	// Use filepath.Base to get just the filename, preventing any directory traversal
	filename := filepath.Base(archivePath)
	outputPath := filepath.Join(baseDir, filename)

	// Validate the final path is within the base directory
	cleanBase := filepath.Clean(baseDir) + string(os.PathSeparator)
	cleanOutput := filepath.Clean(outputPath)

	if !strings.HasPrefix(cleanOutput, cleanBase) && cleanOutput != filepath.Clean(baseDir) {
		return "", fmt.Errorf("path traversal attempt detected: %s", archivePath)
	}

	return outputPath, nil
}

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

	rarReader, err := rardecode.NewReader(rarFile)
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
	case ".epub":
		return extractFirstImageFromEPUB(archivePath, outputFolder)
	default:
		return fmt.Errorf("unsupported archive format: %s", ext)
	}
}

func extractFirstImageFromZip(zipPath, outputFolder string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("invalid or corrupt zip file: %w", err)
	}

	iter := &zipIterator{reader: reader, files: reader.File, index: 0}
	return extractFirstImageFromArchive(iter, outputFolder)
}

func extractFirstImageFromRar(rarPath, outputFolder string) error {
	file, err := os.Open(rarPath)
	if err != nil {
		return fmt.Errorf("failed to open rar file: %w", err)
	}

	reader, err := rardecode.NewReader(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("invalid or corrupt rar file: %w", err)
	}

	iter := &rarIterator{reader: reader, file: file}
	return extractFirstImageFromArchive(iter, outputFolder)
}

func extractFirstImageFromEPUB(epubPath, outputFolder string) error {
	reader, err := zip.OpenReader(epubPath)
	if err != nil {
		return fmt.Errorf("invalid or corrupt epub file: %w", err)
	}

	iter := newEpubIterator(reader)
	return extractFirstImageFromArchive(iter, outputFolder)
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
// ExtractAndStoreFirstImage extracts the first image from an archive and stores it with proper resizing.
// For tall images (webtoons), it crops from the top to capture the cover/title area.
// Returns the stored image URL path.
func ExtractAndStoreFirstImage(archivePath, slug, dataDir string, quality int) (string, error) {
	log.Debugf("Extracting first image from archive '%s' for manga '%s'", archivePath, slug)

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "magi-extract-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the first image
	if err := ExtractFirstImage(archivePath, tempDir); err != nil {
		// If archive is invalid or has no images, log and skip rather than failing
		if strings.Contains(err.Error(), "invalid or corrupt") ||
			strings.Contains(err.Error(), "no image files found") {
			log.Debugf("Skipping invalid or empty archive '%s' for manga '%s': %v", archivePath, slug, err)
			return "", nil
		}
		log.Errorf("Failed to extract first image from '%s' for manga '%s': %v", archivePath, slug, err)
		return "", fmt.Errorf("failed to extract first image: %w", err)
	}

	log.Debugf("Successfully extracted image from archive '%s' for manga '%s'", archivePath, slug)

	// Find the extracted image file (search recursively)
	var extractedImagePath string
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isImageFile(d.Name()) {
			extractedImagePath = path
			log.Debugf("Found extracted image '%s' for manga '%s'", d.Name(), slug)
			return fs.SkipAll // Stop after finding the first image
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk temp directory: %w", err)
	}

	if extractedImagePath == "" {
		log.Debugf("No image file found after extraction from '%s' for manga '%s'", archivePath, slug)
		return "", fmt.Errorf("no image file found after extraction")
	}

	// Process and store the image, cropping from the top for tall webtoon pages
	fileExt := filepath.Ext(extractedImagePath)[1:]
	originalFile := filepath.Join(dataDir, fmt.Sprintf("%s_original.%s", slug, fileExt))
	croppedFile := filepath.Join(dataDir, fmt.Sprintf("%s.%s", slug, fileExt))

	if err := CopyFile(extractedImagePath, originalFile); err != nil {
		return "", fmt.Errorf("failed to copy image to data directory: %w", err)
	}

	if err := ProcessImageWithTopCrop(originalFile, croppedFile, quality); err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	log.Debugf("Successfully processed and stored poster for manga '%s': %s", slug, croppedFile)
	return fmt.Sprintf("/api/posters/%s.%s?v=%s", slug, fileExt, GenerateRandomString(8)), nil
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
		if isImageFile(file.Name) && isSafeArchivePath(file.Name) {
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

	reader, err := rardecode.NewReader(file)
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

// ExtractAndStoreImageWithCrop extracts an image (from a file path or archive) and stores it with optional cropping.
// ExtractAndStoreImageWithCrop extracts an image (from a file path or archive) and stores it with optional cropping.
func ExtractAndStoreImageWithCrop(imagePath string, slug string, cropData map[string]interface{}, quality int, useWebP bool) (string, error) {
	dataDir := GetDataDirectory()
	postersDir := filepath.Join(dataDir, "posters")
	if err := os.MkdirAll(postersDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create posters directory: %w", err)
	}

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
			return processCroppedImage(imagePath, slug, postersDir, cropData, quality, useWebP)
		}
	}

	// For archive files, we'd need to extract them
	// For now, return error as the listing should give direct paths for directories
	return "", fmt.Errorf("archive image extraction not yet supported for custom posters")
}

func processCroppedImage(imagePath, slug, dataDir string, cropData map[string]interface{}, quality int, useWebP bool) (string, error) {
	// Save as WebP if enabled, otherwise JPEG
	var originalFile, croppedFile, ext string
	if useWebP {
		originalFile = filepath.Join(dataDir, fmt.Sprintf("%s_original.webp", slug)) // Save original as WebP with extended build
		croppedFile = filepath.Join(dataDir, fmt.Sprintf("%s.webp", slug))
		ext = "webp"
	} else {
		originalFile = filepath.Join(dataDir, fmt.Sprintf("%s_original.jpg", slug))
		croppedFile = filepath.Join(dataDir, fmt.Sprintf("%s.jpg", slug))
		ext = "jpg"
	}

	// Load the source image and save it in the appropriate format
	img, err := OpenImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %w", err)
	}

	// Save the original in the processing format
	if strings.HasSuffix(originalFile, ".webp") {
		// Use EncodeImageToBytes for WebP (available in extended build)
		data, encodeErr := EncodeImageToBytes(img, "webp", quality)
		if encodeErr != nil {
			return "", fmt.Errorf("failed to encode original image as WebP: %w", encodeErr)
		}
		if err := os.WriteFile(originalFile, data, 0644); err != nil {
			return "", fmt.Errorf("failed to save original image: %w", err)
		}
	} else {
		// Use saveProcessedImage for JPEG
		if err := saveProcessedImage(originalFile, img, quality); err != nil {
			return "", fmt.Errorf("failed to save original image: %w", err)
		}
	}

	// Apply cropping if provided
	if err := ProcessImageWithCrop(originalFile, croppedFile, cropData, quality); err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	return fmt.Sprintf("/api/posters/%s.%s", slug, ext), nil
}

// GetDataDirectory returns the data directory path
func GetDataDirectory() string {
	return dataDirectory
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

	reader, err := rardecode.NewReader(file)
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

// ExtractAndStoreImageWithCropByIndex extracts an image by index with cropping
func ExtractAndStoreImageWithCropByIndex(mangaPath, slug string, imageIndex int, cropData map[string]interface{}, useWebP bool, quality int) (string, error) {
	dataDir := GetDataDirectory()
	postersDir := filepath.Join(dataDir, "posters")
	if err := os.MkdirAll(postersDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create posters directory: %w", err)
	}
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

	return processCroppedImage(imagePath, slug, postersDir, cropData, quality, useWebP)
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
				// Validate and sanitize the path
				outputPath, err := safeJoinPath(outputDir, file.Name)
				if err != nil {
					return "", fmt.Errorf("invalid archive path: %w", err)
				}

				src, err := file.Open()
				if err != nil {
					return "", err
				}
				defer src.Close()

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

	reader, err := rardecode.NewReader(file)
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
				// Validate and sanitize the path
				outputPath, err := safeJoinPath(outputDir, header.Name)
				if err != nil {
					return "", fmt.Errorf("invalid archive path: %w", err)
				}

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
	// Validate and sanitize the path
	outputPath, err := safeJoinPath(outputFolder, file.Name)
	if err != nil {
		return "", fmt.Errorf("invalid archive path: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

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

// GenerateRandomString generates a random alphanumeric string of the specified length
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}
