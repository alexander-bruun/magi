package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexander-bruun/magi/utils"
)

const anilistBaseURL = "https://graphql.anilist.co"

// AniListProvider implements the Provider interface for AniList GraphQL API
type AniListProvider struct {
	apiToken string
}

// NewAniListProvider creates a new AniList metadata provider
func NewAniListProvider(apiToken string) Provider {
	return &AniListProvider{apiToken: apiToken}
}

func init() {
	RegisterProvider("anilist", NewAniListProvider)
}

func (a *AniListProvider) Name() string {
	return "anilist"
}

func (a *AniListProvider) RequiresAuth() bool {
	return false // AniList API doesn't require auth for public data
}

func (a *AniListProvider) SetAuthToken(token string) {
	a.apiToken = token
}

func (a *AniListProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	if metadata == nil || metadata.CoverArtURL == "" {
		return ""
	}
	// AniList CoverArtURL is already the full URL
	return metadata.CoverArtURL
}

func (a *AniListProvider) Search(title string) ([]SearchResult, error) {
	query := `
		query ($search: String) {
			Page(page: 1, perPage: 50) {
				media(search: $search, type: MANGA) {
					id
					title {
						romaji
						english
						native
					}
					description
					startDate {
						year
					}
					coverImage {
						large
						medium
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"search": title,
	}

	response, err := a.executeQuery(query, variables)
	if err != nil {
		return nil, err
	}

	page, ok := response["Page"].(map[string]interface{})
	if !ok {
		return nil, ErrNoResults
	}

	mediaList, ok := page["media"].([]interface{})
	if !ok || len(mediaList) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(mediaList))
	titleLower := strings.ToLower(title)

	for _, item := range mediaList {
		media, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		id := fmt.Sprintf("%v", media["id"])
		
		titleData := media["title"].(map[string]interface{})
		mangaTitle := extractAniListTitle(titleData)

		description := ""
		if desc, ok := media["description"].(string); ok {
			description = stripHTML(desc)
		}

		coverURL := ""
		if coverImage, ok := media["coverImage"].(map[string]interface{}); ok {
			if large, ok := coverImage["large"].(string); ok {
				coverURL = large
			} else if medium, ok := coverImage["medium"].(string); ok {
				coverURL = medium
			}
		}

		year := 0
		if startDate, ok := media["startDate"].(map[string]interface{}); ok {
			if yearFloat, ok := startDate["year"].(float64); ok {
				year = int(yearFloat)
			}
		}

		results = append(results, SearchResult{
			ID:              id,
			Title:           mangaTitle,
			Description:     description,
			CoverArtURL:     coverURL,
			Year:            year,
			SimilarityScore: utils.CompareStrings(titleLower, strings.ToLower(mangaTitle)),
		})
	}

	return results, nil
}

func (a *AniListProvider) GetMetadata(id string) (*MediaMetadata, error) {
	query := `
		query ($id: Int) {
			Media(id: $id, type: MANGA) {
				id
				title {
					romaji
					english
					native
				}
				description
				startDate {
					year
				}
				endDate {
					year
				}
				status
				countryOfOrigin
				isAdult
				format
				genres
				tags {
					name
				}
				synonyms
				coverImage {
					large
					extraLarge
				}
			}
		}
	`

	idInt := 0
	fmt.Sscanf(id, "%d", &idInt)

	variables := map[string]interface{}{
		"id": idInt,
	}

	response, err := a.executeQuery(query, variables)
	if err != nil {
		return nil, err
	}

	media, ok := response["Media"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid AniList response format")
	}

	return a.convertToMediaMetadata(media), nil
}

func (a *AniListProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	results, err := a.Search(title)
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
	return a.GetMetadata(bestMatch.ID)
}

func (a *AniListProvider) executeQuery(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	requestBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anilistBaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create AniList request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if a.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query AniList: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AniList API returned status: %s", resp.Status)
	}

	var result struct {
		Data   map[string]interface{} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode AniList response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("AniList API error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

func (a *AniListProvider) convertToMediaMetadata(media map[string]interface{}) *MediaMetadata {
	titleData := media["title"].(map[string]interface{})
	title := extractAniListTitle(titleData)

	description := ""
	if desc, ok := media["description"].(string); ok {
		description = stripHTML(desc)
	}

	year := 0
	if startDate, ok := media["startDate"].(map[string]interface{}); ok {
		if yearFloat, ok := startDate["year"].(float64); ok {
			year = int(yearFloat)
		}
	}

	status := "ongoing"
	if statusStr, ok := media["status"].(string); ok {
		status = convertAniListStatus(statusStr)
	}

	countryOfOrigin := "JP"
	if country, ok := media["countryOfOrigin"].(string); ok {
		countryOfOrigin = country
	}

	isAdult := false
	if adult, ok := media["isAdult"].(bool); ok {
		isAdult = adult
	}

	format := "media"
	if formatStr, ok := media["format"].(string); ok {
		format = convertAniListFormat(formatStr)
	}

	coverURL := ""
	if coverImage, ok := media["coverImage"].(map[string]interface{}); ok {
		if extraLarge, ok := coverImage["extraLarge"].(string); ok {
			coverURL = extraLarge
		} else if large, ok := coverImage["large"].(string); ok {
			coverURL = large
		}
	}

	metadata := &MediaMetadata{
		Title:            title,
		Description:      description,
		Year:             year,
		Status:           status,
		ContentRating:    convertAniListContentRating(isAdult),
		CoverArtURL:      coverURL,
		ExternalID:       fmt.Sprintf("%v", media["id"]),
		Type:             convertAniListFormat(format),
		OriginalLanguage: convertCountryToLanguage(countryOfOrigin),
	}

	// Extract genres as tags
	if genres, ok := media["genres"].([]interface{}); ok {
		for _, genre := range genres {
			if genreStr, ok := genre.(string); ok {
				metadata.Tags = append(metadata.Tags, genreStr)
			}
		}
	}

	// Extract tags
	if tags, ok := media["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagMap, ok := tag.(map[string]interface{}); ok {
				if name, ok := tagMap["name"].(string); ok {
					metadata.Tags = append(metadata.Tags, name)
				}
			}
		}
	}

	// Extract synonyms
	if synonyms, ok := media["synonyms"].([]interface{}); ok {
		for _, syn := range synonyms {
			if synStr, ok := syn.(string); ok && synStr != "" && synStr != title {
				metadata.AlternativeTitles = append(metadata.AlternativeTitles, synStr)
			}
		}
	}

	// Add alternative title versions
	if english, ok := titleData["english"].(string); ok && english != "" && english != title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, english)
	}
	if native, ok := titleData["native"].(string); ok && native != "" && native != title {
		metadata.AlternativeTitles = append(metadata.AlternativeTitles, native)
	}

	return metadata
}

// Helper functions
func extractAniListTitle(titleData map[string]interface{}) string {
	if english, ok := titleData["english"].(string); ok && english != "" {
		return english
	}
	if romaji, ok := titleData["romaji"].(string); ok && romaji != "" {
		return romaji
	}
	if native, ok := titleData["native"].(string); ok && native != "" {
		return native
	}
	return ""
}

func convertAniListStatus(status string) string {
	switch strings.ToUpper(status) {
	case "FINISHED":
		return "completed"
	case "RELEASING":
		return "ongoing"
	case "NOT_YET_RELEASED":
		return "upcoming"
	case "CANCELLED":
		return "cancelled"
	case "HIATUS":
		return "hiatus"
	default:
		return "ongoing"
	}
}

func convertAniListContentRating(isAdult bool) string {
	if isAdult {
		return "pornographic"
	}
	return "safe"
}

func convertAniListFormat(format string) string {
	switch strings.ToUpper(format) {
	case "MANGA":
		return "manga"
	case "LIGHT_NOVEL", "NOVEL":
		return "novel"
	case "ONE_SHOT":
		return "oneshot"
	default:
		return "manga"
	}
}

func convertCountryToLanguage(country string) string {
	switch strings.ToUpper(country) {
	case "JP":
		return "ja"
	case "KR":
		return "ko"
	case "CN", "TW", "HK":
		return "zh"
	case "US", "GB":
		return "en"
	default:
		return "ja"
	}
}

func stripHTML(html string) string {
	// Simple HTML tag removal
	result := html
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")
	result = strings.ReplaceAll(result, "<i>", "")
	result = strings.ReplaceAll(result, "</i>", "")
	result = strings.ReplaceAll(result, "<b>", "")
	result = strings.ReplaceAll(result, "</b>", "")
	
	// Remove remaining HTML tags using a simple approach
	for strings.Contains(result, "<") && strings.Contains(result, ">") {
		start := strings.Index(result, "<")
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	
	return strings.TrimSpace(result)
}
