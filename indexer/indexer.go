package indexer

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2/log"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
)

func IndexManga(absolutePath, librarySlug string) (string, error) {
	start := time.Now()
	defer func() {
		utils.LogDuration("IndexManga", start, absolutePath)
	}()

	deepestFolder := filepath.Base(absolutePath)
	cleanedName := utils.RemovePatterns(deepestFolder)
	if cleanedName == "" {
		return "", nil
	}

	slug := utils.Sluggify(cleanedName)
	exists, _ := models.MangaExists(slug)

	if exists {
		log.Debugf("Skipping: '%s', it has already been indexed.", cleanedName)
		return "", nil
	}

	// Perform manga search
	bestMatch, err := models.GetBestMatchMangadexManga(cleanedName)
	if err != nil {
		log.Warnf("No search result found for: '%s', falling back to local metadata.", slug)
	}

	cachedImageURL, err := handleCoverArt(bestMatch, slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to handle cover image for: '%s'", slug)
		return "", err
	}

	newManga := models.Manga{
		Name:             cleanedName,
		Slug:             slug,
		Description:      getDescription(bestMatch),
		Year:             getYear(bestMatch),
		OriginalLanguage: getOriginalLanguage(bestMatch),
		Status:           getStatus(bestMatch),
		ContentRating:    getContentRating(bestMatch),
		CoverArtURL:      cachedImageURL,
		LibrarySlug:      librarySlug,
		Path:             absolutePath,
		Author:           getAuthor(bestMatch),
	}

	if err := models.CreateManga(newManga); err != nil {
		log.Errorf("Failed to create manga: %s (%s)", slug, err.Error())
		return "", err
	}

	length, err := IndexChapters(slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	log.Infof("Indexed manga: '%s' (%d)", cleanedName, length)
	return slug, nil
}

func handleCoverArt(bestMatch *models.MangaDetail, slug, absolutePath string) (string, error) {
	var coverArtURL string

	if bestMatch != nil {
		for _, rel := range bestMatch.Relationships {
			if rel.Type == "cover_art" {
				if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
					if fileName, exists := attributes["fileName"].(string); exists {
						coverArtURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s",
							bestMatch.ID,
							fileName)
					}
				}
				break // Assuming there's only one cover art
			}
		}
	}

	if coverArtURL == "" {
		return handleLocalImages(slug, absolutePath)
	}

	return downloadAndCacheImage(slug, coverArtURL)
}

func handleLocalImages(slug, absolutePath string) (string, error) {
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
			// Determine file extension
			fileExt := filepath.Ext(absoluteImageFile)
			fileExt = fileExt[1:]

			// Define cache paths
			destinationOriginalFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s_original.%s", slug, fileExt))
			destinationCroppedFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s.%s", slug, fileExt))

			// Copy the image file to cache directory
			if err := utils.CopyFile(absoluteImageFile, destinationOriginalFile); err != nil {
				log.Errorf("Failed to copy file: '%s' to '%s' (%s)", absoluteImageFile, destinationOriginalFile, err)
				return "", err
			}

			// Process the image in the cache directory
			if err := utils.ProcessImage(destinationOriginalFile, destinationCroppedFile); err != nil {
				log.Errorf("Failed to crop image for: '%s' (%s)", slug, err)
				return "", err
			}

			// Return URL for the cropped image
			return fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt), nil
		} else {
			log.Debugf("Image file not found: '%s'", absoluteImageFile)
		}
	}

	return "", nil
}

func downloadAndCacheImage(slug, coverArtURL string) (string, error) {
	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL: %s", err)
		return coverArtURL, nil // Fallback to original URL
	}

	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)[1:]
	cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)

	if err := utils.DownloadImage(cacheDataDirectory, slug, coverArtURL); err != nil {
		log.Errorf("Error downloading file: '%s'", err)
		return coverArtURL, nil // Fallback to original URL
	}

	return cachedImageURL, nil
}

func getDescription(bestMatch *models.MangaDetail) string {
	if bestMatch != nil {
		return bestMatch.Attributes.Description["en"]
	}
	return ""
}

func getYear(bestMatch *models.MangaDetail) int {
	if bestMatch != nil {
		return bestMatch.Attributes.Year
	}
	return 0000
}

func getOriginalLanguage(bestMatch *models.MangaDetail) string {
	if bestMatch != nil {
		return bestMatch.Attributes.OriginalLanguage
	}
	return "n/a"
}

func getStatus(bestMatch *models.MangaDetail) string {
	if bestMatch != nil {
		return bestMatch.Attributes.Status
	}
	return "n/a"
}

func getContentRating(bestMatch *models.MangaDetail) string {
	if bestMatch != nil {
		return bestMatch.Attributes.ContentRating
	}
	return "n/a"
}

func getAuthor(bestMatch *models.MangaDetail) string {
	if bestMatch != nil {
		for _, rel := range bestMatch.Relationships {
			if rel.Type == "author" {
				if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
					if name, exists := attributes["name"].(string); exists {
						return name
					}
				}
				break
			}
		}
	}
	return ""
}

func IndexChapters(slug, path string) (int, error) {
	dir, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileWithoutExtension := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		cleanedName := utils.RemovePatterns(fileWithoutExtension)

		containsNumber := false
		for _, r := range cleanedName {
			if unicode.IsDigit(r) {
				containsNumber = true
				break
			}
		}

		// Any "chapter" that doesn't contain a numeric value is skipped.
		if !containsNumber {
			log.Debugf("Chapter index was skipped for: '%s' - '%s' (no numeric value)", slug, cleanedName)
			continue
		}

		chapter := models.Chapter{
			Name:      cleanedName,
			Slug:      utils.Sluggify(cleanedName),
			File:      entry.Name(),
			MangaSlug: slug,
		}
		if err := models.CreateChapter(chapter); err != nil {
			log.Errorf("Failed to index chapters for: '%s' - '%s' (%s)", slug, cleanedName, err.Error())
			return 0, err
		}
	}

	return len(entries), nil
}
