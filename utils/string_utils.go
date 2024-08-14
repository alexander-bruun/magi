package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
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
		{`v\d+\s*-\s*v\d+`},
		{`c\d+\s*-\s*c\d+`},
		{`v\d+\s*-\s*\d+`},
		{`c\d+\s*-\s*\d+`},
		{`\b\d{1,2}-\d{1,2}\b`},
		{`Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`},
		{`\sS\d+\b`},
		{`\bVolumes?\d+-\d+\+\w+\b`},
	}
	for _, pat := range patterns {
		path = removePattern(path, pat.pattern)
	}
	return path
}

func removePattern(path, pattern string) string {
	switch pattern {
	case `v\d+\s*-\s*v\d+`, `c\d+\s*-\s*c\d+`, `v\d+\s*-\s*\d+`, `c\d+\s*-\s*\d+`:
		return removePatternByPrefix(path, rune(pattern[0]), '-')
	case `\b\d{1,2}-\d{1,2}\b`:
		return removeDashRange(path)
	case `Vol\.\s*\d+\s*\+\s*Vol\.\s*\d+`:
		return removeVolumeRange(path)
	case `\sS\d+\b`:
		return removeSeasonNumbers(path)
	case `\bVolumes?\d+-\d+\+\w+\b`:
		return removeVolumesPattern(path)
	}
	return path
}

func removePatternByPrefix(path string, prefix rune, separator rune) string {
	var builder strings.Builder
	inPrefix := false
	inSeparator := false

	for _, r := range path {
		if r == prefix {
			inPrefix = true
		} else if inPrefix && r == separator {
			inSeparator = true
		} else if inPrefix && inSeparator && !unicode.IsDigit(r) {
			inPrefix = false
			inSeparator = false
		} else {
			if !inPrefix {
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}

func removeDashRange(path string) string {
	var builder strings.Builder
	inRange := false

	for i := 0; i < len(path); i++ {
		if i < len(path)-3 && unicode.IsDigit(rune(path[i])) && path[i+2] == '-' && unicode.IsDigit(rune(path[i+3])) {
			inRange = true
			for i < len(path) && (path[i] == ' ' || path[i] == '-' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		if !inRange {
			builder.WriteByte(path[i])
		}
	}

	return builder.String()
}

func removeVolumeRange(path string) string {
	var builder strings.Builder
	inRange := false

	for i := 0; i < len(path); i++ {
		if i < len(path)-7 && strings.HasPrefix(path[i:], "Vol. ") && unicode.IsDigit(rune(path[i+5])) && path[i+6] == ' ' && path[i+7] == '+' && path[i+8] == ' ' && strings.HasPrefix(path[i+9:], "Vol. ") && unicode.IsDigit(rune(path[i+14])) {
			inRange = true
			for i < len(path) && (path[i] == ' ' || path[i] == '+' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		if !inRange {
			builder.WriteByte(path[i])
		}
	}

	return builder.String()
}

func removeSeasonNumbers(path string) string {
	var builder strings.Builder
	inSeason := false

	for i := 0; i < len(path); i++ {
		if i < len(path)-2 && path[i] == 'S' && unicode.IsDigit(rune(path[i+1])) {
			inSeason = true
			for i < len(path) && (path[i] == ' ' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		if !inSeason {
			builder.WriteByte(path[i])
		}
	}

	return builder.String()
}

func removeVolumesPattern(path string) string {
	var builder strings.Builder
	inPattern := false

	for i := 0; i < len(path); i++ {
		if i < len(path)-5 && (strings.HasPrefix(path[i:], "Volume") || strings.HasPrefix(path[i:], "Volumes")) && unicode.IsDigit(rune(path[i+6])) && path[i+7] == '-' && unicode.IsDigit(rune(path[i+8])) && path[i+9] == '+' {
			inPattern = true
			for i < len(path) && (path[i] == ' ' || path[i] == '+' || path[i] == '\n') {
				i++
			}
			i--
			continue
		}
		if !inPattern {
			builder.WriteByte(path[i])
		}
	}

	return builder.String()
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
