package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexander-bruun/magi/utils/text"
	"github.com/gofiber/fiber/v3/log"
)

const anilistBaseURL = "https://graphql.anilist.co"

// AniListProvider implements the Provider interface for AniList GraphQL API
type AniListProvider struct {
	BaseProvider
}

// NewAniListProvider creates a new AniList metadata provider
func NewAniListProvider(apiToken string) Provider {
	return &AniListProvider{
		BaseProvider: BaseProvider{
			ProviderName: "anilist",
			APIToken:     apiToken,
			BaseURL:      anilistBaseURL,
		},
	}
}

func init() {
	RegisterProvider("anilist", NewAniListProvider)
}

func (a *AniListProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	return DefaultFindBestMatch(a, title)
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
					genres
					tags {
						name
					}
				}
			}
		}
	`

	variables := map[string]any{
		"search": title,
	}

	response, err := a.executeQuery(query, variables)
	if err != nil {
		return nil, err
	}

	page, ok := response["Page"].(map[string]any)
	if !ok {
		return nil, ErrNoResults
	}

	mediaList, ok := page["media"].([]any)
	if !ok || len(mediaList) == 0 {
		return nil, ErrNoResults
	}

	results := make([]SearchResult, 0, len(mediaList))
	titleLower := strings.ToLower(title)

	for _, item := range mediaList {
		media, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id := fmt.Sprintf("%v", media["id"])

		titleData := media["title"].(map[string]any)
		mangaTitle := extractAniListTitle(titleData)

		description := ""
		if desc, ok := media["description"].(string); ok {
			description = stripHTML(desc)
		}

		coverURL := ""
		if coverImage, ok := media["coverImage"].(map[string]any); ok {
			if large, ok := coverImage["large"].(string); ok {
				coverURL = large
			} else if medium, ok := coverImage["medium"].(string); ok {
				coverURL = medium
			}
		}

		year := 0
		if startDate, ok := media["startDate"].(map[string]any); ok {
			if yearFloat, ok := startDate["year"].(float64); ok {
				year = int(yearFloat)
			}
		}

		// Extract tags
		var tags []string
		if genres, ok := media["genres"].([]any); ok {
			for _, genre := range genres {
				if genreStr, ok := genre.(string); ok {
					tags = append(tags, genreStr)
				}
			}
		}
		if tagList, ok := media["tags"].([]any); ok {
			for _, tag := range tagList {
				if tagMap, ok := tag.(map[string]any); ok {
					if name, ok := tagMap["name"].(string); ok {
						tags = append(tags, name)
					}
				}
			}
		}

		results = append(results, SearchResult{
			ID:              id,
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
					month
					day
				}
				endDate {
					year
					month
					day
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
				chapters
				volumes
				averageScore
				popularity
				favourites
				staff(sort: RELEVANCE) {
					edges {
						role
						node {
							name {
								full
							}
						}
					}
				}
				characters(sort: ROLE) {
					edges {
						role
						node {
							name {
								full
							}
						}
					}
				}
				relations {
					edges {
						relationType
						node {
							id
							title {
								english
								romaji
								native
							}
							type
						}
					}
				}
			}
		}
	`

	idInt := 0
	fmt.Sscanf(id, "%d", &idInt)

	variables := map[string]any{
		"id": idInt,
	}

	response, err := a.executeQuery(query, variables)
	if err != nil {
		return nil, err
	}

	media, ok := response["Media"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid AniList response format")
	}

	return a.convertToMediaMetadata(media), nil
}

func (a *AniListProvider) executeQuery(query string, variables map[string]any) (map[string]any, error) {
	requestBody := map[string]any{
		"query":     query,
		"variables": variables,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", a.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create AniList request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if a.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIToken)
	}

	client := a.HTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query AniList: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AniList API returned status: %s", resp.Status)
	}

	var result struct {
		Data   map[string]any `json:"data"`
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

func (a *AniListProvider) convertToMediaMetadata(media map[string]any) *MediaMetadata {
	titleData := media["title"].(map[string]any)
	title := extractAniListTitle(titleData)

	description := ""
	if desc, ok := media["description"].(string); ok {
		description = stripHTML(desc)
	}

	year := 0
	startDateStr := ""
	if startDate, ok := media["startDate"].(map[string]any); ok {
		if yearFloat, ok := startDate["year"].(float64); ok {
			year = int(yearFloat)
			startDateStr = fmt.Sprintf("%04d", year)
			if monthFloat, ok := startDate["month"].(float64); ok && monthFloat > 0 {
				startDateStr += fmt.Sprintf("-%02d", int(monthFloat))
				if dayFloat, ok := startDate["day"].(float64); ok && dayFloat > 0 {
					startDateStr += fmt.Sprintf("-%02d", int(dayFloat))
				}
			}
		}
	}

	endDateStr := ""
	if endDate, ok := media["endDate"].(map[string]any); ok {
		if yearFloat, ok := endDate["year"].(float64); ok {
			endDateStr = fmt.Sprintf("%04d", int(yearFloat))
			if monthFloat, ok := endDate["month"].(float64); ok && monthFloat > 0 {
				endDateStr += fmt.Sprintf("-%02d", int(monthFloat))
				if dayFloat, ok := endDate["day"].(float64); ok && dayFloat > 0 {
					endDateStr += fmt.Sprintf("-%02d", int(dayFloat))
				}
			}
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
	if coverImage, ok := media["coverImage"].(map[string]any); ok {
		if extraLarge, ok := coverImage["extraLarge"].(string); ok {
			coverURL = extraLarge
		} else if large, ok := coverImage["large"].(string); ok {
			coverURL = large
		}
	}

	// Extract additional metadata
	chapterCount := 0
	if chapters, ok := media["chapters"].(float64); ok {
		chapterCount = int(chapters)
	}

	volumeCount := 0
	if volumes, ok := media["volumes"].(float64); ok {
		volumeCount = int(volumes)
	}

	averageScore := 0.0
	if score, ok := media["averageScore"].(float64); ok {
		averageScore = score
	}

	popularity := 0
	if pop, ok := media["popularity"].(float64); ok {
		popularity = int(pop)
	}

	favorites := 0
	if fav, ok := media["favourites"].(float64); ok {
		favorites = int(fav)
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
		StartDate:        startDateStr,
		EndDate:          endDateStr,
		ChapterCount:     chapterCount,
		VolumeCount:      volumeCount,
		AverageScore:     averageScore,
		Popularity:       popularity,
		Favorites:        favorites,
	}

	// Extract genres as structured genres
	if genres, ok := media["genres"].([]any); ok {
		for _, genre := range genres {
			if genreStr, ok := genre.(string); ok {
				metadata.Genres = append(metadata.Genres, genreStr)
				metadata.Tags = append(metadata.Tags, genreStr) // Also add to tags for compatibility
			}
		}
	}

	// Extract tags
	if tags, ok := media["tags"].([]any); ok {
		for _, tag := range tags {
			if tagMap, ok := tag.(map[string]any); ok {
				if name, ok := tagMap["name"].(string); ok {
					metadata.Tags = append(metadata.Tags, name)
				}
			}
		}
	}

	log.Debugf("Extracted %d tags for AniList media %v: %v", len(metadata.Tags), media["id"], metadata.Tags)

	// Extract synonyms
	if synonyms, ok := media["synonyms"].([]any); ok {
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

	// Extract staff (authors/artists)
	if staff, ok := media["staff"].(map[string]any); ok {
		if edges, ok := staff["edges"].([]any); ok {
			for _, edge := range edges {
				if edgeMap, ok := edge.(map[string]any); ok {
					role := ""
					if roleStr, ok := edgeMap["role"].(string); ok {
						role = roleStr
					}

					if node, ok := edgeMap["node"].(map[string]any); ok {
						if nameData, ok := node["name"].(map[string]any); ok {
							if fullName, ok := nameData["full"].(string); ok {
								authorInfo := AuthorInfo{
									Name: fullName,
									Role: role,
								}

								// Categorize by role
								roleLower := strings.ToLower(role)
								if strings.Contains(roleLower, "author") || strings.Contains(roleLower, "writer") || strings.Contains(roleLower, "story") {
									metadata.Authors = append(metadata.Authors, authorInfo)
									if metadata.Author == "" {
										metadata.Author = fullName // Keep backward compatibility
									}
								} else if strings.Contains(roleLower, "artist") || strings.Contains(roleLower, "illustrator") || strings.Contains(roleLower, "art") {
									metadata.Artists = append(metadata.Artists, authorInfo)
								}
							}
						}
					}
				}
			}
		}
	}

	// Extract main characters
	if characters, ok := media["characters"].(map[string]any); ok {
		if edges, ok := characters["edges"].([]any); ok {
			for _, edge := range edges {
				if edgeMap, ok := edge.(map[string]any); ok {
					if node, ok := edgeMap["node"].(map[string]any); ok {
						if nameData, ok := node["name"].(map[string]any); ok {
							if fullName, ok := nameData["full"].(string); ok {
								metadata.Characters = append(metadata.Characters, fullName)
							}
						}
					}
				}
			}
		}
	}

	// Extract relations
	if relations, ok := media["relations"].(map[string]any); ok {
		if edges, ok := relations["edges"].([]any); ok {
			for _, edge := range edges {
				if edgeMap, ok := edge.(map[string]any); ok {
					relationType := ""
					if typeStr, ok := edgeMap["relationType"].(string); ok {
						relationType = strings.ToUpper(typeStr)
					}

					if node, ok := edgeMap["node"].(map[string]any); ok {
						relationTitle := ""
						if titleData, ok := node["title"].(map[string]any); ok {
							if english, ok := titleData["english"].(string); ok && english != "" {
								relationTitle = english
							} else if romaji, ok := titleData["romaji"].(string); ok && romaji != "" {
								relationTitle = romaji
							} else if native, ok := titleData["native"].(string); ok && native != "" {
								relationTitle = native
							}
						}

						relationID := ""
						if idFloat, ok := node["id"].(float64); ok {
							relationID = fmt.Sprintf("%d", int(idFloat))
						}

						if relationTitle != "" {
							metadata.Relations = append(metadata.Relations, Relation{
								Type:  relationType,
								Title: relationTitle,
								ID:    relationID,
							})
						}
					}
				}
			}
		}
	}

	return metadata
}

// Helper functions
func extractAniListTitle(titleData map[string]any) string {
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
