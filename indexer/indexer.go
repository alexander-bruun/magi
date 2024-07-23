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
				log.Warnf("Couldn't process: '%s' (%s)", cleanedName, err.Error())
				return 0, err
			}

			if bestMatch == nil {
				log.Errorf("how did we get here? (%s)", cleanedName)
				return 0, fmt.Errorf("how did we get here? (%s)", cleanedName)
			}

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
		cleanedName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		chapter := models.Chapter{
			Name:    cleanedName,
			Slug:    utils.Sluggify(cleanedName),
			File:    entry.Name(),
			MangaID: id,
		}
		err := models.CreateChapter(chapter)
		if err != nil {
			log.Error(err.Error())
			return 0, err
		}
	}

	return len(entries), nil
}
