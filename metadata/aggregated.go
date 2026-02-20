package metadata

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3/log"
)

// AttributionLink represents a link to the source metadata page
type AttributionLink struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

// AggregatedMediaMetadata combines metadata from multiple providers
type AggregatedMediaMetadata struct {
	// Primary fields (consolidated from all providers)
	Title             string   `json:"title"`
	AlternativeTitles []string `json:"alternative_titles"`
	Description       string   `json:"description"`
	Year              int      `json:"year"`
	OriginalLanguage  string   `json:"original_language"`
	Status            string   `json:"status"` // ongoing, completed, hiatus, cancelled
	ContentRating     string   `json:"content_rating"`
	CoverArtURLs      []string `json:"cover_art_urls"`
	Tags              []string `json:"tags"`
	Type              string   `json:"type"`
	Author            string   `json:"author"` // Backward compatibility

	// Additional rich metadata fields
	Authors       []AuthorInfo `json:"authors"`
	Artists       []AuthorInfo `json:"artists"`
	StartDate     string       `json:"start_date"`
	EndDate       string       `json:"end_date"`
	ChapterCount  int          `json:"chapter_count"`
	VolumeCount   int          `json:"volume_count"`
	AverageScore  float64      `json:"average_score"`
	Popularity    int          `json:"popularity"`
	Favorites     int          `json:"favorites"`
	Demographic   string       `json:"demographic"`
	Publisher     string       `json:"publisher"`
	Magazine      string       `json:"magazine"`
	Serialization string       `json:"serialization"`
	Genres        []string     `json:"genres"`
	Characters    []string     `json:"characters"`
	Relations     []Relation   `json:"relations"`

	// Attribution and source data
	AttributionLinks []AttributionLink         `json:"attribution_links"`
	ProviderData     map[string]*MediaMetadata `json:"provider_data"`
}

// AggregateMetadata combines metadata from multiple providers into a single aggregated result
func AggregateMetadata(title string, providerResults map[string]*MediaMetadata) *AggregatedMediaMetadata {
	aggregated := &AggregatedMediaMetadata{
		Title:             title,
		AlternativeTitles: []string{},
		CoverArtURLs:      []string{},
		Tags:              []string{},
		Genres:            []string{},
		Authors:           []AuthorInfo{},
		Artists:           []AuthorInfo{},
		Characters:        []string{},
		Relations:         []Relation{},
		AttributionLinks:  []AttributionLink{},
		ProviderData:      providerResults,
	}

	// Maps to track unique values
	titleMap := make(map[string]bool)
	altTitleMap := make(map[string]bool)
	tagMap := make(map[string]bool)
	genreMap := make(map[string]bool)
	coverMap := make(map[string]bool)
	authorMap := make(map[string]bool)
	artistMap := make(map[string]bool)
	characterMap := make(map[string]bool)

	for providerName, meta := range providerResults {
		if meta == nil {
			continue
		}

		// Add primary title if not already present
		if meta.Title != "" && !titleMap[meta.Title] {
			titleMap[meta.Title] = true
			if aggregated.Title == "" {
				aggregated.Title = meta.Title
			}
		}

		// Add alternative titles
		for _, altTitle := range meta.AlternativeTitles {
			if altTitle != "" && !altTitleMap[altTitle] {
				altTitleMap[altTitle] = true
				aggregated.AlternativeTitles = append(aggregated.AlternativeTitles, altTitle)
			}
		}

		// Use description from first provider that has one
		if aggregated.Description == "" && meta.Description != "" {
			aggregated.Description = meta.Description
		}

		// Use year from first provider that has one
		if aggregated.Year == 0 && meta.Year > 0 {
			aggregated.Year = meta.Year
		}

		// Use original language from first provider that has one
		if aggregated.OriginalLanguage == "" && meta.OriginalLanguage != "" {
			aggregated.OriginalLanguage = meta.OriginalLanguage
		}

		// Use status from first provider that has one
		if aggregated.Status == "" && meta.Status != "" {
			aggregated.Status = meta.Status
		}

		// Use content rating from first provider that has one
		if aggregated.ContentRating == "" && meta.ContentRating != "" {
			aggregated.ContentRating = meta.ContentRating
		}

		// Add cover art URLs
		if meta.CoverArtURL != "" && !coverMap[meta.CoverArtURL] {
			coverMap[meta.CoverArtURL] = true
			aggregated.CoverArtURLs = append(aggregated.CoverArtURLs, meta.CoverArtURL)
		}

		// Add tags
		for _, tag := range meta.Tags {
			if tag != "" && !tagMap[tag] {
				tagMap[tag] = true
				aggregated.Tags = append(aggregated.Tags, tag)
			}
		}

		// Add genres
		for _, genre := range meta.Genres {
			if genre != "" && !genreMap[genre] {
				genreMap[genre] = true
				aggregated.Genres = append(aggregated.Genres, genre)
			}
		}

		// Use type from first provider that has one
		if aggregated.Type == "" && meta.Type != "" {
			aggregated.Type = meta.Type
		}

		// Use author from first provider that has one (backward compatibility)
		if aggregated.Author == "" && meta.Author != "" {
			aggregated.Author = meta.Author
		}

		// Add authors
		for _, author := range meta.Authors {
			key := author.Name + "|" + author.Role
			if !authorMap[key] {
				authorMap[key] = true
				aggregated.Authors = append(aggregated.Authors, author)
			}
		}

		// Add artists
		for _, artist := range meta.Artists {
			key := artist.Name + "|" + artist.Role
			if !artistMap[key] {
				artistMap[key] = true
				aggregated.Artists = append(aggregated.Artists, artist)
			}
		}

		// Add characters
		for _, character := range meta.Characters {
			if character != "" && !characterMap[character] {
				characterMap[character] = true
				aggregated.Characters = append(aggregated.Characters, character)
			}
		}

		// Use start/end dates from first provider that has them
		if aggregated.StartDate == "" && meta.StartDate != "" {
			aggregated.StartDate = meta.StartDate
		}
		if aggregated.EndDate == "" && meta.EndDate != "" {
			aggregated.EndDate = meta.EndDate
		}

		// Use chapter/volume counts from first provider that has them
		if aggregated.ChapterCount == 0 && meta.ChapterCount > 0 {
			aggregated.ChapterCount = meta.ChapterCount
		}
		if aggregated.VolumeCount == 0 && meta.VolumeCount > 0 {
			aggregated.VolumeCount = meta.VolumeCount
		}

		// Use average score from first provider that has it
		if aggregated.AverageScore == 0 && meta.AverageScore > 0 {
			aggregated.AverageScore = meta.AverageScore
		}

		// Use popularity/favorites from first provider that has them
		if aggregated.Popularity == 0 && meta.Popularity > 0 {
			aggregated.Popularity = meta.Popularity
		}
		if aggregated.Favorites == 0 && meta.Favorites > 0 {
			aggregated.Favorites = meta.Favorites
		}

		// Use demographic from first provider that has it
		if aggregated.Demographic == "" && meta.Demographic != "" {
			aggregated.Demographic = meta.Demographic
		}

		// Use magazine/serialization from first provider that has it
		if aggregated.Magazine == "" && meta.Magazine != "" {
			aggregated.Magazine = meta.Magazine
		}

		// Add relations
		for _, relation := range meta.Relations {
			// Simple deduplication by title
			found := false
			for _, existing := range aggregated.Relations {
				if existing.Title == relation.Title {
					found = true
					break
				}
			}
			if !found {
				aggregated.Relations = append(aggregated.Relations, relation)
			}
		}

		// Add attribution link with proper URL
		var url string
		switch providerName {
		case "mangadex":
			url = fmt.Sprintf("https://mangadex.org/title/%s", meta.ExternalID)
		case "anilist":
			url = fmt.Sprintf("https://anilist.co/manga/%s", meta.ExternalID)
		case "kitsu":
			// Kitsu ExternalID format: "type:id" (e.g., "anime:46231" or "manga:46231")
			parts := strings.Split(meta.ExternalID, ":")
			if len(parts) == 2 {
				mediaType := parts[0]
				id := parts[1]
				url = fmt.Sprintf("https://kitsu.io/%s/%s", mediaType, id)
			} else {
				// Fallback for old format
				url = fmt.Sprintf("https://kitsu.io/manga/%s", meta.ExternalID)
			}
		case "jikan":
			// Jikan ExternalID format: "type:id" (e.g., "anime:52299" or "manga:121496")
			parts := strings.Split(meta.ExternalID, ":")
			if len(parts) == 2 {
				mediaType := parts[0]
				id := parts[1]
				url = fmt.Sprintf("https://myanimelist.net/%s/%s", mediaType, id)
			} else {
				// Fallback for old format
				url = fmt.Sprintf("https://myanimelist.net/manga/%s", meta.ExternalID)
			}
		case "mangaupdates":
			url = meta.ExternalID
		default:
			url = fmt.Sprintf("https://%s.example.com/title/%s", providerName, meta.ExternalID)
		}

		attribution := AttributionLink{
			Provider: providerName,
			URL:      url,
			Title:    meta.Title,
		}
		log.Debugf("AggregateMetadata: Adding attribution link for %s: Provider=%s, URL=%s, Title=%s", providerName, attribution.Provider, attribution.URL, attribution.Title)
		aggregated.AttributionLinks = append(aggregated.AttributionLinks, attribution)
	}

	return aggregated
}

// QueryAllProviders searches all available metadata providers for a title and returns aggregated results
func QueryAllProviders(title string) (*AggregatedMediaMetadata, error) {
	providerNames := ListProviders()
	results := make(map[string]*MediaMetadata)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Query each provider in parallel
	for _, providerName := range providerNames {
		wg.Add(1)
		go func(pName string) {
			defer wg.Done()

			provider, err := GetProvider(pName, "")
			if err != nil {
				// Skip providers that can't be initialized
				log.Debugf("QueryAllProviders: Failed to get provider %s: %v", pName, err)
				return
			}

			log.Debugf("QueryAllProviders: Querying provider %s for title '%s'", pName, title)
			meta, err := provider.FindBestMatch(title)
			if err != nil {
				// Skip providers that fail
				log.Debugf("QueryAllProviders: Provider %s failed for '%s': %v", pName, title, err)
				return
			}

			if meta != nil {
				log.Debugf("QueryAllProviders: Provider %s found result: %s (ID: %s)", pName, meta.Title, meta.ExternalID)
				mu.Lock()
				results[pName] = meta
				mu.Unlock()
			} else {
				log.Debugf("QueryAllProviders: Provider %s returned nil for '%s'", pName, title)
			}
		}(providerName)
	}

	// Wait for all providers to complete
	wg.Wait()

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	return AggregateMetadata(title, results), nil
}
