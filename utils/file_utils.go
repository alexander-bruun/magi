package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
)

// CountImageFiles counts the number of image files in an archive (zip, cbz, rar, or cbr).
func CountImageFiles(archiveFilePath string) (int, error) {
	lowerPath := strings.ToLower(archiveFilePath)
	if strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".cbz") {
		return countImageFilesInZip(archiveFilePath)
	} else if strings.HasSuffix(lowerPath, ".rar") || strings.HasSuffix(lowerPath, ".cbr") {
		return countImageFilesInRar(archiveFilePath)
	} else {
		return 0, fmt.Errorf("unsupported file type")
	}
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
		if strings.Contains(fileName, "..") {
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

	outputPath := filepath.Join(outputFolder, filepath.Base(file.Name))
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
func isImageFile(fileName string) bool {
	ext := strings.ToLower(fileName[strings.LastIndex(fileName, ".")+1:])
	switch ext {
	case "jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp":
		return true
	default:
		return false
	}
}
