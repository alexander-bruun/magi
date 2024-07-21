package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

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
	Title                  map[string]string   `json:"title"`
	AltTitles              []map[string]string `json:"altTitles"`
	Description            map[string]string   `json:"description"`
	IsLocked               bool                `json:"isLocked"`
	Links                  map[string]string   `json:"links"`
	OriginalLanguage       string              `json:"originalLanguage"`
	LastVolume             string              `json:"lastVolume"`
	LastChapter            string              `json:"lastChapter"`
	PublicationDemographic interface{}         `json:"publicationDemographic"`
	Status                 string              `json:"status"`
	Year                   int                 `json:"year"`
	ContentRating          string              `json:"contentRating"`
	Tags                   []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Name        map[string]string `json:"name"`
			Description map[string]string `json:"description"`
			Group       string            `json:"group"`
			Version     int               `json:"version"`
		} `json:"attributes"`
		Relationships []interface{} `json:"relationships"`
	} `json:"tags"`
	State                          string   `json:"state"`
	ChapterNumbersResetOnNewVolume bool     `json:"chapterNumbersResetOnNewVolume"`
	CreatedAt                      string   `json:"createdAt"`
	UpdatedAt                      string   `json:"updatedAt"`
	Version                        int      `json:"version"`
	AvailableTranslatedLanguages   []string `json:"availableTranslatedLanguages"`
	LatestUploadedChapter          string   `json:"latestUploadedChapter"`
}

// Relationship represents the relationship details in MangaDetail
type Relationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		FileName string `json:"fileName"`
	} `json:"attributes"`
}

var baseURL = "https://api.mangadex.org"

func GetMangadexManga(id string) (*MangaDetail, error) {
	url := fmt.Sprintf("%s/manga/%s?includes[]=cover_art", baseURL, id)
	log.Info(url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var mangaResponse SingleMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mangaResponse); err != nil {
		return nil, err
	}

	if mangaResponse.Result != "ok" {
		return nil, fmt.Errorf("API returned an error: %s", mangaResponse.Result)
	}

	return &mangaResponse.Data, nil
}

// GetMangadexMangas searches for manga based on title and finds all matches
func GetMangadexMangas(title string) (*ListMangaResponse, error) {
	titleEncoded := url.QueryEscape(title)

	url := fmt.Sprintf("%s/manga?title=%s&limit=25&contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica&contentRating[]=pornographic&includes[]=cover_art", baseURL, titleEncoded)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var mangaResponse ListMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mangaResponse); err != nil {
		return nil, err
	}

	if mangaResponse.Result != "ok" {
		return nil, fmt.Errorf("API returned an error: %s", mangaResponse.Result)
	}

	return &mangaResponse, nil
}

// FindManga searches for manga based on the title and returns the best match
func GetBestMatchMangadexManga(title string) (*MangaDetail, error) {
	mangaResponse, err := GetMangadexMangas(title)
	if err != nil {
		return nil, err
	}

	// Find the best match for the original search title
	bestMatch := findBestMatch(mangaResponse.Data, title)
	if bestMatch == nil {
		return nil, errors.New("failed to find a good match")
	}

	return bestMatch, nil
}

// findBestMatch finds the best match for the original search title
func findBestMatch(mangaDetail []MangaDetail, originalTitle string) *MangaDetail {
	var bestMatch *MangaDetail
	var highestScore float64

	// Convert the original title to lowercase for case-insensitive comparison
	originalTitleLower := strings.ToLower(originalTitle)

	for _, manga := range mangaDetail {
		var mangaTitle string

		// First, try to find a suitable English title from AltTitles
		var foundEnglishTitle bool
		for _, altTitleMap := range manga.Attributes.AltTitles {
			if title, ok := altTitleMap["en"]; ok && title != "" {
				mangaTitle = title
				foundEnglishTitle = true
				break
			}
		}

		// If no suitable English title found in AltTitles, try the main English title
		if !foundEnglishTitle {
			mangaTitle = manga.Attributes.Title["en"]
		}

		// If still no English title found, try to find a suitable Japanese title from Attributes
		if mangaTitle == "" {
			if title, ok := manga.Attributes.Title["ja"]; ok && title != "" {
				mangaTitle = title
			}
		}

		// If still no Japanese title found in Attributes, try AltTitles for Japanese
		if mangaTitle == "" {
			for _, altTitleMap := range manga.Attributes.AltTitles {
				if title, ok := altTitleMap["ja"]; ok && title != "" {
					mangaTitle = title
					break
				}
			}
		}

		// If no suitable title found, continue to the next manga
		if mangaTitle == "" {
			continue
		}

		// Calculate similarity using bigram comparison
		similarityScore := utils.CompareStrings(originalTitleLower, strings.ToLower(mangaTitle))
		if similarityScore > highestScore {
			highestScore = similarityScore
			bestMatch = &manga
		}
	}

	return bestMatch
}
