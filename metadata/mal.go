package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexander-bruun/magi/utils"
)

const malBaseURL = "https://api.myanimelist.net/v2"

// MALProvider implements the Provider interface for MyAnimeList API
type MALProvider struct {
	apiToken string
}

// NewMALProvider creates a new MyAnimeList metadata provider
func NewMALProvider(apiToken string) Provider {
	return &MALProvider{apiToken: apiToken}
}

func init() {
	RegisterProvider("mal", NewMALProvider)
}

func (m *MALProvider) Name() string {
	return "mal"
}

func (m *MALProvider) RequiresAuth() bool {
	return true
}

func (m *MALProvider) SetAuthToken(token string) {
	m.apiToken = token
}

func (m *MALProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// MAL CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (m *MALProvider) Search(title string) ([]SearchResult, error) {
	if m.apiToken == "" {
		return nil, ErrAuthRequired
	}

	titleEncoded := url.QueryEscape(title)
	searchURL := fmt.Sprintf("%s/series?q=%s&limit=50&fields=id,title,synopsis,main_picture,start_date,mean,media_type,alternative_titles,genres", malBaseURL, titleEncoded)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MAL request: %w", err)
	}

	req.Header.Set("X-MAL-CLIENT-ID", m.apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search MAL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidCredentials
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API returned status: %s", resp.Status)
	}

	var response struct {
		Data []struct {
			Node malMediaNode `json:"node"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MAL response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(response.Data))
	titleLower := strings.ToLower(title)

	for _, item := range response.Data {
		node := item.Node
		coverURL := ""
		if node.MainPicture.Medium != "" {
			coverURL = node.MainPicture.Medium
		} else if node.MainPicture.Large != "" {
			coverURL = node.MainPicture.Large
		}

		year := 0
		if node.StartDate != "" {
			fmt.Sscanf(node.StartDate, "%d", &year)
		}

		// Extract tags from genres
		var tags []string
		for _, genre := range node.Genres {
			tags = append(tags, genre.Name)
		}

		results = append(results, SearchResult{
			ID:              fmt.Sprintf("%d", node.ID),
			Title:           node.Title,
			Description:     node.Synopsis,
			CoverArtURL:     coverURL,
			Year:            year,
			SimilarityScore: utils.CompareStrings(titleLower, strings.ToLower(node.Title)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (m *MALProvider) GetMetadata(id string) (*MediaMetadata, error) {
	if m.apiToken == "" {
		return nil, ErrAuthRequired
	}

	fetchURL := fmt.Sprintf("%s/series/%s?fields=id,title,synopsis,main_picture,start_date,end_date,mean,media_type,status,genres,alternative_titles,nsfw,num_chapters", malBaseURL, id)

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MAL request: %w", err)
	}

	req.Header.Set("X-MAL-CLIENT-ID", m.apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MAL metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidCredentials
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API returned status: %s", resp.Status)
	}

	var node malMediaNode
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("failed to decode MAL response: %w", err)
	}

	return m.convertToMediaMetadata(&node), nil
}

func (m *MALProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	results, err := m.Search(title)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	// Find the result with the highest similarity score
	bestMatch := results[0]
	for _, result := range results[1:] {
		if result.SimilarityScore > bestMatch.SimilarityScore {
			bestMatch = result
		}
	}

	// Fetch full metadata for the best match
	return m.GetMetadata(bestMatch.ID)
}

func (m *MALProvider) convertToMediaMetadata(node *malMediaNode) *MediaMetadata {
	coverURL := ""
	if node.MainPicture.Large != "" {
		coverURL = node.MainPicture.Large
	} else if node.MainPicture.Medium != "" {
		coverURL = node.MainPicture.Medium
	}

	year := 0
	if node.StartDate != "" {
		fmt.Sscanf(node.StartDate, "%d", &year)
	}

	metadata := &MediaMetadata{
		Title:        node.Title,
		Description:  node.Synopsis,
		Year:         year,
		Status:       convertMALStatus(node.Status),
		ContentRating: convertMALContentRating(node.NSFW),
		CoverArtURL:  coverURL,
		ExternalID:   fmt.Sprintf("%d", node.ID),
		Type:         convertMALMediaType(node.MediaType),
	}

	// Extract tags from genres
	for _, genre := range node.Genres {
		metadata.Tags = append(metadata.Tags, genre.Name)
	}

	// Extract alternative titles
	if node.AlternativeTitles.En != "" && node.AlternativeTitles.En != node.Title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, node.AlternativeTitles.En)
	}
	if node.AlternativeTitles.Ja != "" && node.AlternativeTitles.Ja != node.Title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, node.AlternativeTitles.Ja)
	}
	for _, syn := range node.AlternativeTitles.Synonyms {
		if syn != "" && syn != node.Title {
			metadata.AlternativeTitles = append(metadata.AlternativeTitles, syn)
		}
	}

	// Try to determine language from media type
	switch metadata.Type {
	case "media":
		metadata.OriginalLanguage = "ja"
	case "manhwa":
		metadata.OriginalLanguage = "ko"
	case "manhua":
		metadata.OriginalLanguage = "zh"
	default:
		metadata.OriginalLanguage = "ja"
	}

	return metadata
}

// MAL API response structures
type malMediaNode struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Synopsis          string `json:"synopsis"`
	StartDate         string `json:"start_date"`
	EndDate           string `json:"end_date"`
	Status            string `json:"status"`
	MediaType         string `json:"media_type"`
	NSFW              string `json:"nsfw"`
	NumChapters       int    `json:"num_chapters"`
	MainPicture       struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"main_picture"`
	AlternativeTitles struct {
		Synonyms []string `json:"synonyms"`
		En       string   `json:"en"`
		Ja       string   `json:"ja"`
	} `json:"alternative_titles"`
	Genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

// Helper functions to convert MAL formats to our standard
func convertMALStatus(status string) string {
	switch strings.ToLower(status) {
	case "finished":
		return "completed"
	case "currently_publishing":
		return "ongoing"
	case "on_hiatus":
		return "hiatus"
	case "discontinued":
		return "cancelled"
	default:
		return "ongoing"
	}
}

func convertMALContentRating(nsfw string) string {
	switch strings.ToLower(nsfw) {
	case "white":
		return "safe"
	case "gray":
		return "suggestive"
	case "black":
		return "pornographic"
	default:
		return "safe"
	}
}

func convertMALMediaType(mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "manga":
		return "manga"
	case "manhwa":
		return "manhwa"
	case "manhua":
		return "manhua"
	case "one_shot":
		return "oneshot"
	case "doujinshi":
		return "doujinshi"
	case "light_novel", "novel":
		return "novel"
	default:
		return "manga"
	}
}
