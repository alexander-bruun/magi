package metadata

import (
	"strings"
)

// DetermineMediaTypeByLanguage returns a suggested type (media/manhwa/manhua/etc.)
// based on the original language code.
func DetermineMediaTypeByLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ja", "jp":
		return "media"
	case "ko":
		return "manhwa"
	case "zh", "cn", "zh-cn", "zh-hk", "zh-tw":
		return "manhua"
	case "fr":
		return "manfra"
	case "en":
		return "oel"
	default:
		return "media"
	}
}

// ApplyMetadataToMedia is a helper interface for updating media models
// This allows the metadata package to provide update functionality without importing models
type MediaUpdater interface {
	SetName(string)
	SetDescription(string)
	SetYear(int)
	SetOriginalLanguage(string)
	SetStatus(string)
	SetContentRating(string)
	SetType(string)
	SetCoverArtURL(string)
}

// UpdateMedia updates media fields from metadata
func UpdateMedia(media MediaUpdater, meta *MediaMetadata, coverArtURL string) {
	if meta == nil {
		return
	}

	media.SetName(meta.Title)
	media.SetDescription(meta.Description)
	media.SetYear(meta.Year)
	media.SetOriginalLanguage(meta.OriginalLanguage)
	media.SetStatus(meta.Status)
	media.SetContentRating(meta.ContentRating)
	media.SetType(meta.Type)
	media.SetCoverArtURL(coverArtURL)
}
