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

	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
)

const (
	localServerBaseURL = "/api/images"
)

// IndexManga inspects a manga directory or file (.cbz/.cbr), syncing metadata and chapters with the database.
func IndexManga(absolutePath, librarySlug string) (string, error) {
	defer utils.LogDuration("IndexManga", time.Now(), absolutePath)

	// Check if this is a file (single-chapter manga)
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path '%s': %w", absolutePath, err)
	}
	isSingleFile := !fileInfo.IsDir()

	// For single files, use the filename without extension as the manga name
	baseName := filepath.Base(absolutePath)
	if isSingleFile {
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}
	
	cleanedName := utils.RemovePatterns(baseName)
	if cleanedName == "" {
		return "", nil
	}

	slug := utils.Sluggify(cleanedName)

	// If manga already exists, avoid external API calls and heavy image work.
	// Only update the path if needed and index any new chapters.
	// Use GetMangaUnfiltered to bypass content rating filter for indexing operations
	existingManga, err := models.GetMangaUnfiltered(slug)
	if err != nil {
		log.Errorf("Failed to lookup manga '%s': %s", slug, err)
	}

	if existingManga != nil {
		// Detect if this is a different folder being added to an existing manga
		if existingManga.Path != "" && existingManga.Path != absolutePath {
			// Ensure consistent ordering for the DB lookup
			fp1, fp2 := existingManga.Path, absolutePath
			if fp1 > fp2 {
				fp1, fp2 = fp2, fp1
			}

			// Check if we've already recorded this duplicate; if so, skip logging/creating
			existingDup, err := models.GetMangaDuplicateByFolders(slug, fp1, fp2)
			if err != nil {
				// On DB error, fall back to logging and attempt to create (best-effort)
				log.Errorf("Failed to check existing manga duplicate for '%s': %v", slug, err)
			}

			if existingDup == nil {
				// This is a new duplicate: log and record it
				log.Warnf("Detected duplicate folder for manga '%s': existing='%s', new='%s'", 
					slug, existingManga.Path, absolutePath)

				duplicate := models.MangaDuplicate{
					MangaSlug:   slug,
					LibrarySlug: librarySlug,
					FolderPath1: existingManga.Path,
					FolderPath2: absolutePath,
				}

				if err := models.CreateMangaDuplicate(duplicate); err != nil {
					log.Errorf("Failed to record manga duplicate for '%s': %v", slug, err)
				}
			}
			// Still index the chapters from this new folder
		}
		
		// Fast path 1: use stored file_count on the Manga. If the number of
		// candidate files (files that look like chapters) matches the stored
		// FileCount, assume no changes and skip.
		if absolutePath != "" {
			var candidateCount int
			
			if isSingleFile {
				// Single file manga always has exactly 1 chapter
				candidateCount = 1
			} else {
				// Count files (fast): we only need to count entries that look
				// like chapters (contain a number after cleaning). This is a
				// fast directory read that avoids creating objects.
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
			}
			
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
		added, deleted, newChapterSlugs, err := IndexChapters(slug, absolutePath)
		if err != nil {
			log.Errorf("Failed to index chapters for existing manga '%s': %s", slug, err)
			return slug, err
		}
		
		// If new chapters were added, notify users
		if added > 0 && len(newChapterSlugs) > 0 {
			if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
				log.Errorf("Failed to create notifications for new chapters in manga '%s': %s", slug, err)
			}
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
	config, err := models.GetAppConfig()
	var meta *metadata.MangaMetadata
	var provider metadata.Provider
	if err == nil {
		provider, err = metadata.GetProviderFromConfig(&config)
		if err == nil {
			meta, _ = provider.FindBestMatch(cleanedName)
		}
	}

	var cachedImageURL string
	if meta != nil {
		coverURL := provider.GetCoverImageURL(meta)
		if coverURL != "" {
			cachedImageURL, _ = DownloadAndCacheImage(slug, coverURL)
		}
	}
	
	// If no cover was found, try local images
	if cachedImageURL == "" {
		cachedImageURL, _ = HandleLocalImages(slug, absolutePath)
	}

	newManga := createMangaFromMetadata(meta, cleanedName, slug, librarySlug, absolutePath, cachedImageURL)

	// If no metadata match was found, try to detect webtoon by checking image dimensions
	if meta == nil {
		detectedType := detectWebtoonFromImages(absolutePath, slug)
		if detectedType != "" {
			newManga.Type = detectedType
		}
	}

	if err := models.CreateManga(newManga); err != nil {
		log.Errorf("Failed to create manga: %s (%s)", slug, err.Error())
		return "", err
	}

	// Persist tags from metadata provider (if any)
	if meta != nil && len(meta.Tags) > 0 {
		if err := models.SetTagsForManga(slug, meta.Tags); err != nil {
			log.Errorf("Failed to set tags for manga '%s': %s", slug, err)
		}
	}

	added, deleted, newChapterSlugs, err := IndexChapters(slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	// If new chapters were added, check for users reading this manga and notify them
	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in manga '%s': %s", slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		if meta == nil {
			log.Infof("Indexed manga: '%s' (added=%d deleted=%d, fetched from local metadata)", cleanedName, added, deleted)
		} else {
			log.Infof("Indexed manga: '%s' (added=%d deleted=%d)", cleanedName, added, deleted)
		}
	}
	return slug, nil
}

func createMangaFromMetadata(meta *metadata.MangaMetadata, name, slug, librarySlug, path, coverURL string) models.Manga {
	manga := models.Manga{
		Name:        name,
		Slug:        slug,
		LibrarySlug: librarySlug,
		Path:        path,
		CoverArtURL: coverURL,
	}
	
	if meta != nil {
		manga.Description = meta.Description
		manga.Year = meta.Year
		manga.OriginalLanguage = meta.OriginalLanguage
		manga.Type = meta.Type
		manga.Status = meta.Status
		manga.ContentRating = meta.ContentRating
		manga.Author = meta.Author
	}
	
	return manga
}

func HandleLocalImages(slug, absolutePath string) (string, error) {
	// First, check for standalone poster/thumbnail images
	imageFiles := []string{"poster.jpg", "poster.jpeg", "poster.png", "thumbnail.jpg", "thumbnail.jpeg", "thumbnail.png"}

	for _, filename := range imageFiles {
		imagePath := filepath.Join(absolutePath, filename)
		if _, err := os.Stat(imagePath); err == nil {
			return processLocalImage(slug, imagePath)
		}
	}

	// If no standalone image found, try to extract from archive files
	fileInfo, err := os.Stat(absolutePath)
	if err == nil && !fileInfo.IsDir() {
		// This is a file (likely an archive like .cbz, .cbr, .zip, .rar)
		lowerPath := strings.ToLower(absolutePath)
		if strings.HasSuffix(lowerPath, ".cbz") || strings.HasSuffix(lowerPath, ".cbr") ||
			strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".rar") {
			return utils.ExtractAndCacheFirstImage(absolutePath, slug, cacheDataDirectory)
		}
	} else if err == nil && fileInfo.IsDir() {
		// For directories, try to extract from archive files within the directory
		entries, err := os.ReadDir(absolutePath)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					lowerName := strings.ToLower(entry.Name())
					if strings.HasSuffix(lowerName, ".cbz") || strings.HasSuffix(lowerName, ".cbr") ||
						strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".rar") {
						archivePath := filepath.Join(absolutePath, entry.Name())
						return utils.ExtractAndCacheFirstImage(archivePath, slug, cacheDataDirectory)
					}
				}
			}
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

func DownloadAndCacheImage(slug, coverArtURL string) (string, error) {
	log.Infof("Attempting to download cover image for '%s' from URL: %s", slug, coverArtURL)
	
	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL: %s", err)
		return coverArtURL, nil
	}

	fileExt := filepath.Ext(u.Path)[1:]
	cachedImageURL := fmt.Sprintf("%s/%s.%s", localServerBaseURL, slug, fileExt)

	if err := utils.DownloadImage(cacheDataDirectory, slug, coverArtURL); err != nil {
		log.Errorf("Error downloading file from %s: %s", coverArtURL, err)
		return coverArtURL, nil
	}

	log.Infof("Successfully downloaded and cached cover image for '%s'", slug)
	return cachedImageURL, nil
}

// Deprecated: Use DownloadAndCacheImage instead
func downloadAndCacheImage(slug, coverArtURL string) (string, error) {
	return DownloadAndCacheImage(slug, coverArtURL)
}

// IndexChapters reconciles chapter files on disk with the stored chapter records.
func IndexChapters(slug, path string) (int, int, []string, error) {
	var addedCount int
	var deletedCount int
	var newChapterSlugs []string

	// Load existing chapters once to avoid querying the DB per file.
	existing, err := models.GetChapters(slug)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to load existing chapters for manga '%s': %w", slug, err)
	}
	existingMap := make(map[string]models.Chapter, len(existing))
	for _, c := range existing {
		existingMap[c.Slug] = c
	}

	// Check if path is a single file (for .cbz/.cbr files)
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to stat path '%s': %w", path, err)
	}

	type presentInfo struct {
		Rel  string
		Name string
	}
	presentMap := make(map[string]presentInfo)

	if !fileInfo.IsDir() {
		// Single file manga - treat the file itself as chapter 1
		fileName := filepath.Base(path)
		cleanedName := utils.RemovePatterns(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
		chapterSlug := utils.Sluggify(cleanedName)
		presentMap[chapterSlug] = presentInfo{Rel: fileName, Name: cleanedName}
	} else {
		// Build map of files currently present (slug -> relPath). This is a
		// full scan but cheaper than many DB ops; we only do it when file_count
		// mismatch was observed.
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
		});
		if err != nil {
			return 0, 0, nil, err
		}
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
				return addedCount, deletedCount, newChapterSlugs, fmt.Errorf("failed to create chapter '%s' for manga '%s': %w", info.Name, slug, err)
			}
			addedCount++
			newChapterSlugs = append(newChapterSlugs, slugKey)
		}
	}

	// Delete chapters that are no longer present on disk
	for slugKey := range existingMap {
		if _, ok := presentMap[slugKey]; !ok {
			if err := models.DeleteChapter(slug, slugKey); err != nil {
				return addedCount, deletedCount, newChapterSlugs, fmt.Errorf("failed to delete missing chapter '%s' for manga '%s': %w", slugKey, slug, err)
			}
			deletedCount++
		}
	}

	// Update manga file count and timestamp
	m, err := models.GetMangaUnfiltered(slug)
	if err == nil && m != nil {
		m.FileCount = len(presentMap)
		if err := models.UpdateManga(m); err != nil {
			log.Errorf("Failed to update manga file_count for '%s': %s", slug, err)
		}
	}

	return addedCount, deletedCount, newChapterSlugs, nil
}

func containsNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// detectWebtoonFromImages attempts to detect if a manga is a webtoon by checking
// the aspect ratio of the middle image in the first chapter.
// Returns "webtoon" if detected, or empty string if not detected or on error.
func detectWebtoonFromImages(mangaPath, slug string) string {
	// Check if path is a single file or directory
	fileInfo, err := os.Stat(mangaPath)
	if err != nil {
		log.Debugf("Failed to stat manga path for webtoon detection: %v", err)
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
			cleanedName := utils.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))

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
	width, height, err := utils.GetMiddleImageDimensions(chapterPath)
	if err != nil {
		log.Debugf("Failed to get image dimensions for webtoon detection: %v", err)
		return ""
	}

	log.Debugf("Webtoon detection for '%s': middle image dimensions = %dx%d", slug, width, height)

	// Check if the image is a webtoon (height >= 3x width)
	if utils.IsWebtoonByAspectRatio(width, height) {
		log.Infof("Detected webtoon for '%s' based on image aspect ratio (%dx%d)", slug, width, height)
		return "webtoon"
	}

	return ""
}

