package indexer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2/log"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
)

func IndexManga(absolutePath string, libraryID uint) (uint, error) {
	deepestFolder := filepath.Base(absolutePath)
	cleanedName := utils.RemovePatterns(deepestFolder)
	if cleanedName != "" {
		slug := utils.Sluggify(cleanedName)
		exists, err := models.MangaExists(slug)
		if err != nil {
			log.Errorf("Failed to look for existing manga: %s", cleanedName)
		}

		if !exists {
			log.Infof("Indexing manga: '%s'", cleanedName)

			// Perform manga search
			bestMatch, err := SearchManga(cleanedName)
			if err != nil {
				return 0, err
			}

			// Print the best match details
			if bestMatch != nil {
				// Fetch cover art URL
				coverArtURL := ""
				for _, rel := range bestMatch.Relationships {
					if rel.Type == "cover_art" {
						coverArtURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s",
							bestMatch.ID,
							rel.Attributes.FileName)
					}
				}

				u, err := url.Parse(coverArtURL)
				if err != nil {
					log.Errorf("Error parsing URL:", err)
				}

				filename := filepath.Base(u.Path)
				fileExt := filepath.Ext(filename)
				fileExt = fileExt[1:]
				cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)

				err = utils.DownloadFile(cacheDataDirectory, slug, coverArtURL)
				if err != nil {
					fmt.Println("Error downloading file:", err)
					cachedImageURL = coverArtURL // Fallback cover url
				}
				newManga := models.Manga{
					Name:             cleanedName,
					Slug:             slug,
					Description:      bestMatch.Attributes.Description["en"],
					Year:             bestMatch.Attributes.Year,
					OriginalLanguage: bestMatch.Attributes.OriginalLanguage,
					Status:           bestMatch.Attributes.Status,
					ContentRating:    bestMatch.Attributes.ContentRating,
					CoverArtURL:      cachedImageURL, // Assign cover art URL
					LibraryID:        libraryID,
				}
				mangaID, err := models.CreateManga(newManga)
				if err != nil {
					return 0, err
				}

				if mangaID != 0 {
					return mangaID, err
				}
			} else {
				log.Warn("No matching manga found. Falling back to local metadata.")
			}
		} else {
			log.Debugf("Skipping: '%s', it has already been indexed.", cleanedName)
		}
	}
	return 0, nil
}

func IndexChapters(absolutePath string, mangaID uint) error {
	deepestFolder := filepath.Base(absolutePath)
	cleanedName := utils.RemovePatterns(deepestFolder)
	if cleanedName != "" {
		slug := utils.Sluggify(cleanedName)
		exists, err := models.ChapterExists(slug)
		if err != nil {
			return err
		}

		if !exists {
			log.Infof("Indexing chapter: '%s'", cleanedName)

			// Chapter indexing logic here...
		} else {
			log.Debugf("Skipping chapter: '%s', it has already been indexed.", cleanedName)
		}
	}
	return nil
}

// SearchManga searches for manga with the given title and returns the best match
func SearchManga(title string) (*models.MangaDetail, error) {
	baseURL := "https://api.mangadex.org"

	// Encode the manga title for URL
	titleEncoded := url.QueryEscape(title)

	// Construct the URL with encoded query parameters
	url := fmt.Sprintf("%s/manga?title=%s&contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica&contentRating[]=pornographic&includes[]=cover_art", baseURL, titleEncoded)
	// Make the GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	// Decode the JSON response
	var mangaResponse models.MangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mangaResponse); err != nil {
		return nil, err
	}

	// Check result field in response
	if mangaResponse.Result != "ok" {
		return nil, fmt.Errorf("API returned an error: %s", mangaResponse.Result)
	}

	// Find the best match for the original search title
	bestMatch := findBestMatch(mangaResponse.Data, title)

	if bestMatch == nil {
		return nil, nil // No matching manga found
	}

	return bestMatch, nil
}

// findBestMatch finds the best match for the original search title
func findBestMatch(mangaDetail []models.MangaDetail, originalTitle string) *models.MangaDetail {
	var bestMatch *models.MangaDetail
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
