package indexer

import (
	"fmt"
	"io/fs"
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
	localServerBaseURL = "/api/images"
)

// IndexManga inspects a manga directory, syncing metadata and chapters with the database.
func IndexManga(absolutePath, librarySlug string) (string, error) {
	defer utils.LogDuration("IndexManga", time.Now(), absolutePath)

	cleanedName := utils.RemovePatterns(filepath.Base(absolutePath))
	if cleanedName == "" {
		return "", nil
	}

	slug := utils.Sluggify(cleanedName)

	// If manga already exists, avoid external API calls and heavy image work.
	// Only update the path if needed and index any new chapters.
	existingManga, err := models.GetManga(slug)
	if err != nil {
		log.Errorf("Failed to lookup manga '%s': %s", slug, err)
	}

	if existingManga != nil {
		// Fast path 1: use stored file_count on the Manga. If the number of
		// candidate files (files that look like chapters) matches the stored
		// FileCount, assume no changes and skip.
		if absolutePath != "" {
			// Count files (fast): we only need to count entries that look
			// like chapters (contain a number after cleaning). This is a
			// fast directory read that avoids creating objects.
			var candidateCount int
			_ = filepath.WalkDir(absolutePath, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				name := d.Name()
				cleanedName := utils.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))
				if containsNumber(cleanedName) {
					candidateCount++
				}
				return nil
			})
			if candidateCount == existingManga.FileCount {
				return slug, nil
			}
		}

		// Only update path if changed
		if existingManga.Path == "" || existingManga.Path != absolutePath {
			existingManga.Path = absolutePath
			if err := models.UpdateManga(existingManga); err != nil {
				log.Errorf("Failed to update manga path for '%s': %s", slug, err)
			}
		}

		// Index chapters recursively; returns added and deleted counts.
		added, deleted, err := IndexChapters(slug, absolutePath)
		if err != nil {
			log.Errorf("Failed to index chapters for existing manga '%s': %s", slug, err)
			return slug, err
		}
		if added > 0 || deleted > 0 {
			// Update manga updated_at to mark the index time
			if err := models.UpdateManga(existingManga); err != nil {
				log.Errorf("Failed to update manga timestamp for '%s': %s", slug, err)
			}
			log.Infof("Indexed manga: '%s' (added: %d deleted: %d)", cleanedName, added, deleted)
		}
		return slug, nil
	}

	// Manga does not exist yet — fetch metadata, create it and index chapters
	bestMatch, _ := models.GetBestMatchMangadexManga(cleanedName)
	if bestMatch == nil {
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

	// Persist tags from Mangadex response (if any)
	if bestMatch != nil && len(bestMatch.Attributes.Tags) > 0 {
		var tags []string
		for _, t := range bestMatch.Attributes.Tags {
			// prefer English name if available
			if name, ok := t.Attributes.Name["en"]; ok && name != "" {
				tags = append(tags, name)
			} else {
				// fallback: pick the first available name
				for _, v := range t.Attributes.Name {
					if v != "" {
						tags = append(tags, v)
						break
					}
				}
			}
		}
		if len(tags) > 0 {
			if err := models.SetTagsForManga(slug, tags); err != nil {
				log.Errorf("Failed to set tags for manga '%s': %s", slug, err)
			}
		}
	}

	added, deleted, err := IndexChapters(slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	if added > 0 || deleted > 0 {
		log.Infof("Indexed manga: '%s' (added=%d deleted=%d)", cleanedName, added, deleted)
	}
	return slug, nil
}

func createMangaFromMatch(match *models.MangaDetail, name, slug, librarySlug, path, coverURL string) models.Manga {
	// derive type from original language
	derivedType := models.DetermineMangaType(match)

	return models.Manga{
		Name:             name,
		Slug:             slug,
		Description:      getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.Description["en"] }),
		Year:             getIntAttribute(match, func(m *models.MangaDetail) int { return m.Attributes.Year }),
		OriginalLanguage: getStringAttribute(match, func(m *models.MangaDetail) string { return m.Attributes.OriginalLanguage }),
		Type:             derivedType,
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

// IndexChapters reconciles chapter files on disk with the stored chapter records.
func IndexChapters(slug, path string) (int, int, error) {
	var addedCount int
	var deletedCount int

	// Load existing chapters once to avoid querying the DB per file.
	existing, err := models.GetChapters(slug)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to load existing chapters for manga '%s': %w", slug, err)
	}
	existingMap := make(map[string]models.Chapter, len(existing))
	for _, c := range existing {
		existingMap[c.Slug] = c
	}

	// Build map of files currently present (slug -> relPath). This is a
	// full scan but cheaper than many DB ops; we only do it when file_count
	// mismatch was observed.
	type presentInfo struct {
		Rel  string
		Name string
	}
	presentMap := make(map[string]presentInfo)
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		cleanedName := utils.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))
		if !containsNumber(cleanedName) {
			return nil
		}
		chapterSlug := utils.Sluggify(cleanedName)
		relPath, err := filepath.Rel(path, p)
		if err != nil {
			relPath = name
		}
		presentMap[chapterSlug] = presentInfo{Rel: filepath.ToSlash(relPath), Name: cleanedName}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	// Create missing chapters
	for slugKey, info := range presentMap {
		if _, ok := existingMap[slugKey]; !ok {
			// not present in DB -> create with pretty name
			chapter := models.Chapter{
				Name:      info.Name,
				Slug:      slugKey,
				File:      info.Rel,
				MangaSlug: slug,
			}
			if err := models.CreateChapter(chapter); err != nil {
				return addedCount, deletedCount, fmt.Errorf("failed to create chapter '%s' for manga '%s': %w", info.Name, slug, err)
			}
			addedCount++
		}
	}

	// Delete chapters that are no longer present on disk
	for slugKey := range existingMap {
		if _, ok := presentMap[slugKey]; !ok {
			if err := models.DeleteChapter(slug, slugKey); err != nil {
				return addedCount, deletedCount, fmt.Errorf("failed to delete missing chapter '%s' for manga '%s': %w", slugKey, slug, err)
			}
			deletedCount++
		}
	}

	// Update manga file count and timestamp
	m, err := models.GetManga(slug)
	if err == nil && m != nil {
		m.FileCount = len(presentMap)
		if err := models.UpdateManga(m); err != nil {
			log.Errorf("Failed to update manga file_count for '%s': %s", slug, err)
		}
	}

	return addedCount, deletedCount, nil
}

func containsNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
