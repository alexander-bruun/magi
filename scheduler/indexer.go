package scheduler

import (
	"archive/zip"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3/log"

	"github.com/alexander-bruun/magi/filestore"
	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/utils/text"
)

type presentInfo struct {
	Rel  string
	Name string
}

func init() {
	IndexMediaFunc = IndexMedia
}

// isSafeZipPath returns true if the given zip entry path does not contain traversal elements or an absolute path.
func isSafeZipPath(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	// Also ensure cleaned path does not start with "../" or "/" (in case)
	if strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return false
	}
	return true
}

const (
	localServerBaseURL = "/api/posters"
)

type EPUBMetadata struct {
	Author        string
	Description   string
	Year          int
	Language      string
	Status        string
	ContentRating string
	CoverArtURL   string
	Tags          []string
}

// OPF represents the structure of an EPUB's OPF file
type OPF struct {
	Metadata struct {
		Meta []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
		Subject []string `xml:"dc:subject"`
	} `xml:"metadata"`
	Manifest struct {
		Item []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
}

// IndexMedia inspects a media directory or file (.cbz/.cbr), syncing metadata and chapters with the database.
func IndexMedia(absolutePath, librarySlug string, dataBackend filestore.DataBackend) (string, error) {
	// Check if this is a file (single-chapter media)
	fileInfo, err := os.Stat(absolutePath)
	pathExists := err == nil
	isSingleFile := pathExists && !fileInfo.IsDir()

	// For single files, use the filename without extension as the media name
	baseName := filepath.Base(absolutePath)
	if isSingleFile {
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	cleanedName := text.RemovePatterns(baseName)
	if cleanedName == "" {
		return "", nil
	}

	slug := text.Sluggify(cleanedName)

	// Check if media already exists
	existingMedia, err := models.GetMediaUnfiltered(slug)
	if err != nil {
		log.Errorf("Failed to lookup media '%s': %s", slug, err)
	}

	// If path doesn't exist but media exists, handle as missing path
	if !pathExists && existingMedia != nil {
		log.Warnf("Media path '%s' does not exist, cleaning up existing media '%s'", absolutePath, slug)
		return handleMissingMediaPath(existingMedia, librarySlug)
	}

	// If path doesn't exist and media doesn't exist, nothing to do
	if !pathExists {
		return "", fmt.Errorf("failed to stat path '%s': %w", absolutePath, err)
	}

	// Get metadata provider if configured
	config, err := models.GetAppConfig()
	var provider metadata.Provider
	if err == nil {
		// Get library to check for library-specific metadata provider
		library, libErr := models.GetLibrary(librarySlug)
		if libErr != nil {
			log.Warnf("Failed to get library '%s': %v", librarySlug, libErr)
		} else {
			// Check if the absolutePath is within one of the library's folders
			pathInLibrary := false
			for _, folder := range library.Folders {
				if strings.HasPrefix(absolutePath, folder) {
					pathInLibrary = true
					break
				}
			}
			if !pathInLibrary {
				return "", fmt.Errorf("path '%s' is not within library '%s' folders", absolutePath, librarySlug)
			}

			provider, err = metadata.GetProviderForLibrary(library.MetadataProvider, &config)
			if err != nil {
				log.Debugf("No metadata provider configured for library '%s'", librarySlug)
			}
		}
	}

	// If media already exists, avoid external API calls and heavy image work.
	// Only update the path if needed and index any new chapters.
	// Use GetMediaUnfiltered to check globally
	existingMedia, err = models.GetMediaUnfiltered(slug)
	if err != nil {
		log.Errorf("Failed to lookup media '%s': %s", slug, err)
	}

	if existingMedia != nil {
		// Count chapters in the new path
		var newCandidateCount int
		if isSingleFile {
			newCandidateCount = 1
		} else {
			_ = filepath.WalkDir(absolutePath, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				name := d.Name()
				cleanedName := text.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))
				if containsNumber(cleanedName) {
					newCandidateCount++
				}
				return nil
			})
		}
	}

	if existingMedia != nil {
		return handleExistingMedia(existingMedia, absolutePath, librarySlug, cleanedName, slug, provider, fileInfo, isSingleFile)
	} else {
		return handleNewMedia(cleanedName, slug, librarySlug, absolutePath, provider, isSingleFile, dataBackend)
	}
}

// handleMissingMediaPath handles the case where a media's path no longer exists
func handleMissingMediaPath(existingMedia *models.Media, librarySlug string) (string, error) {
	slug := existingMedia.Slug
	cleanedName := existingMedia.Name

	// Delete all chapters for this media in this library
	existingChapters, err := models.GetChapters(slug)
	if err != nil {
		return slug, fmt.Errorf("failed to load existing chapters for media '%s': %w", slug, err)
	}

	// Filter to only chapters from this library
	var libraryChapters []models.Chapter
	for _, c := range existingChapters {
		if c.LibrarySlug == librarySlug {
			libraryChapters = append(libraryChapters, c)
		}
	}

	if len(libraryChapters) > 0 {
		tx, txErr := models.BeginTx()
		if txErr != nil {
			return slug, fmt.Errorf("failed to begin transaction: %w", txErr)
		}
		defer tx.Rollback()

		deletedCount := 0
		for _, chapter := range libraryChapters {
			if delErr := models.DeleteChapterTx(tx, slug, chapter.Slug, chapter.LibrarySlug); delErr != nil {
				log.Errorf("Failed to delete chapter '%s' for missing media '%s': %v", chapter.Slug, slug, delErr)
			} else {
				deletedCount++
			}
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return slug, fmt.Errorf("failed to commit transaction: %w", commitErr)
		}

		log.Infof("Deleted %d chapters for missing media '%s' in library '%s'", deletedCount, slug, librarySlug)
		BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Deleted %d chapters for missing series '%s'", deletedCount, cleanedName))
	}

	// Update FileCount (recalculate based on remaining chapters)
	allChapters, err := models.GetChapters(slug)
	if err != nil {
		log.Errorf("Failed to get remaining chapters for media '%s': %s", slug, err)
	} else {
		existingMedia.FileCount = len(allChapters)
		if err := models.UpdateMedia(existingMedia); err != nil {
			log.Errorf("Failed to update media file count for '%s': %s", slug, err)
		}
	}

	// If no chapters remain globally, delete the media entirely
	if existingMedia.FileCount == 0 {
		log.Infof("Deleting media '%s' as it has no remaining chapters", slug)
		BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Deleted series '%s' - no chapters remaining", cleanedName))
		if err := models.DeleteMedia(slug); err != nil {
			log.Errorf("Failed to delete media '%s': %s", slug, err)
		}
		return slug, nil
	}

	return slug, nil
}

func createMediaFromAggregatedMetadata(aggregatedMeta *metadata.AggregatedMediaMetadata, name, slug, _, _, coverURL string) models.Media {
	media := models.Media{
		Name:        name,
		Slug:        slug,
		CoverArtURL: coverURL,
	}

	if aggregatedMeta != nil {
		// Basic metadata fields
		media.Description = aggregatedMeta.Description
		media.Year = aggregatedMeta.Year
		media.OriginalLanguage = aggregatedMeta.OriginalLanguage
		media.Type = aggregatedMeta.Type
		media.Status = aggregatedMeta.Status
		media.ContentRating = aggregatedMeta.ContentRating
		media.Author = aggregatedMeta.Author // Backward compatibility
		media.Tags = aggregatedMeta.Tags

		// Enhanced metadata fields
		media.Authors = make([]models.AuthorInfo, len(aggregatedMeta.Authors))
		for i, author := range aggregatedMeta.Authors {
			media.Authors[i] = models.AuthorInfo{Name: author.Name, Role: author.Role}
		}
		media.Artists = make([]models.AuthorInfo, len(aggregatedMeta.Artists))
		for i, artist := range aggregatedMeta.Artists {
			media.Artists[i] = models.AuthorInfo{Name: artist.Name, Role: artist.Role}
		}
		media.StartDate = aggregatedMeta.StartDate
		media.EndDate = aggregatedMeta.EndDate
		media.ChapterCount = aggregatedMeta.ChapterCount
		media.VolumeCount = aggregatedMeta.VolumeCount
		media.AverageScore = aggregatedMeta.AverageScore
		media.Popularity = aggregatedMeta.Popularity
		media.Favorites = aggregatedMeta.Favorites
		media.Demographic = aggregatedMeta.Demographic
		media.Publisher = aggregatedMeta.Publisher
		media.Magazine = aggregatedMeta.Magazine
		media.Serialization = aggregatedMeta.Serialization
		media.Genres = aggregatedMeta.Genres
		media.Characters = aggregatedMeta.Characters
		media.AlternativeTitles = aggregatedMeta.AlternativeTitles
		media.AttributionLinks = make([]models.AttributionLink, len(aggregatedMeta.AttributionLinks))
		for i, link := range aggregatedMeta.AttributionLinks {
			media.AttributionLinks[i] = models.AttributionLink{Provider: link.Provider, URL: link.URL, Title: link.Title}
		}
	}

	return media
}

func HandleLocalImages(slug, absolutePath string, dataBackend filestore.DataBackend) (string, error) {
	log.Debugf("Attempting to generate poster from local images for media '%s' at path '%s'", slug, absolutePath)

	// First, check for standalone poster/thumbnail images
	imageFiles := []string{"poster.webp", "poster.jpg", "poster.jpeg", "poster.png", "thumbnail.webp", "thumbnail.jpg", "thumbnail.jpeg", "thumbnail.png", "cover.webp", "cover.jpg", "cover.jpeg", "cover.png"}

	for _, filename := range imageFiles {
		imagePath := filepath.Join(absolutePath, filename)
		if _, err := os.Stat(imagePath); err == nil {
			log.Debugf("Found standalone poster image '%s' for media '%s'", filename, slug)
			return processLocalImage(slug, imagePath, dataBackend)
		}
	}

	log.Debugf("No standalone poster images found for media '%s', checking archives", slug)

	// If no standalone image found, try to extract from archive files
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		log.Debugf("Failed to stat path '%s': %v", absolutePath, err)
		return "", err
	}

	if !fileInfo.IsDir() {
		// This is a file (likely an archive like .cbz, .cbr, .zip, .rar, .epub)
		lowerPath := strings.ToLower(absolutePath)
		if strings.HasSuffix(lowerPath, ".cbz") || strings.HasSuffix(lowerPath, ".cbr") ||
			strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".rar") ||
			strings.HasSuffix(lowerPath, ".epub") {
			log.Debugf("Extracting poster from single archive file '%s' for media '%s'", absolutePath, slug)
			return files.ExtractPosterImage(absolutePath, slug, dataBackend, true)
		} else {
			log.Debugf("Path '%s' is a file but not a supported archive format", absolutePath)
		}
	} else {
		log.Debugf("Path '%s' is a directory, checking for archives and images", absolutePath)
		// For directories, try to extract from archive files within the directory
		entries, err := os.ReadDir(absolutePath)
		if err != nil {
			log.Debugf("Failed to read directory '%s': %v", absolutePath, err)
			return "", err
		}

		log.Debugf("Found %d entries in directory '%s'", len(entries), absolutePath)

		// First, try to extract from archive files within the directory
		for _, entry := range entries {
			if !entry.IsDir() {
				lowerName := strings.ToLower(entry.Name())
				if strings.HasSuffix(lowerName, ".cbz") || strings.HasSuffix(lowerName, ".cbr") ||
					strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".rar") ||
					strings.HasSuffix(lowerName, ".epub") {
					archivePath := filepath.Join(absolutePath, entry.Name())
					log.Debugf("Extracting poster from archive '%s' in directory for media '%s'", entry.Name(), slug)
					return files.ExtractPosterImage(archivePath, slug, dataBackend, true)
				}
			}
		}

		log.Debugf("No archives found in directory for media '%s', checking for chapter directories with images", slug)

		// If no archives found, try to find chapter directories with loose images
		if entries != nil {
			// Sort entries to get the first chapter
			var dirs []string
			for _, entry := range entries {
				if entry.IsDir() {
					dirs = append(dirs, entry.Name())
				}
			}
			// Simple sort by name (assuming chapter names are sortable)
			sort.Strings(dirs)
			for _, dirName := range dirs {
				chapterPath := filepath.Join(absolutePath, dirName)
				chapterEntries, err := os.ReadDir(chapterPath)
				if err != nil {
					continue
				}
				for _, chapterEntry := range chapterEntries {
					if !chapterEntry.IsDir() {
						lowerName := strings.ToLower(chapterEntry.Name())
						if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
							strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".webp") ||
							strings.HasSuffix(lowerName, ".bmp") || strings.HasSuffix(lowerName, ".gif") {
							imagePath := filepath.Join(chapterPath, chapterEntry.Name())
							log.Debugf("Found first image '%s' in chapter directory '%s' for media '%s'", chapterEntry.Name(), dirName, slug)
							return processLocalImage(slug, imagePath, dataBackend)
						}
					}
				}
			}
		}

		// If no chapter directories found, check for loose images in the main directory
		log.Debugf("No chapter directories with images found for media '%s', checking for loose images in main directory", slug)
		if entries != nil {
			var imageFiles []string
			for _, entry := range entries {
				if !entry.IsDir() {
					lowerName := strings.ToLower(entry.Name())
					if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
						strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".webp") ||
						strings.HasSuffix(lowerName, ".bmp") || strings.HasSuffix(lowerName, ".gif") {
						imageFiles = append(imageFiles, entry.Name())
					}
				}
			}
			// Sort to get the first image alphabetically
			sort.Strings(imageFiles)
			if len(imageFiles) > 0 {
				imagePath := filepath.Join(absolutePath, imageFiles[0])
				log.Debugf("Found first loose image '%s' in main directory for media '%s'", imageFiles[0], slug)
				return processLocalImage(slug, imagePath, dataBackend)
			}
		}
	}

	log.Debugf("No local images found for poster generation for media '%s'", slug)
	return "", nil
}

func processLocalImage(slug, imagePath string, dataBackend filestore.DataBackend) (string, error) {
	return files.ProcessLocalImageWithThumbnails(imagePath, slug, dataBackend, true)
}

func DownloadAndStoreImage(slug, coverArtURL string, dataBackend filestore.DataBackend) (string, error) {
	log.Debugf("Attempting to download cover image for '%s' from URL: %s", slug, coverArtURL)

	// Determine the stored image URL
	storedImageURL := fmt.Sprintf("%s/%s.webp", localServerBaseURL, slug)

	// Retry logic for downloading images
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := files.DownloadImageWithThumbnails(slug, coverArtURL, dataBackend, true); err != nil {
			log.Warnf("Error downloading file from %s (attempt %d/%d): %s", coverArtURL, attempt, maxRetries, err)
			if attempt < maxRetries {
				// Wait before retrying (exponential backoff)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			log.Errorf("Failed to download image from %s after %d attempts", coverArtURL, maxRetries)
			return coverArtURL, nil
		}
		break
	}

	log.Debugf("Successfully downloaded and stored cover image for '%s'", slug)
	return storedImageURL, nil
}

// IndexChapters reconciles chapter files on disk with the stored chapter records.
// Returns added count, deleted count, new chapter slugs, and total file count.
// If dryRun is true, only counts files without performing database operations.
func IndexChapters(slug, path, librarySlug string, dryRun bool) (int, int, []string, int, error) {
	var addedCount int
	var deletedCount int
	var newChapterSlugs []string

	// Get library for root path
	library, err := models.GetLibrary(librarySlug)
	if err != nil {
		return 0, 0, nil, 0, fmt.Errorf("failed to get library '%s': %w", librarySlug, err)
	}

	// Find the root path that contains the path
	var rootPath string
	for _, folder := range library.Folders {
		if strings.HasPrefix(path, folder) {
			rootPath = folder
			break
		}
	}
	if rootPath == "" {
		return 0, 0, nil, 0, fmt.Errorf("could not find root path for %s in library %s", path, librarySlug)
	}

	// Load existing chapters
	existingMap, existing, err := loadExistingChapters(slug, librarySlug, dryRun)
	if err != nil {
		return 0, 0, nil, 0, err
	}

	// Handle missing path
	if !dryRun {
		if del, err := handleMissingPath(path, slug, existing); err != nil {
			return 0, del, nil, 0, err
		} else if del > 0 {
			return 0, del, nil, 0, nil
		}
	}

	// Collect present chapters
	presentMap, err := collectPresentChapters(path, rootPath)
	if err != nil {
		return 0, 0, nil, 0, err
	}

	// Process additions and deletions
	if !dryRun {
		tx, err := models.BeginTx()
		if err != nil {
			return 0, 0, nil, 0, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		addedCount, newChapterSlugs, err = processAdditions(tx, slug, presentMap, existingMap, librarySlug, rootPath)
		if err != nil {
			return 0, 0, nil, 0, err
		}

		deletedCount, err = processDeletions(tx, slug, presentMap, existingMap, librarySlug)
		if err != nil {
			return addedCount, deletedCount, newChapterSlugs, 0, err
		}

		if err := tx.Commit(); err != nil {
			return 0, 0, nil, 0, fmt.Errorf("failed to commit transaction: %w", err)
		}
	} else {
		deletedCount = processDeletionsDryRun(presentMap, existingMap)
	}

	// Update media file count
	if !dryRun {
		updateMediaFileCount(slug, len(presentMap))
	}

	return addedCount, deletedCount, newChapterSlugs, len(presentMap), nil
}

// loadExistingChapters loads existing chapters for the specific library if not dry run
func loadExistingChapters(slug, librarySlug string, dryRun bool) (map[string]models.Chapter, []models.Chapter, error) {
	if dryRun {
		return nil, nil, nil
	}
	existing, err := models.GetChapters(slug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load existing chapters for media '%s': %w", slug, err)
	}
	// Filter to only chapters from this library
	var filtered []models.Chapter
	for _, c := range existing {
		if c.LibrarySlug == librarySlug {
			filtered = append(filtered, c)
		}
	}
	existingMap := make(map[string]models.Chapter, len(filtered))
	for _, c := range filtered {
		existingMap[c.Slug] = c
	}
	return existingMap, filtered, nil
}

// handleMissingPath deletes all chapters if path doesn't exist
func handleMissingPath(path, slug string, existing []models.Chapter) (int, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Warnf("Media path '%s' does not exist, deleting all chapters for media '%s'", path, slug)
		tx, txErr := models.BeginTx()
		if txErr != nil {
			return 0, fmt.Errorf("failed to begin transaction: %w", txErr)
		}
		defer tx.Rollback()
		deletedCount := 0
		for _, chapter := range existing {
			if delErr := models.DeleteChapterTx(tx, slug, chapter.Slug, chapter.LibrarySlug); delErr != nil {
				log.Errorf("Failed to delete chapter '%s' for missing media '%s': %v", chapter.Slug, slug, delErr)
			} else {
				deletedCount++
			}
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return 0, fmt.Errorf("failed to commit transaction: %w", commitErr)
		}
		return deletedCount, nil
	}
	return 0, nil
}

// collectPresentChapters collects the present chapters from the path
func collectPresentChapters(path string, rootPath string) (map[string]presentInfo, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path '%s': %w", path, err)
	}

	presentMap := make(map[string]presentInfo)

	if !fileInfo.IsDir() {
		// Single file media - treat the file itself as chapter 1
		fileName := filepath.Base(path)
		chapterName := text.ExtractChapterName(fileName)
		chapterSlug := text.Sluggify(chapterName)
		relPath := strings.TrimPrefix(path, rootPath)
		if strings.HasPrefix(relPath, "/") {
			relPath = relPath[1:]
		}
		presentMap[chapterSlug] = presentInfo{Rel: relPath, Name: chapterName}
	} else {
		// Read the top-level entries (files and directories)
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory '%s': %w", path, err)
		}

		relativeMedia := strings.TrimPrefix(path, rootPath)
		if strings.HasPrefix(relativeMedia, "/") {
			relativeMedia = relativeMedia[1:]
		}

		for _, entry := range entries {
			name := entry.Name()
			cleanedName := text.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))
			if !containsNumber(cleanedName) {
				continue
			}
			chapterName := text.ExtractChapterName(name)
			chapterSlug := text.Sluggify(chapterName)
			relPath := filepath.Join(relativeMedia, name)
			presentMap[chapterSlug] = presentInfo{Rel: relPath, Name: chapterName}
		}
	}
	return presentMap, nil
}

// processAdditions creates missing chapters
func processAdditions(tx *sql.Tx, slug string, presentMap map[string]presentInfo, existingMap map[string]models.Chapter, librarySlug string, rootPath string) (int, []string, error) {
	addedCount := 0
	var newChapterSlugs []string

	// Get media for ComicInfo generation
	media, err := models.GetMedia(slug)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get media '%s': %w", slug, err)
	}

	for slugKey, info := range presentMap {
		if existingChapter, ok := existingMap[slugKey]; !ok {
			// not present in DB -> create with pretty name
			chapter := models.Chapter{
				Name:        info.Name,
				Slug:        slugKey,
				File:        info.Rel,
				MediaSlug:   slug,
				LibrarySlug: librarySlug,
			}
			if err := models.CreateChapterTx(tx, chapter); err != nil {
				return 0, nil, fmt.Errorf("failed to create chapter '%s' for media '%s': %w", info.Name, slug, err)
			}
			addedCount++
			newChapterSlugs = append(newChapterSlugs, slugKey)

			// Check if this is a CBZ file and add ComicInfo.xml
			fullPath := filepath.Join(rootPath, info.Rel)
			if strings.HasSuffix(strings.ToLower(fullPath), ".cbz") {
				if err := addComicInfoToChapter(media, chapter, fullPath); err != nil {
					log.Warnf("Failed to add ComicInfo.xml to chapter '%s': %v", fullPath, err)
					// Don't fail the indexing process for this
				}
			}
		} else {
			// Chapter exists, check if File needs updating
			if existingChapter.File != info.Rel {
				if err := models.UpdateChapterFileTx(tx, slug, slugKey, librarySlug, info.Rel); err != nil {
					return 0, nil, fmt.Errorf("failed to update chapter file for '%s' in media '%s': %w", slugKey, slug, err)
				}
			}
		}
	}
	return addedCount, newChapterSlugs, nil
}

// addComicInfoToChapter generates and adds ComicInfo.xml to a CBZ or CBR chapter file
func addComicInfoToChapter(media *models.Media, chapter models.Chapter, chapterPath string) error {
	// Check if metadata embedding is enabled
	cfg, err := models.GetAppConfig()
	if err != nil {
		return fmt.Errorf("failed to get app config: %w", err)
	}
	if !cfg.MetadataEmbeddingEnabled {
		log.Debugf("Metadata embedding disabled, skipping ComicInfo addition for: %s", chapterPath)
		return nil
	}

	// Count pages in the archive
	pageCount, err := files.CountPagesInArchive(chapterPath)
	if err != nil {
		return fmt.Errorf("failed to count pages in archive: %w", err)
	}

	// Extract chapter number from chapter name/slug
	chapterNumber := extractChapterNumber(chapter.Name)

	// Generate ComicInfo
	comicInfo := metadata.GenerateComicInfoFromMedia(media, chapterNumber, chapter.Name, pageCount)
	xmlData, err := comicInfo.ToXML()
	if err != nil {
		return fmt.Errorf("failed to generate ComicInfo XML: %w", err)
	}

	// Add to archive based on file type
	lowerPath := strings.ToLower(chapterPath)
	if strings.HasSuffix(lowerPath, ".cbz") || strings.HasSuffix(lowerPath, ".zip") {
		err = files.AddComicInfoToCBZ(chapterPath, xmlData)
		if err != nil {
			return fmt.Errorf("failed to add ComicInfo to CBZ: %w", err)
		}
	} else if strings.HasSuffix(lowerPath, ".cbr") || strings.HasSuffix(lowerPath, ".rar") {
		err = files.AddComicInfoToCBR(chapterPath, xmlData)
		if err != nil {
			log.Warnf("Failed to add ComicInfo to CBR file (rar command not available?): %v", err)
			// Don't fail the indexing process for CBR files if rar tools aren't available
			return nil
		}
	} else {
		return fmt.Errorf("unsupported archive format for ComicInfo addition: %s", chapterPath)
	}

	log.Debugf("Added ComicInfo.xml to chapter: %s", chapterPath)
	return nil
}

// processDeletions deletes chapters no longer present
func processDeletions(tx *sql.Tx, slug string, presentMap map[string]presentInfo, existingMap map[string]models.Chapter, librarySlug string) (int, error) {
	deletedCount := 0
	for slugKey := range existingMap {
		if _, ok := presentMap[slugKey]; !ok {
			if err := models.DeleteChapterTx(tx, slug, slugKey, librarySlug); err != nil {
				return 0, fmt.Errorf("failed to delete missing chapter '%s' for media '%s': %w", slugKey, slug, err)
			}
			deletedCount++
		}
	}
	return deletedCount, nil
}

// extractChapterNumber extracts the numeric part from a chapter name
func extractChapterNumber(chapterName string) string {
	// Look for patterns like "Chapter 1", "Volume 1", or just "1"
	if matches := regexp.MustCompile(`(?:Chapter|Volume)\s+(\d+)`).FindStringSubmatch(chapterName); matches != nil {
		return matches[1]
	}
	// If it's just a number
	if matches := regexp.MustCompile(`^(\d+)$`).FindStringSubmatch(chapterName); matches != nil {
		return matches[1]
	}
	// Default to 1
	return "1"
}

// processDeletionsDryRun counts deletions for dry run
func processDeletionsDryRun(presentMap map[string]presentInfo, existingMap map[string]models.Chapter) int {
	deletedCount := 0
	for slugKey := range existingMap {
		if _, ok := presentMap[slugKey]; !ok {
			deletedCount++
		}
	}
	return deletedCount
}

// updateMediaFileCount updates the media's file count
func updateMediaFileCount(slug string, fileCount int) {
	log.Debugf("Updating media '%s' file count to %d", slug, fileCount)
	m, err := models.GetMediaUnfiltered(slug)
	if err == nil && m != nil {
		m.FileCount = fileCount
		if err := models.UpdateMedia(m); err != nil {
			log.Errorf("Failed to update media file_count for '%s': %s", slug, err)
		}
	}
}

func containsNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// DetectWebtoonFromImages attempts to detect if a media is a webtoon by checking
// the aspect ratio of the middle image in the first chapter.
// Returns "webtoon" if detected, or empty string if not detected or on error.
func DetectWebtoonFromImages(mangaPath, slug string) string {
	// Check if path is a single file or directory
	fileInfo, err := os.Stat(mangaPath)
	if err != nil {
		log.Debugf("Failed to stat media path for webtoon detection: %v", err)
		return ""
	}

	var chapterPath string

	if fileInfo.IsDir() {
		// For directories, find the first chapter file/folder
		entries, err := os.ReadDir(mangaPath)
		if err != nil {
			log.Debugf("Failed to read directory for webtoon detection: %v", err)
			return ""
		}

		// Look for the first entry that looks like a chapter
		for _, entry := range entries {
			name := entry.Name()
			cleanedName := text.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))

			// Check if it contains a number (chapter indicator)
			if containsNumber(cleanedName) {
				chapterPath = filepath.Join(mangaPath, name)
				break
			}
		}

		// If no chapter found, try the first directory/file with images
		if chapterPath == "" && len(entries) > 0 {
			chapterPath = filepath.Join(mangaPath, entries[0].Name())
		}
	} else {
		// For single files, use the file directly as the chapter
		chapterPath = mangaPath
	}

	if chapterPath == "" {
		log.Debugf("No chapter path found for webtoon detection in: %s", mangaPath)
		return ""
	}

	// Get the middle image dimensions
	width, height, err := files.GetMiddleImageDimensions(chapterPath)
	if err != nil {
		log.Debugf("Failed to get image dimensions for webtoon detection: %v", err)
		return ""
	}

	log.Debugf("Webtoon detection for '%s': middle image dimensions = %dx%d", slug, width, height)

	// Check if the image is a webtoon (height >= 3x width)
	if files.IsWebtoonByAspectRatio(width, height) {
		log.Debugf("Detected webtoon for '%s' based on image aspect ratio (%dx%d)", slug, width, height)
		return "webtoon"
	}

	return ""
}

// extractEPUBMetadata extracts metadata from an EPUB file
func extractEPUBMetadata(epubPath string) (*EPUBMetadata, error) {
	reader, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer reader.Close()

	metadata := &EPUBMetadata{
		ContentRating: "safe",
	}

	// Find the OPF file
	var opfPath string
	for _, file := range reader.File {
		if !isSafeZipPath(file.Name) {
			continue
		}
		if strings.HasSuffix(file.Name, ".opf") {
			opfPath = file.Name
			break
		}
	}

	if opfPath == "" {
		log.Warnf("No OPF file found in EPUB: %s", epubPath)
		return metadata, nil
	}

	// Parse the OPF file
	opfFile, err := reader.Open(opfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OPF file: %w", err)
	}
	defer opfFile.Close()

	opfData, err := io.ReadAll(opfFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPF file: %w", err)
	}

	var opf OPF
	if err := xml.Unmarshal(opfData, &opf); err != nil {
		return nil, fmt.Errorf("failed to parse OPF: %w", err)
	}

	// Extract metadata
	if len(opf.Metadata.Meta) > 0 || len(opf.Metadata.Subject) > 0 {
		for _, meta := range opf.Metadata.Meta {
			switch meta.Name {
			case "author":
				metadata.Author = meta.Content
			case "description":
				metadata.Description = meta.Content
			case "year":
				if year, err := time.Parse("2006", meta.Content); err == nil {
					metadata.Year = year.Year()
				}
			case "language":
				metadata.Language = meta.Content
			case "status":
				metadata.Status = meta.Content
			case "content-rating":
				metadata.ContentRating = meta.Content
			}
		}

		// Extract tags from subject elements
		for _, subject := range opf.Metadata.Subject {
			metadata.Tags = append(metadata.Tags, subject)
		}
	}

	// Find cover image
	var coverHref string
	for _, item := range opf.Manifest.Item {
		if strings.Contains(item.ID, "cover") || item.Properties == "cover-image" {
			coverHref = item.Href
			break
		}
	}

	if coverHref != "" {
		// Resolve relative path
		coverPath := filepath.Join(filepath.Dir(opfPath), coverHref)
		coverPath = filepath.Clean(coverPath)

		// Validate coverPath before extraction
		if isSafeZipPath(coverPath) {
			// Extract and store the cover image
			if cachedURL, err := extractAndCacheEPUBImage(reader, coverPath, epubPath); err == nil {
				metadata.CoverArtURL = cachedURL
			} else {
				log.Warnf("Failed to extract cover from EPUB %s: %v", epubPath, err)
			}
		} else {
			log.Warnf("Unsafe cover image path in EPUB %s: %s", epubPath, coverPath)
		}
	}

	return metadata, nil
}

// extractAndCacheEPUBImage extracts a cover image from an EPUB and caches it
func extractAndCacheEPUBImage(reader *zip.ReadCloser, imagePath, epubPath string) (string, error) {
	// Find the image file in the ZIP
	var imageFile *zip.File
	for _, file := range reader.File {
		if !isSafeZipPath(file.Name) {
			continue
		}
		if file.Name == imagePath {
			imageFile = file
			break
		}
	}

	if imageFile == nil {
		return "", fmt.Errorf("cover image not found: %s", imagePath)
	}

	// Open the image file
	rc, err := imageFile.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer rc.Close()

	// Read the image data
	imageData, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// Generate slug from EPUB path
	baseName := filepath.Base(epubPath)
	slug := text.Sluggify(strings.TrimSuffix(baseName, filepath.Ext(baseName)))

	// Determine file extension
	ext := strings.ToLower(filepath.Ext(imagePath))
	if ext == "" {
		// Try to detect from content
		if len(imageData) > 4 {
			if string(imageData[:4]) == "\xff\xd8\xff\xe0" {
				ext = ".jpg"
			} else if string(imageData[:4]) == "\x89PNG" {
				ext = ".png"
			}
		}
	}

	if ext == "" {
		ext = ".jpg" // default
	}

	// Store the image
	dataPath := filepath.Join(DataDirectory, "posters", fmt.Sprintf("%s%s", slug, ext))
	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.WriteFile(dataPath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to store image: %w", err)
	}

	return fmt.Sprintf("/api/posters/%s%s", slug, ext), nil
}

// handleExistingMedia handles updating an existing media entry
func handleExistingMedia(existingMedia *models.Media, absolutePath, librarySlug, cleanedName, slug string, _ metadata.Provider, fileInfo os.FileInfo, isSingleFile bool) (string, error) {
	// If existing media has no tags, try to fetch metadata to get tags
	if len(existingMedia.Tags) == 0 {
		aggregatedMeta, err := metadata.QueryAllProviders(cleanedName)
		if err == nil && aggregatedMeta != nil && len(aggregatedMeta.Tags) > 0 {
			if err := models.SetTagsForMedia(slug, aggregatedMeta.Tags); err != nil {
				log.Errorf("Failed to set tags for existing media '%s': %s", slug, err)
			} else {
				log.Debugf("Fetched and set %d tags for existing media '%s'", len(aggregatedMeta.Tags), slug)
			}
		}
	}

	// Fast path 1: use stored file_count on the Media. If the number of
	// candidate files (files that look like chapters) matches the stored
	// FileCount, assume no changes and skip.
	if absolutePath != "" {
		var candidateCount int
		var err error

		if isSingleFile {
			// Single file media always has exactly 1 chapter
			candidateCount = 1
		} else {
			// Use dry run to get the actual count without database operations
			_, _, _, candidateCount, err = IndexChapters(slug, absolutePath, librarySlug, true)
			if err != nil {
				log.Errorf("Failed to count chapters for '%s': %s", slug, err)
				// Fall back to full indexing
			}
		}

		if candidateCount == existingMedia.FileCount {
			return slug, nil
		}
	}

	// Check if directory contains EPUB files, if so, set type to novel
	if ContainsEPUBFiles(absolutePath) && existingMedia.Type != "novel" {
		originalType := existingMedia.Type
		existingMedia.Type = "novel"
		log.Debugf("Updated type to novel (was '%s') for existing media '%s' based on presence of EPUB files", originalType, slug)
		if err := models.UpdateMedia(existingMedia); err != nil {
			log.Errorf("Failed to update media type for '%s': %s", slug, err)
		} else {
			BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Updated series '%s' - detected as novel (contains EPUB files)", cleanedName))
		}
	}

	// Skip indexing if the directory hasn't been modified since last update
	// But force re-indexing if the library changed (cross-library prioritization)
	// Or if the media was updated very recently (within last hour, indicating prioritization)
	forceReindex := false
	recentlyUpdated := time.Since(existingMedia.UpdatedAt) < 1*time.Hour
	if recentlyUpdated {
		forceReindex = true
	}
	if !forceReindex && !fileInfo.ModTime().After(existingMedia.UpdatedAt) {
		return slug, nil
	}

	added, deleted, newChapterSlugs, presentCount, err := IndexChapters(slug, absolutePath, librarySlug, false)
	if err != nil {
		log.Errorf("Failed to index chapters for existing media '%s': %s", slug, err)
		return slug, err
	}

	// Update FileCount after indexing
	existingMedia.FileCount = presentCount
	if err := models.UpdateMedia(existingMedia); err != nil {
		log.Errorf("Failed to update media file count for '%s': %s", slug, err)
	}

	// If no chapters remain, delete the media entirely
	if existingMedia.FileCount == 0 {
		// Double-check by querying all chapters for this media
		allChapters, err := models.GetChapters(slug)
		if err != nil {
			log.Errorf("Failed to check remaining chapters for media '%s': %s", slug, err)
		} else if len(allChapters) == 0 {
			log.Infof("Deleting media '%s' as it has no remaining chapters", slug)
			BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Deleted series '%s' - no chapters remaining", cleanedName))
			if err := models.DeleteMedia(slug); err != nil {
				log.Errorf("Failed to delete media '%s': %s", slug, err)
			}
			return slug, nil
		}
	}

	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in media '%s': %s", slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		log.Infof("Updated series: %s (added=%d deleted=%d) in %s", cleanedName, added, deleted, librarySlug)
		BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Updated series '%s' (added: %d chapters, deleted: %d)", cleanedName, added, deleted))
	}
	return slug, nil
}

// handleNewMedia handles creating a new media entry
func handleNewMedia(cleanedName, slug, librarySlug, absolutePath string, _ metadata.Provider, _ bool, dataBackend filestore.DataBackend) (string, error) {
	// Media does not exist yet — fetch metadata, create it and index chapters
	var aggregatedMeta *metadata.AggregatedMediaMetadata
	// Use aggregated metadata from all providers instead of single provider
	aggregatedMeta, err := metadata.QueryAllProviders(cleanedName)
	if err != nil {
		log.Errorf("Failed to find aggregated metadata for '%s': %s", cleanedName, err.Error())
	}

	var storedImageURL string
	// Note: async image processing will be started after media creation

	newMedia := createMediaFromAggregatedMetadata(aggregatedMeta, cleanedName, slug, librarySlug, absolutePath, storedImageURL)

	// Check if directory contains EPUB files, if so, set type to novel
	if ContainsEPUBFiles(absolutePath) {
		originalType := newMedia.Type
		newMedia.Type = "novel"
		log.Debugf("Detected novel (overriding metadata type '%s') for '%s' based on presence of EPUB files", originalType, slug)
	}

	// If no type was set from metadata, determine type based on image aspect ratio
	if newMedia.Type == "" {
		detectedType := DetectWebtoonFromImages(absolutePath, slug)
		if detectedType == "webtoon" {
			newMedia.Type = "webtoon"
			log.Debugf("Detected webtoon for '%s' based on image aspect ratio", slug)
		} else {
			newMedia.Type = "manga"
			log.Debugf("Defaulting to manga type for '%s' (no metadata type and not detected as webtoon)", slug)
		}
	}

	if err := models.CreateMedia(newMedia); err != nil {
		log.Errorf("Failed to create series: %s (%s)", slug, err.Error())
		BroadcastLog("indexer_"+librarySlug, "error", fmt.Sprintf("Failed to create series '%s': %s", cleanedName, err.Error()))
		return "", err
	}

	BroadcastLog("indexer_"+librarySlug, "info", fmt.Sprintf("Created new series: %s", cleanedName))

	// Persist tags from metadata provider (if any) - must be done right after CreateMedia
	if aggregatedMeta != nil && len(aggregatedMeta.Tags) > 0 {
		log.Debugf("Setting %d tags for new media '%s': %v", len(aggregatedMeta.Tags), slug, aggregatedMeta.Tags)
		if err := models.SetTagsForMedia(slug, aggregatedMeta.Tags); err != nil {
			log.Errorf("Failed to set tags for media '%s': %s", slug, err)
		}
	} else if aggregatedMeta != nil {
		log.Debugf("No tags found in metadata for new media '%s'", slug)
	}

	// Start async image processing after series is created
	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		go func() {
			var finalImageURL string
			log.Debugf("Starting async image processing for series '%s'", slug)

			// Always try local images first
			log.Debugf("Attempting local poster generation for new media '%s'", slug)
			url, err := HandleLocalImages(slug, absolutePath, dataBackend)
			log.Debugf("HandleLocalImages returned for '%s': url='%s', err='%v'", slug, url, err)
			if err == nil && url != "" {
				finalImageURL = url
				log.Debugf("Successfully generated poster from local images for new media '%s': %s", slug, finalImageURL)
			} else {
				log.Debugf("Failed to generate poster from local images for new media '%s': err=%v", slug, err)
				// If no local images, try metadata cover
				if aggregatedMeta != nil && len(aggregatedMeta.CoverArtURLs) > 0 {
					coverURL := aggregatedMeta.CoverArtURLs[0] // Use first cover URL
					if coverURL != "" {
						log.Debugf("Found metadata cover URL for '%s': %s", slug, coverURL)
						if url, err := DownloadAndStoreImage(slug, coverURL, dataBackend); err == nil {
							finalImageURL = url
							log.Debugf("Successfully downloaded cover for '%s': %s", slug, finalImageURL)
						} else {
							log.Debugf("Failed to download cover for '%s': %v", slug, err)
						}
					} else {
						log.Debugf("No cover URL in metadata for '%s'", slug)
					}
				} else {
					log.Debugf("No metadata found for '%s'", slug)
				}
			}

			// Update media with cover URL if we got one
			if finalImageURL != "" {
				log.Debugf("Updating media '%s' with cover URL: %s", slug, finalImageURL)
				if media, err := models.GetMediaUnfiltered(slug); err == nil && media != nil {
					media.CoverArtURL = finalImageURL
					if err := models.UpdateMedia(media); err != nil {
						log.Errorf("Failed to update cover URL for media '%s': %s", slug, err)
					} else {
						log.Debugf("Successfully updated cover URL for media '%s'", slug)
					}
				} else if err != nil {
					log.Errorf("Failed to get media for update '%s': %v", slug, err)
				} else {
					log.Debugf("Media '%s' no longer exists, skipping cover URL update (likely deleted due to no chapters)", slug)
				}
			} else {
				log.Debugf("No cover URL found for media '%s'", slug)
			}
		}()
	}

	added, deleted, newChapterSlugs, presentCount, err := IndexChapters(slug, absolutePath, librarySlug, false)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	// Update FileCount after indexing
	if presentCount == 0 {
		// No chapters found, delete the newly created media
		log.Warnf("No chapters found for new media '%s', deleting", slug)
		if err := models.DeleteMedia(slug); err != nil {
			log.Errorf("Failed to delete empty media '%s': %s", slug, err)
		}
		return "", nil
	} else {
		// Update the FileCount on the newly created media
		if media, err := models.GetMediaUnfiltered(slug); err == nil && media != nil {
			media.FileCount = presentCount
			if err := models.UpdateMedia(media); err != nil {
				log.Errorf("Failed to update file count for new media '%s': %s", slug, err)
			}
		}
	}

	// If new chapters were added, check for users reading this media and notify them
	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in media '%s': %s", slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		if aggregatedMeta == nil {
			log.Infof("Created series: '%s' (added=%d deleted=%d, fetched from local metadata)", cleanedName, added, deleted)
		} else {
			log.Infof("Created series: '%s' (added=%d deleted=%d)", cleanedName, added, deleted)
		}
	}
	return slug, nil
}
