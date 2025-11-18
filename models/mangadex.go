package models

import (
	"strings"
)

// DetermineMangaTypeByLanguage returns a suggested type (manga/manhwa/manhua/etc.)
// based on the original language code.
func DetermineMangaTypeByLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ja", "jp":
		return "manga"
	case "ko":
		return "manhwa"
	case "zh", "cn", "zh-cn", "zh-hk", "zh-tw":
		return "manhua"
	case "fr":
		return "manfra"
	case "en":
		return "oel"
	default:
		return "manga"
	}
}
