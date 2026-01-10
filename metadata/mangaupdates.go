package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/utils/text"
)

const mangaupdatesBaseURL = "https://api.mangaupdates.com/v1"

// MangaUpdatesProvider implements the Provider interface for MangaUpdates API
type MangaUpdatesProvider struct {
	apiToken string
	config   ConfigProvider
	client   *http.Client // HTTP client for making requests (configurable for testing)
	baseURL  string       // Base URL for API calls (configurable for testing)
}

// NewMangaUpdatesProvider creates a new MangaUpdates metadata provider
func NewMangaUpdatesProvider(apiToken string) Provider {
	return &MangaUpdatesProvider{
		apiToken: apiToken,
		client:   &http.Client{},
		baseURL:  mangaupdatesBaseURL,
	}
}

func init() {
	RegisterProvider("mangaupdates", NewMangaUpdatesProvider)
}

func (m *MangaUpdatesProvider) Name() string {
	return "mangaupdates"
}

func (m *MangaUpdatesProvider) RequiresAuth() bool {
	return false
}

func (m *MangaUpdatesProvider) SetAuthToken(token string) {
	m.apiToken = token
}

func (m *MangaUpdatesProvider) SetConfig(config ConfigProvider) {
	m.config = config
}

func (m *MangaUpdatesProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// MangaUpdates CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (m *MangaUpdatesProvider) Search(title string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("%s/series/search", m.baseURL)

	searchBody := map[string]interface{}{
		"search":  title,
		"page":    1,
		"perpage": 50,
	}

	jsonData, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := http.NewRequest("POST", searchURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search MangaUpdates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaUpdates API returned status: %s", resp.Status)
	}

	var response mangaupdatesSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MangaUpdates response: %w", err)
	}

	if len(response.Results) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(response.Results))
	titleLower := strings.ToLower(title)

	for _, result := range response.Results {
		series := result.Record
		seriesTitle := extractMangaUpdatesTitle(series)
		if seriesTitle == "" {
			continue
		}

		coverURL := extractMangaUpdatesCoverURL(series)
		description := extractMangaUpdatesDescription(series)

		// Extract tags/genres
		var tags []string
		if series.Genres != nil {
			for _, genre := range series.Genres {
				if genre.Genre != "" {
					tags = append(tags, genre.Genre)
				}
			}
		}

		// Extract categories
		if series.Categories != nil {
			for _, category := range series.Categories {
				if category.Category != "" {
					tags = append(tags, category.Category)
				}
			}
		}

		results = append(results, SearchResult{
			ID:              fmt.Sprintf("%d", series.SeriesID),
			Title:           seriesTitle,
			Description:     description,
			CoverArtURL:     coverURL,
			Year:            extractMangaUpdatesYear(series),
			SimilarityScore: text.CompareStrings(titleLower, strings.ToLower(seriesTitle)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (m *MangaUpdatesProvider) GetMetadata(id string) (*MediaMetadata, error) {
	fetchURL := fmt.Sprintf("%s/series/%s", m.baseURL, id)

	resp, err := m.client.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MangaUpdates metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaUpdates API returned status: %s", resp.Status)
	}

	var response mangaupdatesSeriesDetail
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MangaUpdates response: %w", err)
	}

	return m.convertToMediaMetadata(&response), nil
}

func (m *MangaUpdatesProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	results, err := m.Search(title)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	// Find the best match by prioritizing:
	// 1. High similarity score
	// 2. Non-doujinshi content (prefer official series)
	// 3. Higher rating (if available)
	bestMatch := results[0]
	bestScore := calculateMangaUpdatesMatchScore(bestMatch, title)

	for _, result := range results[1:] {
		score := calculateMangaUpdatesMatchScore(result, title)
		if score > bestScore {
			bestMatch = result
			bestScore = score
		}
	}

	// Fetch full metadata for the best match
	return m.GetMetadata(bestMatch.ID)
}

// Helper function to calculate match score
func calculateMangaUpdatesMatchScore(result SearchResult, originalTitle string) float64 {
	baseScore := result.SimilarityScore

	// Penalize doujinshi content
	for _, tag := range result.Tags {
		if strings.ToLower(tag) == "doujinshi" {
			baseScore -= 0.3
			break
		}
	}

	// Boost series with actual years
	if result.Year > 0 {
		baseScore += 0.1
	}

	return baseScore
}

func (m *MangaUpdatesProvider) convertToMediaMetadata(detail *mangaupdatesSeriesDetail) *MediaMetadata {
	metadata := &MediaMetadata{
		Title:            extractMangaUpdatesTitle(*detail),
		Description:      extractMangaUpdatesDescription(*detail),
		Year:             extractMangaUpdatesYear(*detail),
		OriginalLanguage: extractMangaUpdatesLanguage(*detail),
		Status:           extractMangaUpdatesStatus(*detail),
		ContentRating:    extractMangaUpdatesContentRating(*detail),
		CoverArtURL:      extractMangaUpdatesCoverURL(*detail),
		ExternalID:       fmt.Sprintf("%d", detail.SeriesID),
		Type:             extractMangaUpdatesType(*detail),
	}

	// Extract tags/genres
	if detail.Genres != nil {
		for _, genre := range detail.Genres {
			if genre.Genre != "" {
				metadata.Tags = append(metadata.Tags, genre.Genre)
			}
		}
	}

	// Extract categories
	if detail.Categories != nil {
		for _, category := range detail.Categories {
			if category.Category != "" {
				metadata.Tags = append(metadata.Tags, category.Category)
			}
		}
	}

	// Extract alternative titles
	if detail.Associated != nil {
		for _, assoc := range detail.Associated {
			if assoc.Title != "" && assoc.Title != metadata.Title {
				metadata.AlternativeTitles = append(metadata.AlternativeTitles, assoc.Title)
			}
		}
	}

	// Extract author
	if detail.Authors != nil && len(detail.Authors) > 0 {
		metadata.Author = detail.Authors[0].Name
	}

	return metadata
}

// MangaUpdates API response structures
type mangaupdatesSearchResponse struct {
	TotalHits int                        `json:"total_hits"`
	Page      int                        `json:"page"`
	PerPage   int                        `json:"per_page"`
	Results   []mangaupdatesSearchResult `json:"results"`
}

type mangaupdatesSearchResult struct {
	Record   mangaupdatesSeriesDetail `json:"record"`
	HitTitle string                   `json:"hit_title"`
	Metadata map[string]interface{}   `json:"metadata"`
}

type mangaupdatesSeriesDetail struct {
	SeriesID    int    `json:"series_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Image       struct {
		URL struct {
			Original string `json:"original"`
			Thumb    string `json:"thumb"`
		} `json:"url"`
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"image"`
	Type           string  `json:"type"`
	Year           string  `json:"year"`
	BayesianRating float64 `json:"bayesian_rating"`
	RatingVotes    int     `json:"rating_votes"`
	Genres         []struct {
		Genre string `json:"genre"`
	} `json:"genres"`
	Categories []struct {
		Category string `json:"category"`
	} `json:"categories"`
	LatestChapter int    `json:"latest_chapter"`
	Status        string `json:"status"`
	Completed     bool   `json:"completed"`
	Anime         struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"anime"`
	RelatedSeries []struct {
		RelatedSeriesID int    `json:"related_series_id"`
		Title           string `json:"title"`
		RelationType    string `json:"relation_type"`
	} `json:"related_series"`
	Authors []struct {
		AuthorID int    `json:"author_id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
	} `json:"authors"`
	Publishers []struct {
		PublisherID int    `json:"publisher_id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
	} `json:"publishers"`
	Publications []struct {
		PublicationID int    `json:"publication_id"`
		Name          string `json:"name"`
		PublisherName string `json:"publisher_name"`
	} `json:"publications"`
	Associated []struct {
		Title string `json:"title"`
	} `json:"associated"`
	Recommendations []struct {
		SeriesID int    `json:"series_id"`
		Title    string `json:"title"`
	} `json:"recommendations"`
	CategoryRecommendations []struct {
		SeriesID int    `json:"series_id"`
		Title    string `json:"title"`
	} `json:"category_recommendations"`
	Rank struct {
		Position struct {
			Overall int `json:"overall"`
			Year    int `json:"year"`
			Month   int `json:"month"`
			Week    int `json:"week"`
		} `json:"position"`
		OldPosition struct {
			Overall int `json:"overall"`
			Year    int `json:"year"`
			Month   int `json:"month"`
			Week    int `json:"week"`
		} `json:"old_position"`
	} `json:"rank"`
}

// Helper functions
func extractMangaUpdatesTitle(series mangaupdatesSeriesDetail) string {
	return series.Title
}

func extractMangaUpdatesDescription(series mangaupdatesSeriesDetail) string {
	return series.Description
}

func extractMangaUpdatesCoverURL(series mangaupdatesSeriesDetail) string {
	return series.Image.URL.Original
}

func extractMangaUpdatesYear(series mangaupdatesSeriesDetail) int {
	// Parse year from string, default to 0 if invalid
	if series.Year == "" {
		return 0
	}
	// Take first 4 characters if it's a year range like "2020-2021"
	if len(series.Year) >= 4 {
		yearStr := series.Year[:4]
		if year, err := strconv.Atoi(yearStr); err == nil {
			return year
		}
	}
	return 0
}

func extractMangaUpdatesLanguage(series mangaupdatesSeriesDetail) string {
	// MangaUpdates doesn't explicitly provide language, but we can infer from type
	switch strings.ToLower(series.Type) {
	case "japanese manga", "manga":
		return "ja"
	case "korean manhwa", "manhwa":
		return "ko"
	case "chinese manhua", "manhua":
		return "zh"
	default:
		return "ja" // default to Japanese
	}
}

func extractMangaUpdatesStatus(series mangaupdatesSeriesDetail) string {
	if series.Completed {
		return "completed"
	}
	switch strings.ToLower(series.Status) {
	case "complete", "completed":
		return "completed"
	case "ongoing":
		return "ongoing"
	case "hiatus":
		return "hiatus"
	case "cancelled":
		return "cancelled"
	default:
		return "ongoing"
	}
}

func extractMangaUpdatesContentRating(series mangaupdatesSeriesDetail) string {
	// MangaUpdates doesn't have explicit content rating, but we can check genres
	for _, genre := range series.Genres {
		genreLower := strings.ToLower(genre.Genre)
		if strings.Contains(genreLower, "hentai") || strings.Contains(genreLower, "adult") {
			return "pornographic"
		}
		if strings.Contains(genreLower, "ecchi") || strings.Contains(genreLower, "mature") {
			return "erotica"
		}
		if strings.Contains(genreLower, "seinen") || strings.Contains(genreLower, "shoujo") || strings.Contains(genreLower, "shounen") {
			return "suggestive"
		}
	}
	return "safe"
}

func extractMangaUpdatesType(series mangaupdatesSeriesDetail) string {
	typeLower := strings.ToLower(series.Type)
	switch {
	case strings.Contains(typeLower, "manhwa"):
		return "manhwa"
	case strings.Contains(typeLower, "manhua"):
		return "manhua"
	case strings.Contains(typeLower, "webtoon"):
		return "webtoon"
	case strings.Contains(typeLower, "novel"):
		return "novel"
	default:
		return "manga"
	}
}
