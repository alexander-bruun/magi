package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexander-bruun/magi/utils/text"
	"github.com/gofiber/fiber/v3/log"
)

const mangadexBaseURL = "https://api.mangadex.org"

// MangaDexProvider implements the Provider interface for MangaDex API
type MangaDexProvider struct {
	BaseProvider
}

// NewMangaDexProvider creates a new MangaDex metadata provider
func NewMangaDexProvider(apiToken string) Provider {
	return &MangaDexProvider{
		BaseProvider: BaseProvider{
			ProviderName: "mangadex",
			APIToken:     apiToken,
			Client:       &http.Client{},
			BaseURL:      mangadexBaseURL,
		},
	}
}

func init() {
	RegisterProvider("mangadex", NewMangaDexProvider)
}

func (m *MangaDexProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	return DefaultFindBestMatch(m, title)
}

func (m *MangaDexProvider) Search(title string) ([]SearchResult, error) {
	titleEncoded := url.QueryEscape(title)

	// Build content rating query parameters based on global setting
	var contentRatingParams []string
	if m.Config != nil {
		limit := m.Config.GetContentRatingLimit()
		if limit >= 0 {
			contentRatingParams = append(contentRatingParams, "contentRating[]=safe")
		}
		if limit >= 1 {
			contentRatingParams = append(contentRatingParams, "contentRating[]=suggestive")
		}
		if limit >= 2 {
			contentRatingParams = append(contentRatingParams, "contentRating[]=erotica")
		}
		if limit >= 3 {
			contentRatingParams = append(contentRatingParams, "contentRating[]=pornographic")
		}
	} else {
		// Default to all if no config
		contentRatingParams = []string{
			"contentRating[]=safe",
			"contentRating[]=suggestive",
			"contentRating[]=erotica",
			"contentRating[]=pornographic",
		}
	}

	contentRatingQuery := strings.Join(contentRatingParams, "&")
	searchURL := fmt.Sprintf("%s/manga?title=%s&limit=50&%s&includes[]=cover_art", m.BaseURL, titleEncoded, contentRatingQuery)

	resp, err := m.Client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search MangaDex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API returned status: %s", resp.Status)
	}

	var response struct {
		Result string                `json:"result"`
		Data   []mangadexMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MangaDex response: %w", err)
	}

	if response.Result != "ok" {
		return nil, fmt.Errorf("MangaDex API error: %s", response.Result)
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
			SimilarityScore: text.CompareStrings(titleLower, strings.ToLower(mangaTitle)),
			Tags:            tags,
		})
	}

	return results, nil
}

func (m *MangaDexProvider) GetMetadata(id string) (*MediaMetadata, error) {
	fetchURL := fmt.Sprintf("%s/manga/%s?includes[]=cover_art&includes[]=author&includes[]=artist&includes[]=scanlation_group", m.BaseURL, id)

	resp, err := m.Client.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MangaDex metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API returned status: %s", resp.Status)
	}

	var response struct {
		Result string              `json:"result"`
		Data   mangadexMediaDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode MangaDex response: %w", err)
	}

	if response.Result != "ok" {
		return nil, fmt.Errorf("MangaDex API error: %s", response.Result)
	}

	return m.convertToMediaMetadata(&response.Data), nil
}

func (m *MangaDexProvider) convertToMediaMetadata(detail *mangadexMediaDetail) *MediaMetadata {
	metadata := &MediaMetadata{
		Title:            extractBestTitle(detail.Attributes.Title, detail.Attributes.AltTitles),
		Description:      extractDescription(detail.Attributes.Description),
		Year:             detail.Attributes.Year,
		OriginalLanguage: detail.Attributes.OriginalLanguage,
		Status:           detail.Attributes.Status,
		ContentRating:    normalizeMangaDexContentRating(detail.Attributes.ContentRating),
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

	// Extract authors and artists from relationships
	for _, rel := range detail.Relationships {
		switch rel.Type {
		case "author":
			if name, ok := rel.Attributes["name"].(string); ok && name != "" {
				authorInfo := AuthorInfo{
					Name: name,
					Role: "author",
				}
				metadata.Authors = append(metadata.Authors, authorInfo)
				if metadata.Author == "" {
					metadata.Author = name
				}
			}
		case "artist":
			if name, ok := rel.Attributes["name"].(string); ok && name != "" {
				artistInfo := AuthorInfo{
					Name: name,
					Role: "artist",
				}
				metadata.Artists = append(metadata.Artists, artistInfo)
			}
		}
	}

	return metadata
}

// MangaDex API response structures
type mangadexMediaDetail struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Attributes    mangadexAttributes     `json:"attributes"`
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

// normalizeMangaDexContentRating maps MangaDex API content rating to internal values.
func normalizeMangaDexContentRating(rating string) string {
	if strings.ToLower(rating) == "pornographic" {
		return "explicit"
	}
	return rating
}

type mangadexTag struct {
	ID         string `json:"id"`
	Attributes struct {
		Name map[string]string `json:"name"`
	} `json:"attributes"`
}

type mangadexRelationship struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
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
