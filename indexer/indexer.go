package indexer

import (
	"fmt"
	"net/url"
	"os"
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
			return 0, err
		}

		if !exists {
			// Perform manga search
			bestMatch, err := models.GetBestMatchMangadexManga(cleanedName)
			if err != nil {
				bestMatch = nil
			}

			// Fetch cover art URL
			coverArtURL := ""
			cachedImageURL := ""
			if bestMatch != nil {
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
				cachedImageURL = fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)

				err = utils.DownloadImage(cacheDataDirectory, slug, coverArtURL)
				if err != nil {
					log.Errorf("Error downloading file: '%s'", err)
					cachedImageURL = coverArtURL // Fallback cover url
				}
			} else {
				imageFiles := []string{
					"poster.jpg",
					"poster.jpeg",
					"poster.png",
					"thumbnail.jpg",
					"thumbnail.jpeg",
					"thumbnail.png",
				}

				for _, filename := range imageFiles {
					absoluteImageFile := filepath.Join(absolutePath, filename)
					if _, err := os.Stat(absoluteImageFile); err == nil {
						fileExt := filepath.Ext(absoluteImageFile)
						fileExt = fileExt[1:]

						destinationOriginalFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s_original.%s", slug, fileExt))
						destinationCroppedFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s.%s", slug, fileExt))
						utils.CopyFile(absoluteImageFile, destinationOriginalFile)

						err := utils.ProcessImage(absoluteImageFile, destinationCroppedFile)
						if err != nil {
							log.Errorf("Failed to crop image for: '%s' (%s)", slug, err)
						}

						cachedImageURL = fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)
						break
					}
				}
			}

			description := ""
			year := 0000
			originalLanguage := "n/a"
			status := "n/a"
			contentRating := "n/a"

			if bestMatch != nil {
				description = bestMatch.Attributes.Description["en"]
				year = bestMatch.Attributes.Year
				originalLanguage = bestMatch.Attributes.OriginalLanguage
				status = bestMatch.Attributes.Status
				contentRating = bestMatch.Attributes.ContentRating
			}

			newManga := models.Manga{
				Name:             cleanedName,
				Slug:             slug,
				Description:      description,
				Year:             year,
				OriginalLanguage: originalLanguage,
				Status:           status,
				ContentRating:    contentRating,
				CoverArtURL:      cachedImageURL,
				LibraryID:        libraryID,
				Path:             absolutePath,
			}
			mangaID, err := models.CreateManga(newManga)
			if err != nil {
				return 0, err
			}

			if mangaID != 0 {

				length, err := IndexChapters(mangaID, absolutePath)
				if err != nil {
					return 0, err
				}

				log.Infof("Indexed manga: '%s' (%d)", cleanedName, length)

				return mangaID, err
			}
		} else {
			log.Debugf("Skipping: '%s', it has already been indexed.", cleanedName)
		}
	}
	return 0, nil
}

func IndexChapters(id uint, path string) (int, error) {
	// Open the directory
	dir, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer dir.Close()

	// Read the directory entries
	entries, err := dir.Readdir(-1)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		fileWithoutExtension := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		cleanedName := utils.RemovePatterns(fileWithoutExtension)
		chapter := models.Chapter{
			Name:    cleanedName,
			Slug:    utils.Sluggify(cleanedName),
			File:    entry.Name(),
			MangaID: id,
		}
		err := models.CreateChapter(chapter)
		if err != nil {
			log.Errorf("Failed to index chapters for: '%s' (%s)", cleanedName, err.Error())
			return 0, err
		}
	}

	return len(entries), nil
}
