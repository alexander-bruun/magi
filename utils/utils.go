package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nwaples/rardecode"
)

func RemovePatterns(path string) string {
	patterns := []string{
		`\([^)]*\)`,                      // Matches anything inside parentheses
		`\[[^\]]*\]`,                     // Matches anything inside square brackets
		`\{[^}]*\}`,                      // Matches anything inside curly brackets
		`(?i)^manga\s*`,                  // Matches "manga" at the beginning of the string, case-insensitively
		`(?i)\smanga$`,                   // Matches "manga" at the end of the string, case-insensitively
		` - archived$`,                   // Matches exact phrase " - archived" at the end of the string
		`v\d+\s*-\s*v\d+`,                // Matches vNUMBER - vNUMBER
		`c\d+\s*-\s*c\d+`,                // Matches cNUMBER - cNUMBER
		`v\d+\s*-\s*\d+`,                 // Matches vNUMBER - NUMBER
		`c\d+\s*-\s*\d+`,                 // Matches cNUMBER - NUMBER
		` -\s*$`,                         // Matches trailing " -" with optional whitespace at the end of the string
		`\b\d{1,2}-\d{1,2}\b`,            // Matches patterns like "1-3", "10-20", etc.
		`Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`, // Matches "Vol. 1 + Vol. 2"
		`\sS\d+\b`,                       // Matches season numbers like " S1", " S2", etc., with preceding whitespace and word boundary
		`\bVolumes?\d+-\d+\+\w+\b`,       // Matches patterns like "Volumes1-2+Bonus", where \w+ matches one or more word characters
		` RAR$`,                          // Matches exact phrase " RAR" at the end of the string
		` ZIP$`,                          // Matches exact phrase " ZIP" at the end of the string
		` rar$`,                          // Matches exact phrase " rar" at the end of the string
		` zip$`,                          // Matches exact phrase " zip" at the end of the string
		` \+Plus$`,                       // Matches exact phrase " +Plus" at the end of the string
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		path = re.ReplaceAllString(path, "")
	}

	// Remove multiple spaces
	reSpaces := regexp.MustCompile(`\s+`)
	path = reSpaces.ReplaceAllString(path, " ")

	path = strings.TrimSpace(path) // Trim leading and trailing whitespace

	// Check if the path ends with ", The"
	if strings.HasSuffix(path, ", The") {
		// Remove ", The" from the end
		path = strings.TrimSuffix(path, ", The")
		// Prepend "The " to the beginning
		path = "The " + path
	}

	return path
}

func Sluggify(s string) string {
	// Convert the string to lowercase
	s = strings.ToLower(s)

	// Replace periods and commas with a dash
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, ",", "-")

	// Remove all non-alphanumeric characters except hyphens and spaces
	re := regexp.MustCompile(`[^a-z0-9\s-]`)
	s = re.ReplaceAllString(s, "")

	// Replace spaces and multiple hyphens with a single hyphen
	re = regexp.MustCompile(`[\s-]+`)
	s = re.ReplaceAllString(s, "-")

	// Trim any leading or trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

func ExtractNumber(name string) (int, error) {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(name)
	if match == "" {
		return 0, fmt.Errorf("no number found in string")
	}
	return strconv.Atoi(match)
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
	// Open the zip file.
	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return 0, err
	}
	defer zipFile.Close()

	imageCount := 0

	// Iterate through each file in the zip archive.
	for _, file := range zipFile.File {
		if isImageFile(file.Name) {
			imageCount++
		}
	}

	return imageCount, nil
}

// countImageFilesInRar counts the number of image files in a rar archive.
func countImageFilesInRar(rarFilePath string) (int, error) {
	// Open the rar file.
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

	// Iterate through each file in the rar archive.
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
		if isImageFile(file.Name) {
			return extractZipFile(file, outputFolder)
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
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy the content
	_, err = io.Copy(destinationFile, sourceFile)
	return err
}
