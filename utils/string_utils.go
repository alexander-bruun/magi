package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gofiber/fiber/v2/log"
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
