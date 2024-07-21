package utils

import (
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
