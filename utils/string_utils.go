package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gofiber/fiber/v2/log"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// RemovePatterns applies custom parsing to clean up the path string.
func RemovePatterns(path string) string {
	path = removeParenthesesContent(path)
	path = removeBracketsContent(path)
	path = removeBracesContent(path)
	path = handleSpecialCases(path)
	path = processComplexPatterns(path)
	path = removeTrailingSuffixes(path)
	path = strings.Join(strings.Fields(path), " ")
	path = strings.TrimSpace(path)
	path = removeNonASCII(path)
	if strings.HasSuffix(path, ", The") {
		path = "The " + strings.TrimSuffix(path, ", The")
	}
	return path
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
	patterns := []struct {
		pattern string
	}{
		{`v\d+\s*-\s*v\d+`},                // Removing patterns like 'v1 - v2' or 'v12 - v34'
		{`c\d+\s*-\s*c\d+`},                // Removing patterns like 'c1 - c2' or 'c10 - c20'
		{`v\d+\s*-\s*\d+`},                 // Removing patterns like 'v1 - 2' or 'v12 - 34'
		{`c\d+\s*-\s*\d+`},                 // Removing patterns like 'c1 - 2' or 'c10 - 34'
		{`\b\d{1,2}-\d{1,2}\b`},            // Removing patterns like '12-34' or '1-9'
		{`\b\d{3,}-\d{3,}\b`},              // Removing patterns like '000-305' or '123-456'
		{`Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`}, // Removing patterns like 'Vol. 1 + Vol. 2'
		{`\sS\d+\b`},                       // Removing patterns like ' S1' or ' S12'
		{`\bVolumes?\d+-\d+\+\w+\b`},       // Removing patterns like 'Volume1-2+ABC'
	}

	for _, pat := range patterns {
		re, err := regexp.Compile(pat.pattern)
		if err != nil {
			log.Errorf("Failed to compile pattern %s: %v", pat.pattern, err)
			continue
		}
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
	var numStr string
	found := false

	for _, r := range name {
		if unicode.IsDigit(r) {
			numStr += string(r)
			found = true
		} else if found {
			break
		}
	}

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
