package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/gofiber/fiber/v2/log"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Pre-compiled regexes for performance
var complexPatterns = []*regexp.Regexp{
	regexp.MustCompile(`v\d+\s*-\s*v\d+`),                // Removing patterns like 'v1 - v2' or 'v12 - v34'
	regexp.MustCompile(`c\d+\s*-\s*c\d+`),                // Removing patterns like 'c1 - c2' or 'c10 - c20'
	regexp.MustCompile(`v\d+\s*-\s*\d+`),                 // Removing patterns like 'v1 - 2' or 'v12 - 34'
	regexp.MustCompile(`c\d+\s*-\s*\d+`),                 // Removing patterns like 'c1 - 2' or 'c10 - 34'
	regexp.MustCompile(`\b\d{1,2}-\d{1,2}\b`),            // Removing patterns like '12-34' or '1-9'
	regexp.MustCompile(`\b\d{3,}-\d{3,}\b`),              // Removing patterns like '000-305' or '123-456'
	regexp.MustCompile(`Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`), // Removing patterns like 'Vol. 1 + Vol. 2'
	regexp.MustCompile(`\sS\d+\b`),                       // Removing patterns like ' S1' or ' S12'
	regexp.MustCompile(`\bVolumes?\d+-\d+\+\w+\b`),       // Removing patterns like 'Volume1-2+ABC'
}

// Simple cache for RemovePatterns results
var removePatternsCache = make(map[string]string)
var cacheMutex sync.RWMutex
const maxCacheSize = 10000

// RemovePatterns applies custom parsing to clean up the path string.
func RemovePatterns(path string) string {
	// Check cache first (read lock)
	cacheMutex.RLock()
	if cached, exists := removePatternsCache[path]; exists {
		cacheMutex.RUnlock()
		return cached
	}
	cacheMutex.RUnlock()

	// Process the path
	processed := path
	processed = removeParenthesesContent(processed)
	processed = removeBracketsContent(processed)
	processed = removeBracesContent(processed)
	processed = handleSpecialCases(processed)
	processed = processComplexPatterns(processed)
	processed = removeTrailingSuffixes(processed)
	processed = strings.Join(strings.Fields(processed), " ")
	processed = strings.TrimSpace(processed)
	processed = removeNonASCII(processed)
	if strings.HasSuffix(processed, ", The") {
		processed = "The " + strings.TrimSuffix(processed, ", The")
	}

	// Cache the result (write lock)
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	// Double-check the cache in case another goroutine added it
	if cached, exists := removePatternsCache[path]; exists {
		return cached
	}
	
	// Cache the result (simple eviction by clearing when full)
	if len(removePatternsCache) >= maxCacheSize {
		removePatternsCache = make(map[string]string)
	}
	removePatternsCache[path] = processed

	return processed
}

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
	return removeContentBetween(path, "(", ")")
}

func removeBracketsContent(path string) string {
	return removeContentBetween(path, "[", "]")
}

func removeBracesContent(path string) string {
	return removeContentBetween(path, "{", "}")
}

func removeContentBetween(path, open, close string) string {
	for i := strings.Index(path, open); i != -1; i = strings.Index(path, open) {
		end := strings.Index(path[i:], close)
		if end == -1 {
			break
		}
		path = path[:i] + path[i+end+len(close):]
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
	for _, re := range complexPatterns {
		path = re.ReplaceAllString(path, "")
	}
	return strings.TrimSpace(path)
}

// Sluggify transforms a string into a URL-friendly slug.
func Sluggify(s string) string {
	s = strings.ToLower(s)
	var builder strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else if r == '.' || r == ',' {
			builder.WriteRune('-')
		} else if unicode.IsSpace(r) {
			builder.WriteRune('-')
		}
	}

	result := builder.String()
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

	return strings.Trim(builder.String(), "-")
}

// ExtractNumber extracts the first number found in the given string.
func ExtractNumber(name string) (int, error) {
	var builder strings.Builder
	found := false

	for _, r := range name {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
			found = true
		} else if found {
			break
		}
	}

	numStr := builder.String()
	if numStr == "" {
		return 0, fmt.Errorf("no number found in string")
	}

	return strconv.Atoi(numStr)
}

// LevenshteinDistance calculates the Levenshtein distance between two strings.
// This is the minimum number of single-character edits (insertions, deletions, or substitutions)
// required to change one word into the other.
func LevenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create a 2D matrix to store distances
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first column and row
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill in the rest of the matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// SimilarityRatio calculates a similarity ratio between two strings (0.0 to 1.0).
// 1.0 means identical, 0.0 means completely different.
func SimilarityRatio(s1, s2 string) float64 {
	// Normalize to lowercase for comparison
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)
	
	maxLen := len(s1Lower)
	if len(s2Lower) > maxLen {
		maxLen = len(s2Lower)
	}
	
	if maxLen == 0 {
		return 1.0
	}
	
	distance := LevenshteinDistance(s1Lower, s2Lower)
	return 1.0 - float64(distance)/float64(maxLen)
}

// ExtractChapterName attempts to extract a volume or chapter name from a filename.
// If no volume/chapter pattern is found, returns the cleaned filename.
func ExtractChapterName(filename string) string {
	// Look for volume patterns (v01, vol.1, volume 1, etc.)
	if vol := regexp.MustCompile(`(?i)(?:v(?:ol(?:ume)?)?)\.?\s*(\d+)`).FindStringSubmatch(filename); vol != nil {
		num, _ := strconv.Atoi(vol[1])
		return fmt.Sprintf("Volume %d", num)
	}
	// Look for chapter patterns (c01, ch.1, chapter 1, etc.)
	if ch := regexp.MustCompile(`(?i)c(h)?\.?\s*(\d+)`).FindStringSubmatch(filename); ch != nil {
		num, _ := strconv.Atoi(ch[2])
		return fmt.Sprintf("Chapter %d", num)
	}
	// Otherwise, return the cleaned filename
	cleaned := RemovePatterns(strings.TrimSuffix(filename, filepath.Ext(filename)))
	// If the cleaned name is just digits, assume it's a chapter number
	if regexp.MustCompile(`^\d+$`).MatchString(cleaned) {
		if num, err := strconv.Atoi(cleaned); err == nil {
			return fmt.Sprintf("Chapter %d", num)
		}
	}
	return cleaned
}

// MarkdownToHTML converts markdown text to safe HTML using goldmark
func MarkdownToHTML(markdown string) template.HTML {
	if markdown == "" {
		return template.HTML("")
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,        // GitHub Flavored Markdown
			extension.Linkify,    // Auto-link URLs
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Add IDs to headings
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),   // Convert newlines to <br>
			html.WithXHTML(),       // Use XHTML-style tags
			html.WithUnsafe(),      // Allow raw HTML (be careful with user input!)
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		log.Errorf("Failed to convert markdown to HTML: %v", err)
		// Return plain text wrapped in <p> tag as fallback
		return template.HTML("<p>" + template.HTMLEscapeString(markdown) + "</p>")
	}

	return template.HTML(buf.String())
}
