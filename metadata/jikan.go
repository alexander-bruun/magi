package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexander-bruun/magi/utils"
)

const jikanBaseURL = "https://api.jikan.moe/v4"

// JikanProvider implements the Provider interface for Jikan API (unofficial MAL API)
type JikanProvider struct {
	apiToken string // Not used, but kept for interface compatibility
}

// NewJikanProvider creates a new Jikan API metadata provider
func NewJikanProvider(apiToken string) Provider {
	return &JikanProvider{apiToken: apiToken}
}

func init() {
	RegisterProvider("jikan", NewJikanProvider)
}

func (j *JikanProvider) Name() string {
	return "jikan"
}

func (j *JikanProvider) RequiresAuth() bool {
	return false
}

func (j *JikanProvider) SetAuthToken(token string) {
	j.apiToken = token
}

func (j *JikanProvider) GetCoverImageURL(metadata *MangaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// Jikan CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (j *JikanProvider) Search(title string) ([]SearchResult, error) {
	titleEncoded := url.QueryEscape(title)
	searchURL := fmt.Sprintf("%s/manga?q=%s&limit=50&order_by=popularity", jikanBaseURL, titleEncoded)

	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jikan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jikan API returned status: %s", resp.Status)
	}

	var response struct {
		Data []jikanMangaData `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Jikan response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(response.Data))
	titleLower := strings.ToLower(title)

	for _, manga := range response.Data {
		coverURL := ""
		if manga.Images.JPG.LargeImageURL != "" {
			coverURL = manga.Images.JPG.LargeImageURL
		} else if manga.Images.JPG.ImageURL != "" {
			coverURL = manga.Images.JPG.ImageURL
		}

		year := 0
		if manga.Published.From != "" {
			fmt.Sscanf(manga.Published.From, "%d", &year)
		}

		results = append(results, SearchResult{
			ID:              fmt.Sprintf("%d", manga.MalID),
			Title:           manga.Title,
			Description:     manga.Synopsis,
			CoverArtURL:     coverURL,
			Year:            year,
			SimilarityScore: utils.CompareStrings(titleLower, strings.ToLower(manga.Title)),
		})
	}

	return results, nil
}

func (j *JikanProvider) GetMetadata(id string) (*MangaMetadata, error) {
	fetchURL := fmt.Sprintf("%s/manga/%s/full", jikanBaseURL, id)

	resp, err := http.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Jikan metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jikan API returned status: %s", resp.Status)
	}

	var response struct {
		Data jikanMangaData `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Jikan response: %w", err)
	}

	return j.convertToMangaMetadata(&response.Data), nil
}

func (j *JikanProvider) FindBestMatch(title string) (*MangaMetadata, error) {
	results, err := j.Search(title)
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
	return j.GetMetadata(bestMatch.ID)
}

func (j *JikanProvider) convertToMangaMetadata(data *jikanMangaData) *MangaMetadata {
	coverURL := ""
	if data.Images.JPG.LargeImageURL != "" {
		coverURL = data.Images.JPG.LargeImageURL
	} else if data.Images.JPG.ImageURL != "" {
		coverURL = data.Images.JPG.ImageURL
	}

	year := 0
	if data.Published.From != "" {
		fmt.Sscanf(data.Published.From, "%d", &year)
	}

	metadata := &MangaMetadata{
		Title:        data.Title,
		Description:  data.Synopsis,
		Year:         year,
		Status:       convertJikanStatus(data.Status),
		ContentRating: convertJikanRating(data.Demographics),
		CoverArtURL:  coverURL,
		ExternalID:   fmt.Sprintf("%d", data.MalID),
		Type:         convertJikanType(data.Type),
	}

	// Extract tags from genres, themes, and demographics
	for _, genre := range data.Genres {
		metadata.Tags = append(metadata.Tags, genre.Name)
	}
	for _, theme := range data.Themes {
		metadata.Tags = append(metadata.Tags, theme.Name)
	}
	for _, demo := range data.Demographics {
		metadata.Tags = append(metadata.Tags, demo.Name)
	}

	// Extract alternative titles
	if data.TitleEnglish != "" && data.TitleEnglish != data.Title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, data.TitleEnglish)
	}
	if data.TitleJapanese != "" && data.TitleJapanese != data.Title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, data.TitleJapanese)
	}
	for _, syn := range data.TitleSynonyms {
		if syn != "" && syn != data.Title {
			metadata.AlternativeTitles = append(metadata.AlternativeTitles, syn)
		}
	}

	// Determine language based on type
	switch metadata.Type {
	case "manga":
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

// Jikan API response structures
type jikanMangaData struct {
	MalID          int    `json:"mal_id"`
	Title          string `json:"title"`
	TitleEnglish   string `json:"title_english"`
	TitleJapanese  string `json:"title_japanese"`
	TitleSynonyms  []string `json:"title_synonyms"`
	Synopsis       string `json:"synopsis"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Published      struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"published"`
	Images struct {
		JPG struct {
			ImageURL      string `json:"image_url"`
			SmallImageURL string `json:"small_image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"jpg"`
	} `json:"images"`
	Genres []struct {
		MalID int    `json:"mal_id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
	} `json:"genres"`
	Themes []struct {
		MalID int    `json:"mal_id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
	} `json:"themes"`
	Demographics []struct {
		MalID int    `json:"mal_id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
	} `json:"demographics"`
}

// Helper functions
func convertJikanStatus(status string) string {
	switch strings.ToLower(status) {
	case "finished":
		return "completed"
	case "publishing":
		return "ongoing"
	case "on hiatus":
		return "hiatus"
	case "discontinued":
		return "cancelled"
	default:
		return "ongoing"
	}
}

func convertJikanRating(demographics []struct {
	MalID int    `json:"mal_id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
}) string {
	// Check demographics for content rating hints
	for _, demo := range demographics {
		name := strings.ToLower(demo.Name)
		if strings.Contains(name, "hentai") || strings.Contains(name, "erotica") {
			return "pornographic"
		}
		if strings.Contains(name, "ecchi") {
			return "suggestive"
		}
	}
	return "safe"
}

func convertJikanType(mangaType string) string {
	switch strings.ToLower(mangaType) {
	case "manga":
		return "manga"
	case "manhwa":
		return "manhwa"
	case "manhua":
		return "manhua"
	case "novel", "light novel":
		return "novel"
	case "one-shot":
		return "oneshot"
	case "doujinshi":
		return "doujinshi"
	default:
		return "manga"
	}
}
