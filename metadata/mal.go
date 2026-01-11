package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils/text"
)

const malBaseURL = "https://api.myanimelist.net/v2"

// MALTokenResponse represents the OAuth2 token response from MAL
type MALTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// MALProvider implements the Provider interface for MyAnimeList API
type MALProvider struct {
	clientID     string
	clientSecret string
	accessToken  string    // Cached access token
	tokenExpiry  time.Time // When the token expires
	config       ConfigProvider
	client       *http.Client // HTTP client for making requests (configurable for testing)
	baseURL      string       // Base URL for API calls (configurable for testing)
}

// NewMALProvider creates a new MyAnimeList metadata provider
// apiToken should be in the format "clientID:clientSecret"
func NewMALProvider(apiToken string) Provider {
	parts := strings.Split(apiToken, ":")
	clientID := ""
	clientSecret := ""
	if len(parts) >= 1 {
		clientID = parts[0]
	}
	if len(parts) >= 2 {
		clientSecret = parts[1]
	}

	return &MALProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       &http.Client{},
		baseURL:      malBaseURL,
	}
}

func init() {
	// Deprecated: MAL provider requires authentication and is no longer supported
	// RegisterProvider("mal", NewMALProvider)
}

// getAccessToken retrieves an OAuth2 access token using client credentials flow
// Note: MAL doesn't support client_credentials grant type, so we use client ID as API key
func (m *MALProvider) getAccessToken() error {
	// For MAL, we don't actually get OAuth tokens - we use the client ID directly
	// as an API key in the X-MAL-CLIENT-ID header
	if m.clientID == "" {
		return fmt.Errorf("MAL client ID is required")
	}

	// Set the access token to the client ID for compatibility with the rest of the code
	m.accessToken = m.clientID
	m.tokenExpiry = time.Now().Add(24 * time.Hour) // API keys don't expire

	return nil
}

func (m *MALProvider) Name() string {
	return "mal"
}

func (m *MALProvider) RequiresAuth() bool {
	return true
}

func (m *MALProvider) SetAuthToken(token string) {
	// For backward compatibility, accept "clientID:clientSecret" format
	parts := strings.Split(token, ":")
	if len(parts) >= 1 {
		m.clientID = parts[0]
	}
	if len(parts) >= 2 {
		m.clientSecret = parts[1]
	}
	// Clear cached token to force refresh
	m.accessToken = ""
	m.tokenExpiry = time.Time{}
}

func (m *MALProvider) SetConfig(config ConfigProvider) {
	m.config = config
}

func (m *MALProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// MAL CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (m *MALProvider) Search(title string) ([]SearchResult, error) {
	// Search both anime and manga
	animeResults, err := m.searchMediaType(title, "anime")
	if err != nil {
		return nil, err
	}

	mangaResults, err := m.searchMediaType(title, "manga")
	if err != nil {
		return nil, err
	}

	// Combine results
	allResults := append(animeResults, mangaResults...)

	// Sort by similarity score (highest first)
	// The FindBestMatch will pick the highest scoring result
	return allResults, nil
}

// searchMediaType searches for a specific media type (anime or manga)
func (m *MALProvider) searchMediaType(title, mediaType string) ([]SearchResult, error) {
	// Ensure we have a valid access token
	if err := m.getAccessToken(); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	titleEncoded := url.QueryEscape(title)
	searchURL := fmt.Sprintf("%s/%s?q=%s&limit=50&fields=id,title,synopsis,main_picture,start_date,mean,media_type,alternative_titles,genres", m.baseURL, mediaType, titleEncoded)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MAL request: %w", err)
	}

	req.Header.Set("X-MAL-CLIENT-ID", m.accessToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search MAL %s: %w", mediaType, err)
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
		return []SearchResult{}, nil
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
			SimilarityScore: text.CompareStrings(titleLower, strings.ToLower(node.Title)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (m *MALProvider) GetMetadata(id string) (*MediaMetadata, error) {
	// Try anime first
	meta, err := m.getMetadataForType(id, "anime")
	if err == nil {
		return meta, nil
	}

	// If anime fails, try manga
	return m.getMetadataForType(id, "manga")
}

func (m *MALProvider) getMetadataForType(id, mediaType string) (*MediaMetadata, error) {
	// Ensure we have a valid access token
	if err := m.getAccessToken(); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	fetchURL := fmt.Sprintf("%s/%s/%s?fields=id,title,synopsis,main_picture,start_date,end_date,mean,media_type,status,genres,alternative_titles,nsfw,num_chapters,num_volumes,authors,serialization,demographics,recommendations", m.baseURL, mediaType, id)

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MAL request: %w", err)
	}

	req.Header.Set("X-MAL-CLIENT-ID", m.accessToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MAL metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidCredentials
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("media not found")
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
	startDate := ""
	if node.StartDate != "" {
		startDate = node.StartDate
		if len(node.StartDate) >= 4 {
			fmt.Sscanf(node.StartDate[:4], "%d", &year)
		}
	}

	endDate := ""
	if node.EndDate != "" {
		endDate = node.EndDate
	}

	metadata := &MediaMetadata{
		Title:         node.Title,
		Description:   node.Synopsis,
		Year:          year,
		Status:        convertMALStatus(node.Status),
		ContentRating: convertMALContentRating(node.NSFW),
		CoverArtURL:   coverURL,
		ExternalID:    fmt.Sprintf("%d", node.ID),
		Type:          convertMALMediaType(node.MediaType),
		StartDate:     startDate,
		EndDate:       endDate,
		ChapterCount:  node.NumChapters,
		VolumeCount:   node.NumVolumes,
		AverageScore:  node.Mean,
	}

	// Extract tags from genres
	for _, genre := range node.Genres {
		metadata.Genres = append(metadata.Genres, genre.Name)
		metadata.Tags = append(metadata.Tags, genre.Name)
	}

	// Extract authors
	for _, author := range node.Authors {
		fullName := strings.TrimSpace(author.FirstName + " " + author.LastName)
		if fullName == " " {
			fullName = author.FirstName + author.LastName
		}

		authorInfo := AuthorInfo{
			Name: fullName,
			Role: author.Role,
		}

		roleLower := strings.ToLower(author.Role)
		if strings.Contains(roleLower, "author") || strings.Contains(roleLower, "writer") || strings.Contains(roleLower, "story") {
			metadata.Authors = append(metadata.Authors, authorInfo)
			if metadata.Author == "" {
				metadata.Author = fullName
			}
		} else if strings.Contains(roleLower, "artist") || strings.Contains(roleLower, "illustrator") {
			metadata.Artists = append(metadata.Artists, authorInfo)
		}
	}

	// Extract serialization information
	for _, serial := range node.Serialization {
		if metadata.Magazine == "" {
			metadata.Magazine = serial.Name
		} else {
			metadata.Magazine += ", " + serial.Name
		}
	}

	// Extract demographics
	for _, demo := range node.Demographics {
		if metadata.Demographic == "" {
			metadata.Demographic = demo.Name
		} else {
			metadata.Demographic += ", " + demo.Name
		}
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
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Synopsis    string  `json:"synopsis"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Status      string  `json:"status"`
	MediaType   string  `json:"media_type"`
	NSFW        string  `json:"nsfw"`
	NumChapters int     `json:"num_chapters"`
	NumVolumes  int     `json:"num_volumes"`
	Mean        float64 `json:"mean"`
	MainPicture struct {
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
	Authors []struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
	} `json:"authors"`
	Serialization []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"serialization"`
	Demographics []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"demographics"`
	Recommendations []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"recommendations"`
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
