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

const (
	localServerBaseURL = "http://localhost:3000/api/images"
)

func IndexManga(absolutePath, librarySlug string) (string, error) {
	defer utils.LogDuration("IndexManga", time.Now(), absolutePath)

	cleanedName := utils.RemovePatterns(filepath.Base(absolutePath))
	if cleanedName == "" {
		return "", nil
	}

	slug := utils.Sluggify(cleanedName)
	if exists, _ := models.MangaExists(slug); exists {
		log.Debugf("Skipping: '%s', it has already been indexed", cleanedName)
		return "", nil
	}

	bestMatch, err := models.GetBestMatchMangadexManga(cleanedName)
	if err != nil {
		log.Warnf("No search result found for: '%s', falling back to local metadata", slug)
	}

	cachedImageURL, err := handleCoverArt(bestMatch, slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to handle cover image for: '%s'", slug)
		return "", err
	}

	newManga := createMangaFromMatch(bestMatch, cleanedName, slug, librarySlug, absolutePath, cachedImageURL)

	if err := models.CreateManga(newManga); err != nil {
		log.Errorf("Failed to create manga: %s (%s)", slug, err.Error())
		return "", err
	}

	chapterCount, err := IndexChapters(slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	log.Infof("Indexed manga: '%s' (%d chapters)", cleanedName, chapterCount)
	return slug, nil
}

func createMangaFromMatch(match *models.MangaDetail, name, slug, librarySlug, path, coverURL string) models.Manga {
	return models.Manga{
		Name:             name,
		Slug:             slug,
		Description:      getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.Description["en"] }),
		Year:             getIntAttribute(match, func(m *models.MangaDetail) int { return m.Attributes.Year }),
		OriginalLanguage: getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.OriginalLanguage }),
		Status:           getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.Status }),
		ContentRating:    getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.ContentRating }),
		CoverArtURL:      coverURL,
		LibrarySlug:      librarySlug,
		Path:             path,
		Author:           getAuthor(match),
	}
}

func handleCoverArt(bestMatch *models.MangaDetail, slug, absolutePath string) (string, error) {
	coverArtURL := getCoverArtURL(bestMatch)
	if coverArtURL == "" {
		return handleLocalImages(slug, absolutePath)
	}
	return downloadAndCacheImage(slug, coverArtURL)
}

func getCoverArtURL(match *models.MangaDetail) string {
	if match == nil {
		return ""
	}
	for _, rel := range match.Relationships {
		if rel.Type == "cover_art" {
			if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
				if fileName, exists := attributes["fileName"].(string); exists {
					return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", match.ID, fileName)
				}
			}
			break
		}
	}
	return ""
}

func handleLocalImages(slug, absolutePath string) (string, error) {
	imageFiles := []string{"poster.jpg", "poster.jpeg", "poster.png", "thumbnail.jpg", "thumbnail.jpeg", "thumbnail.png"}

	for _, filename := range imageFiles {
		imagePath := filepath.Join(absolutePath, filename)
		if _, err := os.Stat(imagePath); err == nil {
			return processLocalImage(slug, imagePath)
		}
	}

	return "", nil
}

func processLocalImage(slug, imagePath string) (string, error) {
	fileExt := filepath.Ext(imagePath)[1:]
	originalFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s_original.%s", slug, fileExt))
	croppedFile := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s.%s", slug, fileExt))

	if err := utils.CopyFile(imagePath, originalFile); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	if err := utils.ProcessImage(originalFile, croppedFile); err != nil {
		return "", fmt.Errorf("failed to crop image: %w", err)
	}

	return fmt.Sprintf("%s/%s.%s", localServerBaseURL, slug, fileExt), nil
}

func downloadAndCacheImage(slug, coverArtURL string) (string, error) {
	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL: %s", err)
		return coverArtURL, nil
	}

	fileExt := filepath.Ext(u.Path)[1:]
	cachedImageURL := fmt.Sprintf("%s/%s.%s", localServerBaseURL, slug, fileExt)

	if err := utils.DownloadImage(cacheDataDirectory, slug, coverArtURL); err != nil {
		log.Errorf("Error downloading file: %s", err)
		return coverArtURL, nil
	}

	return cachedImageURL, nil
}

func getStringAttribute(match *models.MangaDetail, getter func(*models.MangaDetail) string) string {
	if match != nil {
		return getter(match)
	}
	return "n/a"
}

func getIntAttribute(match *models.MangaDetail, getter func(*models.MangaDetail) int) int {
	if match != nil {
		return getter(match)
	}
	return 0
}

func getAuthor(match *models.MangaDetail) string {
	if match == nil {
		return ""
	}
	for _, rel := range match.Relationships {
		if rel.Type == "author" {
			if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
				if name, exists := attributes["name"].(string); exists {
					return name
				}
			}
			break
		}
	}
	return ""
}

func IndexChapters(slug, path string) (int, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}

	var chapterCount int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		cleanedName := utils.RemovePatterns(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		if !containsNumber(cleanedName) {
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
			return 0, fmt.Errorf("failed to index chapter '%s' for manga '%s': %w", cleanedName, slug, err)
		}
		chapterCount++
	}

	return chapterCount, nil
}

func containsNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
