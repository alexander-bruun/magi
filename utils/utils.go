package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func DownloadFile(downloadDir, fileName, fileUrl string) error {
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

	// Construct the full file path
	filePath := filepath.Join(downloadDir, fileName)

	// Create the file
	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Get the data
	resp, err := http.Get(fileUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// BigramSearch performs a bigram search algorithm
func BigramSearch(keyword string, items []string) []string {
	var results []string
	for _, item := range items {
		score := CompareStrings(keyword, item)
		// For simplicity, consider a threshold of 0.3 for similarity
		if score > 0.3 {
			results = append(results, item)
		}
	}
	return results
}

// compareStrings compares two strings using a bigram-based similarity algorithm
func CompareStrings(str1, str2 string) float64 {
	if str1 == str2 {
		return 1
	}

	len1 := len(str1)
	len2 := len(str2)
	if len1 < 2 || len2 < 2 {
		return 0
	}

	bigramCounts := make(map[string]int)
	commonBigramsCount := 0
	totalBigrams := 0

	// Process the first string
	for i := 0; i < len1-1; i++ {
		bigram := str1[i : i+2]
		bigramCounts[bigram]++
	}

	// Process the second string and calculate common bigrams
	for i := 0; i < len2-1; i++ {
		bigram := str2[i : i+2]
		if bigramCounts[bigram] > 0 {
			commonBigramsCount++
			bigramCounts[bigram]--
		}
		totalBigrams++
	}

	// Include remaining bigrams from the first string
	totalBigrams += len1 - 1

	return (2.0 * float64(commonBigramsCount)) / float64(totalBigrams)
}
