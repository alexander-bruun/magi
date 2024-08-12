package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2/log"
	"github.com/nwaples/rardecode"
)

// Regex pattern removal times took on average ~250µs, the pure go pattern removal takes ~15µs.

// RemovePatterns applies custom parsing to clean up the path string.
func RemovePatterns(path string) string {
	// Apply custom parsing functions
	path = removeParenthesesContent(path)
	path = removeBracketsContent(path)
	path = removeBracesContent(path)
	path = handleSpecialCases(path)
	path = processComplexPatterns(path)

	// Remove trailing specific suffixes
	path = removeTrailingSuffixes(path)

	// Remove multiple spaces
	path = strings.Join(strings.Fields(path), " ")

	// Trim leading and trailing whitespace
	path = strings.TrimSpace(path)

	// Remove non-ASCII characters
	path = removeNonASCII(path)

	// Check if the path ends with ", The" and modify if necessary
	if strings.HasSuffix(path, ", The") {
		path = "The " + strings.TrimSuffix(path, ", The")
	}

	return path
}

// removeNonASCII removes non-ASCII characters from the string.
func removeNonASCII(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if r <= unicode.MaxASCII {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func removeParenthesesContent(path string) string {
	for i := strings.Index(path, "("); i != -1; i = strings.Index(path, "(") {
		end := strings.Index(path[i:], ")")
		if end == -1 {
			break
		}
		path = path[:i] + path[i+end+1:]
	}
	return path
}

func removeBracketsContent(path string) string {
	for i := strings.Index(path, "["); i != -1; i = strings.Index(path, "[") {
		end := strings.Index(path[i:], "]")
		if end == -1 {
			break
		}
		path = path[:i] + path[i+end+1:]
	}
	return path
}

func removeBracesContent(path string) string {
	for i := strings.Index(path, "{"); i != -1; i = strings.Index(path, "{") {
		end := strings.Index(path[i:], "}")
		if end == -1 {
			break
		}
		path = path[:i] + path[i+end+1:]
	}
	return path
}

func removeTrailingSuffixes(path string) string {
	suffixes := []string{" - archived", " RAR", " ZIP", " rar", " zip", " +Plus"}
	for _, suffix := range suffixes {
		path = strings.TrimSuffix(path, suffix)
	}
	return path
}

func handleSpecialCases(path string) string {
	if strings.Contains(strings.ToLower(path), "manga") {
		path = strings.ReplaceAll(strings.ToLower(path), "manga", "")
	}
	return path
}

func processComplexPatterns(path string) string {
	patterns := []struct {
		pattern string
	}{
		{`v\d+\s*-\s*v\d+`},                // Matches vNUMBER - vNUMBER
		{`c\d+\s*-\s*c\d+`},                // Matches cNUMBER - cNUMBER
		{`v\d+\s*-\s*\d+`},                 // Matches vNUMBER - NUMBER
		{`c\d+\s*-\s*\d+`},                 // Matches cNUMBER - NUMBER
		{`\b\d{1,2}-\d{1,2}\b`},            // Matches patterns like "1-3", "10-20", etc.
		{`Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`}, // Matches "Vol. 1 + Vol. 2"
		{`\sS\d+\b`},                       // Matches season numbers like " S1", " S2", etc.
		{`\bVolumes?\d+-\d+\+\w+\b`},       // Matches patterns like "Volumes1-2+Bonus"
	}

	for _, pat := range patterns {
		path = removePattern(path, pat.pattern)
	}
	return path
}

func removePattern(path, pattern string) string {
	switch pattern {
	case `v\d+\s*-\s*v\d+`:
		path = removePatternByPrefix(path, 'v', '-')
	case `c\d+\s*-\s*c\d+`:
		path = removePatternByPrefix(path, 'c', '-')
	case `v\d+\s*-\s*\d+`:
		path = removePatternByPrefix(path, 'v', '-')
	case `c\d+\s*-\s*\d+`:
		path = removePatternByPrefix(path, 'c', '-')
	case `\b\d{1,2}-\d{1,2}\b`:
		path = removeDashRange(path)
	case `Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`:
		path = removeVolumeRange(path)
	case `\sS\d+\b`:
		path = removeSeasonNumbers(path)
	case `\bVolumes?\d+-\d+\+\w+\b`:
		path = removeVolumesPattern(path)
	}
	return path
}

// Custom functions to handle specific patterns
func removePatternByPrefix(path string, prefix rune, separator rune) string {
	var builder strings.Builder
	for i := 0; i < len(path); i++ {
		if i < len(path)-1 && path[i] == byte(prefix) && isDigit(path[i+1]) {
			j := i + 1
			for j < len(path) && isDigit(path[j]) {
				j++
			}
			if j < len(path) && path[j] == byte(separator) {
				j++
				for j < len(path) && isDigit(path[j]) {
					j++
				}
				if j < len(path) && (path[j] == ' ' || path[j] == '-' || path[j] == '\n') {
					// Skip the entire matched part
					for j < len(path) && (path[j] == ' ' || path[j] == '-' || path[j] == '\n') {
						j++
					}
					i = j - 1
					continue
				}
			}
		}
		builder.WriteByte(path[i])
	}
	return builder.String()
}

func removeDashRange(path string) string {
	var builder strings.Builder
	for i := 0; i < len(path); i++ {
		if i < len(path)-3 && isDigit(path[i]) && path[i+2] == '-' && isDigit(path[i+3]) {
			// Skip the pattern
			for i < len(path) && (path[i] == ' ' || path[i] == '-' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		builder.WriteByte(path[i])
	}
	return builder.String()
}

func removeVolumeRange(path string) string {
	var builder strings.Builder
	for i := 0; i < len(path); i++ {
		if i < len(path)-7 && strings.HasPrefix(path[i:], "Vol. ") && isDigit(path[i+5]) && path[i+6] == ' ' && path[i+7] == '+' && path[i+8] == ' ' && strings.HasPrefix(path[i+9:], "Vol. ") && isDigit(path[i+14]) {
			// Skip the pattern
			for i < len(path) && (path[i] == ' ' || path[i] == '+' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		builder.WriteByte(path[i])
	}
	return builder.String()
}

func removeSeasonNumbers(path string) string {
	var builder strings.Builder
	for i := 0; i < len(path); i++ {
		if i < len(path)-2 && path[i] == 'S' && isDigit(path[i+1]) {
			j := i + 1
			for j < len(path) && isDigit(path[j]) {
				j++
			}
			if j < len(path) && (path[j] == ' ' || path[j] == '\n') {
				// Skip the pattern
				for j < len(path) && (path[j] == ' ' || path[j] == '\n') {
					j++
				}
				i = j - 1
				continue
			}
		}
		builder.WriteByte(path[i])
	}
	return builder.String()
}

func removeVolumesPattern(path string) string {
	var builder strings.Builder
	for i := 0; i < len(path); i++ {
		if i < len(path)-5 && (strings.HasPrefix(path[i:], "Volume") || strings.HasPrefix(path[i:], "Volumes")) && isDigit(path[i+6]) && path[i+7] == '-' && isDigit(path[i+8]) && path[i+9] == '+' {
			// Skip the pattern
			for i < len(path) && (path[i] == ' ' || path[i] == '+' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		builder.WriteByte(path[i])
	}
	return builder.String()
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// Regex sluggify times took on average ~35µs, the pure go sluggifying takes ~3µs.

// Sluggify transforms a string into a URL-friendly slug.
func Sluggify(s string) string {
	// Convert the string to lowercase
	s = strings.ToLower(s)

	// Create a buffer to build the slug
	var builder strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Append alphanumeric characters directly
			builder.WriteRune(r)
		} else if r == '.' || r == ',' {
			// Replace periods and commas with a dash
			builder.WriteRune('-')
		} else if unicode.IsSpace(r) {
			// Replace spaces with a dash
			builder.WriteRune('-')
		}
	}

	// Convert the builder to a string
	result := builder.String()

	// Replace multiple hyphens with a single hyphen and trim leading/trailing hyphens
	result = collapseHyphens(result)

	return result
}

// collapseHyphens reduces multiple hyphens to a single hyphen and trims leading/trailing hyphens.
func collapseHyphens(s string) string {
	var builder strings.Builder
	inHyphen := false

	for _, r := range s {
		if r == '-' {
			if !inHyphen {
				builder.WriteRune(r)
				inHyphen = true
			}
		} else {
			builder.WriteRune(r)
			inHyphen = false
		}
	}

	// Convert the builder to a string
	result := builder.String()

	// Trim leading and trailing hyphens
	return strings.Trim(result, "-")
}

// ExtractNumber extracts the first number found in the given string.
func ExtractNumber(name string) (int, error) {
	var numStr string
	found := false

	for _, r := range name {
		if unicode.IsDigit(r) {
			numStr += string(r)
			found = true
		} else if found {
			// Stop if we have already started and encounter a non-digit character
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("no number found in string")
	}

	return strconv.Atoi(numStr)
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

// LogDuration logs the duration of a function execution
func LogDuration(functionName string, start time.Time, args ...interface{}) {
	duration := time.Since(start)
	if len(args) > 0 {
		log.Debugf("%s took %v with args %v\n", functionName, duration, args)
	} else {
		log.Debugf("%s took %v\n", functionName, duration)
	}
}
