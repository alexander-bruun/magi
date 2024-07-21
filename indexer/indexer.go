package indexer

import (
	"fmt"
	"net/url"
	"path/filepath"

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
			bestMatch, err := models.GetBestMatchMangadexManga(cleanedName)
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

				err = utils.DownloadImage(cacheDataDirectory, slug, coverArtURL)
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
					CoverArtURL:      cachedImageURL,
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
