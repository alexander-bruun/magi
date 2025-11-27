package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

const mangadexBaseURL = "https://api.mangadex.org"

// MediadexProvider implements the Provider interface for MediaDex API
type MediadexProvider struct {
	apiToken string
}

// NewMediadexProvider creates a new MediaDex metadata provider
func NewMediadexProvider(apiToken string) Provider {
	return &MediadexProvider{apiToken: apiToken}
}

func init() {
	RegisterProvider("mangadex", NewMediadexProvider)
}

func (m *MediadexProvider) Name() string {
	return "mangadex"
}

func (m *MediadexProvider) RequiresAuth() bool {
	return false
}

func (m *MediadexProvider) SetAuthToken(token string) {
	m.apiToken = token
}

func (m *MediadexProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// MediaDex CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (m *MediadexProvider) Search(title string) ([]SearchResult, error) {
	titleEncoded := url.QueryEscape(title)
	searchURL := fmt.Sprintf("%s/manga?title=%s&limit=50&contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica&contentRating[]=pornographic&includes[]=cover_art", mangadexBaseURL, titleEncoded)

	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search MediaDex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MediaDex API returned status: %s", resp.Status)
	}

	var response struct {
		Result   string `json:"result"`
		Data     []mangadexMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MediaDex response: %w", err)
	}

	if response.Result != "ok" {
		return nil, fmt.Errorf("MediaDex API error: %s", response.Result)
	}

	if len(response.Data) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(response.Data))
	titleLower := strings.ToLower(title)

	for _, media := range response.Data {
		mangaTitle := extractBestTitle(media.Attributes.Title, media.Attributes.AltTitles)
		if mangaTitle == "" {
			continue
		}

		coverURL := extractCoverURL(media.ID, media.Relationships)
		description := extractDescription(media.Attributes.Description)

		// Extract tags
		var tags []string
		for _, tag := range media.Attributes.Tags {
			if name, ok := tag.Attributes.Name["en"]; ok && name != "" {
				tags = append(tags, name)
			} else {
				for _, v := range tag.Attributes.Name {
					if v != "" {
						tags = append(tags, v)
						break
					}
				}
			}
		}

		results = append(results, SearchResult{
			ID:              media.ID,
			Title:           mangaTitle,
			Description:     description,
			CoverArtURL:     coverURL,
			Year:            media.Attributes.Year,
			SimilarityScore: utils.CompareStrings(titleLower, strings.ToLower(mangaTitle)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (m *MediadexProvider) GetMetadata(id string) (*MediaMetadata, error) {
	fetchURL := fmt.Sprintf("%s/manga/%s?includes[]=cover_art", mangadexBaseURL, id)

	resp, err := http.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MediaDex metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MediaDex API returned status: %s", resp.Status)
	}

	var response struct {
		Result   string `json:"result"`
		Data     mangadexMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MediaDex response: %w", err)
	}

	if response.Result != "ok" {
		return nil, fmt.Errorf("MediaDex API error: %s", response.Result)
	}

	return m.convertToMediaMetadata(&response.Data), nil
}

func (m *MediadexProvider) FindBestMatch(title string) (*MediaMetadata, error) {
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

func (m *MediadexProvider) convertToMediaMetadata(detail *mangadexMediaDetail) *MediaMetadata {
	metadata := &MediaMetadata{
		Title:            extractBestTitle(detail.Attributes.Title, detail.Attributes.AltTitles),
		Description:      extractDescription(detail.Attributes.Description),
		Year:             detail.Attributes.Year,
		OriginalLanguage: detail.Attributes.OriginalLanguage,
		Status:           detail.Attributes.Status,
		ContentRating:    detail.Attributes.ContentRating,
		CoverArtURL:      extractCoverURL(detail.ID, detail.Relationships),
		ExternalID:       detail.ID,
		Type:             determineMediaType(detail),
	}

	// Extract tags
	for _, tag := range detail.Attributes.Tags {
		if name, ok := tag.Attributes.Name["en"]; ok && name != "" {
			metadata.Tags = append(metadata.Tags, name)
		} else {
			for _, v := range tag.Attributes.Name {
				if v != "" {
					metadata.Tags = append(metadata.Tags, v)
					break
				}
			}
		}
	}

	log.Debugf("Extracted %d tags for MangaDex media %s: %v", len(metadata.Tags), detail.ID, metadata.Tags)

	// Extract alternative titles
	for _, altTitleMap := range detail.Attributes.AltTitles {
		for _, title := range altTitleMap {
			if title != "" && title != metadata.Title {
				metadata.AlternativeTitles = append(metadata.AlternativeTitles, title)
			}
		}
	}

	return metadata
}

// MediaDex API response structures
type mangadexMediaDetail struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Attributes    mangadexAttributes `json:"attributes"`
	Relationships []mangadexRelationship `json:"relationships"`
}

type mangadexAttributes struct {
	Title            map[string]string   `json:"title"`
	AltTitles        []map[string]string `json:"altTitles"`
	Description      map[string]string   `json:"description"`
	OriginalLanguage string              `json:"originalLanguage"`
	Status           string              `json:"status"`
	Year             int                 `json:"year"`
	ContentRating    string              `json:"contentRating"`
	Tags             []mangadexTag       `json:"tags"`
}

type mangadexTag struct {
	ID         string `json:"id"`
	Attributes struct {
		Name map[string]string `json:"name"`
	} `json:"attributes"`
}

type mangadexRelationship struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Helper functions
func extractBestTitle(titles map[string]string, altTitles []map[string]string) string {
	// Prefer English title
	if title, ok := titles["en"]; ok && title != "" {
		return title
	}

	// Check alternative English titles
	for _, altTitleMap := range altTitles {
		if title, ok := altTitleMap["en"]; ok && title != "" {
			return title
		}
	}

	// Fallback to Japanese
	if title, ok := titles["ja"]; ok && title != "" {
		return title
	}

	// Check alternative Japanese titles
	for _, altTitleMap := range altTitles {
		if title, ok := altTitleMap["ja"]; ok && title != "" {
			return title
		}
	}

	// Return any available title
	for _, title := range titles {
		if title != "" {
			return title
		}
	}

	return ""
}

func extractDescription(descriptions map[string]string) string {
	if desc, ok := descriptions["en"]; ok && desc != "" {
		return desc
	}
	for _, desc := range descriptions {
		if desc != "" {
			return desc
		}
	}
	return ""
}

func extractCoverURL(mangaID string, relationships []mangadexRelationship) string {
	for _, rel := range relationships {
		if rel.Type == "cover_art" {
			if fileName, ok := rel.Attributes["fileName"].(string); ok {
				return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangaID, fileName)
			}
		}
	}
	return ""
}

func determineMediaType(detail *mangadexMediaDetail) string {
	for _, tag := range detail.Attributes.Tags {
		for _, name := range tag.Attributes.Name {
			lname := strings.ToLower(strings.TrimSpace(name))
			if strings.Contains(lname, "webtoon") || strings.Contains(lname, "web comic") || strings.Contains(lname, "webcomic") {
				return "webtoon"
			}
		}
	}

	// Determine by language
	switch strings.ToLower(strings.TrimSpace(detail.Attributes.OriginalLanguage)) {
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
