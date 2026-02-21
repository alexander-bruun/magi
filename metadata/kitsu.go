package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils/text"
)

const kitsuBaseURL = "https://kitsu.io/api/edge"

// KitsuProvider implements the Provider interface for Kitsu API
type KitsuProvider struct {
	BaseProvider
}

// NewKitsuProvider creates a new Kitsu metadata provider
func NewKitsuProvider(apiToken string) Provider {
	return &KitsuProvider{
		BaseProvider: BaseProvider{
			ProviderName: "kitsu",
			APIToken:     apiToken,
			Client:       &http.Client{},
			BaseURL:      kitsuBaseURL,
		},
	}
}

func init() {
	RegisterProvider("kitsu", NewKitsuProvider)
}

func (k *KitsuProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	return DefaultFindBestMatch(k, title)
}

func (k *KitsuProvider) Search(title string) ([]SearchResult, error) {
	// Search both anime and manga
	animeResults, err := k.searchMediaType(title, "anime")
	if err != nil {
		return nil, err
	}

	mangaResults, err := k.searchMediaType(title, "manga")
	if err != nil {
		return nil, err
	}

	// Combine results
	allResults := append(animeResults, mangaResults...)

	// Sort by similarity score (highest first)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].SimilarityScore > allResults[j].SimilarityScore
	})

	return allResults, nil
}

// searchMediaType searches for a specific media type (anime or manga)
func (k *KitsuProvider) searchMediaType(title, mediaType string) ([]SearchResult, error) {
	titleEncoded := url.QueryEscape(title)
	searchURL := fmt.Sprintf("%s/%s?filter[text]=%s&page[limit]=20", k.BaseURL, mediaType, titleEncoded)

	resp, err := k.Client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Kitsu %s: %w", mediaType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the response body to get error details
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])
		return nil, fmt.Errorf("Kitsu API returned status: %s, body: %s", resp.Status, bodyStr)
	}

	var response struct {
		Data []kitsuMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Kitsu response: %w", err)
	}

	results := make([]SearchResult, 0, len(response.Data))
	titleLower := strings.ToLower(title)

	for _, media := range response.Data {

		mangaTitle := extractKitsuTitle(media.Attributes.Titles)
		if mangaTitle == "" {
			continue
		}

		coverURL := extractKitsuCoverURL(media.Attributes.PosterImage)
		description := media.Attributes.Synopsis
		if description == "" {
			description = media.Attributes.Description
		}

		// Extract tags from categories (Kitsu uses categories instead of tags)
		var tags []string
		// Note: Categories would require additional API calls, so we'll skip for now

		year := 0
		if media.Attributes.StartDate != "" {
			if startTime, err := time.Parse("2006-01-02", media.Attributes.StartDate); err == nil {
				year = startTime.Year()
			}
		}

		results = append(results, SearchResult{
			ID:              media.ID,
			Title:           mangaTitle,
			Description:     description,
			CoverArtURL:     coverURL,
			Year:            year,
			SimilarityScore: text.CompareStrings(titleLower, strings.ToLower(mangaTitle)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (k *KitsuProvider) GetMetadata(id string) (*MediaMetadata, error) {
	// Try anime first
	meta, err := k.getMetadataForType(id, "anime")
	if err == nil {
		return meta, nil
	}

	// If anime fails, try manga
	return k.getMetadataForType(id, "manga")
}

func (k *KitsuProvider) getMetadataForType(id, mediaType string) (*MediaMetadata, error) {
	fetchURL := fmt.Sprintf("%s/%s/%s", k.BaseURL, mediaType, id)

	resp, err := k.Client.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Kitsu metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kitsu API returned status: %s", resp.Status)
	}

	var response struct {
		Data kitsuMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Kitsu response: %w", err)
	}

	return k.convertToMediaMetadata(&response.Data), nil
}

func (k *KitsuProvider) convertToMediaMetadata(detail *kitsuMediaDetail) *MediaMetadata {
	metadata := &MediaMetadata{
		Title:            extractKitsuTitle(detail.Attributes.Titles),
		Description:      detail.Attributes.Synopsis,
		Year:             0,
		OriginalLanguage: "", // Kitsu doesn't provide this directly
		Status:           detail.Attributes.Status,
		ContentRating:    mapKitsuAgeRating(detail.Attributes.AgeRating, detail.Attributes.AgeRatingGuide),
		CoverArtURL:      extractKitsuCoverURL(detail.Attributes.PosterImage),
		ExternalID:       fmt.Sprintf("%s:%s", detail.Type, detail.ID),
		Type:             "manga", // Kitsu is primarily for manga/anime
	}

	if metadata.Description == "" {
		metadata.Description = detail.Attributes.Description
	}

	// Extract year from start date
	if detail.Attributes.StartDate != "" {
		if startTime, err := time.Parse("2006-01-02", detail.Attributes.StartDate); err == nil {
			metadata.Year = startTime.Year()
		}
	}

	// Extract alternative titles
	for _, title := range detail.Attributes.Titles {
		if title != "" && title != metadata.Title {
			metadata.AlternativeTitles = append(metadata.AlternativeTitles, title)
		}
	}

	// Extract author (this would require additional API calls to staff relationships)
	// For now, we'll leave it empty

	return metadata
}

// Kitsu API response structures
type kitsuMediaDetail struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes kitsuAttributes `json:"attributes"`
}

type kitsuAttributes struct {
	CreatedAt           string            `json:"createdAt"`
	UpdatedAt           string            `json:"updatedAt"`
	Slug                string            `json:"slug"`
	Synopsis            string            `json:"synopsis"`
	Description         string            `json:"description"`
	CoverImageTopOffset int               `json:"coverImageTopOffset"`
	Titles              map[string]string `json:"titles"`
	CanonicalTitle      string            `json:"canonicalTitle"`
	AbbreviatedTitles   []string          `json:"abbreviatedTitles"`
	AverageRating       string            `json:"averageRating"`
	UserCount           int               `json:"userCount"`
	FavoritesCount      int               `json:"favoritesCount"`
	StartDate           string            `json:"startDate"`
	EndDate             string            `json:"endDate"`
	PopularityRank      int               `json:"popularityRank"`
	RatingRank          *int              `json:"ratingRank"`
	AgeRating           *string           `json:"ageRating"`
	AgeRatingGuide      *string           `json:"ageRatingGuide"`
	Subtype             string            `json:"subtype"`
	Status              string            `json:"status"`
	Tba                 *string           `json:"tba"`
	PosterImage         kitsuImage        `json:"posterImage"`
	CoverImage          *kitsuImage       `json:"coverImage"`
	ChapterCount        *int              `json:"chapterCount"`
	VolumeCount         *int              `json:"volumeCount"`
	Serialization       *string           `json:"serialization"`
	MangaType           *string           `json:"mangaType"`
}

type kitsuImage struct {
	Tiny     string `json:"tiny"`
	Large    string `json:"large"`
	Small    string `json:"small"`
	Medium   string `json:"medium"`
	Original string `json:"original"`
}

// Helper functions
func extractKitsuTitle(titles map[string]string) string {
	// Prefer canonical title
	if title := titles["canonical"]; title != "" {
		return title
	}

	// Prefer English title
	if title, ok := titles["en"]; ok && title != "" {
		return title
	}

	// Fallback to Japanese
	if title, ok := titles["en_jp"]; ok && title != "" {
		return title
	}

	if title, ok := titles["ja_jp"]; ok && title != "" {
		return title
	}

	// Return any available title
	for _, title := range titles {
		if title != "" {
			return title
		}
	}

	return ""
}

func extractKitsuCoverURL(image kitsuImage) string {
	// Prefer large image
	if image.Large != "" {
		return image.Large
	}

	if image.Medium != "" {
		return image.Medium
	}

	if image.Small != "" {
		return image.Small
	}

	if image.Original != "" {
		return image.Original
	}

	return image.Tiny
}

func mapKitsuAgeRating(ageRating *string, ageRatingGuide *string) string {
	if ageRating == nil {
		return "safe"
	}

	switch strings.ToLower(*ageRating) {
	case "g":
		return "safe"
	case "pg":
		return "suggestive"
	case "r":
		if ageRatingGuide != nil && strings.Contains(strings.ToLower(*ageRatingGuide), "horror") {
			return "erotica" // Kitsu uses R for mature content
		}
		return "suggestive"
	case "r18":
		return "explicit"
	default:
		return "safe"
	}
}
