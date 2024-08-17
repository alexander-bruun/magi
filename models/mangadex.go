package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
)

// API Base URL
const baseURL = "https://api.mangadex.org"

// SingleMangaResponse represents the JSON response for a single manga
type SingleMangaResponse struct {
	Result   string      `json:"result"`
	Response string      `json:"response"`
	Data     MangaDetail `json:"data"`
}

// ListMangaResponse represents the JSON response for a list of mangas
type ListMangaResponse struct {
	Result   string        `json:"result"`
	Response string        `json:"response"`
	Data     []MangaDetail `json:"data"`
	Limit    int           `json:"limit,omitempty"`
	Offset   int           `json:"offset,omitempty"`
	Total    int           `json:"total,omitempty"`
}

// MangaDetail represents details of a manga item in the "data" array of MangaResponse
type MangaDetail struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Attributes    MangaAttributes `json:"attributes"`
	Relationships []Relationship  `json:"relationships"`
}

// MangaAttributes represents the attributes of a manga in MangaDetail
type MangaAttributes struct {
	Title                          map[string]string   `json:"title"`
	AltTitles                      []map[string]string `json:"altTitles"`
	Description                    map[string]string   `json:"description"`
	IsLocked                       bool                `json:"isLocked"`
	Links                          map[string]string   `json:"links"`
	OriginalLanguage               string              `json:"originalLanguage"`
	LastVolume                     string              `json:"lastVolume"`
	LastChapter                    string              `json:"lastChapter"`
	PublicationDemographic         interface{}         `json:"publicationDemographic"`
	Status                         string              `json:"status"`
	Year                           int                 `json:"year"`
	ContentRating                  string              `json:"contentRating"`
	Tags                           []Tag               `json:"tags"`
	State                          string              `json:"state"`
	ChapterNumbersResetOnNewVolume bool                `json:"chapterNumbersResetOnNewVolume"`
	CreatedAt                      time.Time           `json:"createdAt"`
	UpdatedAt                      time.Time           `json:"updatedAt"`
	Version                        int                 `json:"version"`
	AvailableTranslatedLanguages   []string            `json:"availableTranslatedLanguages"`
	LatestUploadedChapter          string              `json:"latestUploadedChapter"`
}

// Tag represents a tag in MangaAttributes
type Tag struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name        map[string]string `json:"name"`
		Description map[string]string `json:"description"`
		Group       string            `json:"group"`
		Version     int               `json:"version"`
	} `json:"attributes"`
	Relationships []interface{} `json:"relationships"`
}

// Relationship represents the relationship details in MangaDetail
type Relationship struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Attributes interface{} `json:"attributes"` // General type for flexibility
}

// GetMangadexManga fetches manga details by ID from the MangaDex API
func GetMangadexManga(id string) (*MangaDetail, error) {
	url := fmt.Sprintf("%s/manga/%s?includes[]=cover_art", baseURL, id)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manga details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var mangaResponse SingleMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mangaResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mangaResponse.Result != "ok" {
		return nil, fmt.Errorf("API returned an error: %s", mangaResponse.Result)
	}

	return &mangaResponse.Data, nil
}

// GetMangadexMangas searches for mangas based on the title and returns a list of matches
func GetMangadexMangas(title string) (*ListMangaResponse, error) {
	titleEncoded := url.QueryEscape(title)
	url := fmt.Sprintf("%s/manga?title=%s&limit=50&contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica&contentRating[]=pornographic&includes[]=cover_art", baseURL, titleEncoded)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to search for mangas: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var mangaResponse ListMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mangaResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mangaResponse.Result != "ok" {
		return nil, fmt.Errorf("API returned an error: %s", mangaResponse.Result)
	}

	if len(mangaResponse.Data) == 0 {
		return nil, errors.New("no search results found")
	}

	return &mangaResponse, nil
}

// GetBestMatchMangadexManga returns the best match manga based on the title
func GetBestMatchMangadexManga(title string) (*MangaDetail, error) {
	mangaResponse, err := GetMangadexMangas(title)
	if err != nil {
		return nil, err
	}

	bestMatch, err := findBestMatch(mangaResponse.Data, title)
	if err != nil {
		return nil, err
	}

	return bestMatch, nil
}

// findBestMatch identifies the manga with the highest similarity to the original title
func findBestMatch(mangas []MangaDetail, originalTitle string) (*MangaDetail, error) {
	originalTitleLower := strings.ToLower(originalTitle)
	var bestMatch *MangaDetail
	highestScore := 0.0

	for _, manga := range mangas {
		mangaTitle := extractTitle(manga.Attributes)
		if mangaTitle == "" {
			continue
		}

		similarityScore := utils.CompareStrings(originalTitleLower, strings.ToLower(mangaTitle))
		if similarityScore > highestScore {
			highestScore = similarityScore
			bestMatch = &manga
		}
	}

	if bestMatch == nil {
		return nil, errors.New("no suitable match found")
	}

	return bestMatch, nil
}

// extractTitle determines the best title to use for similarity comparison
func extractTitle(attributes MangaAttributes) string {
	if title, ok := attributes.Title["en"]; ok && title != "" {
		return title
	}

	for _, altTitleMap := range attributes.AltTitles {
		if title, ok := altTitleMap["en"]; ok && title != "" {
			return title
		}
	}

	if title, ok := attributes.Title["ja"]; ok && title != "" {
		return title
	}

	for _, altTitleMap := range attributes.AltTitles {
		if title, ok := altTitleMap["ja"]; ok && title != "" {
			return title
		}
	}

	return ""
}
