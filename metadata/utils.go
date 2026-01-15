package metadata

import (
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2/log"
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
	// Enhanced metadata setters
	SetAuthors([]models.AuthorInfo)
	SetArtists([]models.AuthorInfo)
	SetStartDate(string)
	SetEndDate(string)
	SetChapterCount(int)
	SetVolumeCount(int)
	SetAverageScore(float64)
	SetPopularity(int)
	SetFavorites(int)
	SetDemographic(string)
	SetPublisher(string)
	SetMagazine(string)
	SetSerialization(string)
	SetGenres([]string)
	SetCharacters([]string)
	SetAlternativeTitles([]string)
	SetAttributionLinks([]models.AttributionLink)
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

	// Enhanced metadata fields - convert types
	authors := make([]models.AuthorInfo, len(meta.Authors))
	for i, author := range meta.Authors {
		authors[i] = models.AuthorInfo{Name: author.Name, Role: author.Role}
	}
	media.SetAuthors(authors)

	artists := make([]models.AuthorInfo, len(meta.Artists))
	for i, artist := range meta.Artists {
		artists[i] = models.AuthorInfo{Name: artist.Name, Role: artist.Role}
	}
	media.SetArtists(artists)

	media.SetStartDate(meta.StartDate)
	media.SetEndDate(meta.EndDate)
	media.SetChapterCount(meta.ChapterCount)
	media.SetVolumeCount(meta.VolumeCount)
	media.SetAverageScore(meta.AverageScore)
	media.SetPopularity(meta.Popularity)
	media.SetFavorites(meta.Favorites)
	media.SetDemographic(meta.Demographic)
	media.SetPublisher(meta.Publisher)
	media.SetMagazine(meta.Magazine)
	media.SetSerialization(meta.Serialization)
	media.SetGenres(meta.Genres)
	media.SetCharacters(meta.Characters)
	media.SetAlternativeTitles(meta.AlternativeTitles)
	// Note: AttributionLinks are only available in AggregatedMediaMetadata, not MediaMetadata
}

// UpdateMediaFromAggregated updates media fields from aggregated metadata
func UpdateMediaFromAggregated(media MediaUpdater, aggregatedMeta *AggregatedMediaMetadata, coverArtURL string) {
	if aggregatedMeta == nil {
		return
	}

	media.SetName(aggregatedMeta.Title)
	media.SetDescription(aggregatedMeta.Description)
	media.SetYear(aggregatedMeta.Year)
	media.SetOriginalLanguage(aggregatedMeta.OriginalLanguage)
	media.SetStatus(aggregatedMeta.Status)
	media.SetContentRating(aggregatedMeta.ContentRating)
	media.SetType(aggregatedMeta.Type)
	media.SetCoverArtURL(coverArtURL)

	// Enhanced metadata fields - convert types
	authors := make([]models.AuthorInfo, len(aggregatedMeta.Authors))
	for i, author := range aggregatedMeta.Authors {
		authors[i] = models.AuthorInfo{Name: author.Name, Role: author.Role}
	}
	media.SetAuthors(authors)

	artists := make([]models.AuthorInfo, len(aggregatedMeta.Artists))
	for i, artist := range aggregatedMeta.Artists {
		artists[i] = models.AuthorInfo{Name: artist.Name, Role: artist.Role}
	}
	media.SetArtists(artists)

	media.SetStartDate(aggregatedMeta.StartDate)
	media.SetEndDate(aggregatedMeta.EndDate)
	media.SetChapterCount(aggregatedMeta.ChapterCount)
	media.SetVolumeCount(aggregatedMeta.VolumeCount)
	media.SetAverageScore(aggregatedMeta.AverageScore)
	media.SetPopularity(aggregatedMeta.Popularity)
	media.SetFavorites(aggregatedMeta.Favorites)
	media.SetDemographic(aggregatedMeta.Demographic)
	media.SetPublisher(aggregatedMeta.Publisher)
	media.SetMagazine(aggregatedMeta.Magazine)
	media.SetSerialization(aggregatedMeta.Serialization)
	media.SetGenres(aggregatedMeta.Genres)
	media.SetCharacters(aggregatedMeta.Characters)
	media.SetAlternativeTitles(aggregatedMeta.AlternativeTitles)

	attributionLinks := make([]models.AttributionLink, len(aggregatedMeta.AttributionLinks))
	for i, link := range aggregatedMeta.AttributionLinks {
		attributionLinks[i] = models.AttributionLink{Provider: link.Provider, URL: link.URL, Title: link.Title}
	}
	log.Debugf("UpdateMediaFromAggregated: setting %d attribution links for media", len(attributionLinks))
	for _, link := range attributionLinks {
		log.Debugf("  - Provider: %s, URL: %s, Title: %s", link.Provider, link.URL, link.Title)
	}
	media.SetAttributionLinks(attributionLinks)
}
