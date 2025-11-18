package metadata

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

// ApplyMetadataToManga is a helper interface for updating manga models
// This allows the metadata package to provide update functionality without importing models
type MangaUpdater interface {
	SetName(string)
	SetDescription(string)
	SetYear(int)
	SetOriginalLanguage(string)
	SetStatus(string)
	SetContentRating(string)
	SetType(string)
	SetCoverArtURL(string)
}

// UpdateManga updates manga fields from metadata
func UpdateManga(manga MangaUpdater, meta *MangaMetadata, coverArtURL string) {
	if meta == nil {
		return
	}

	manga.SetName(meta.Title)
	manga.SetDescription(meta.Description)
	manga.SetYear(meta.Year)
	manga.SetOriginalLanguage(meta.OriginalLanguage)
	manga.SetStatus(meta.Status)
	manga.SetContentRating(meta.ContentRating)
	manga.SetType(meta.Type)
	manga.SetCoverArtURL(coverArtURL)
}
