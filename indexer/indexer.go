package indexer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2/log"

	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
)

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
	localServerBaseURL = "/api/images"
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
func IndexMedia(absolutePath, librarySlug string) (string, error) {
	defer utils.LogDuration("IndexMedia", time.Now(), absolutePath)

	// Check if this is a file (single-chapter media)
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path '%s': %w", absolutePath, err)
	}
	isSingleFile := !fileInfo.IsDir()

	// For single files, use the filename without extension as the media name
	baseName := filepath.Base(absolutePath)
	if isSingleFile {
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}
	
	cleanedName := utils.RemovePatterns(baseName)
	if cleanedName == "" {
		return "", nil
	}

	slug := utils.Sluggify(cleanedName)

	// If media already exists, avoid external API calls and heavy image work.
	// Only update the path if needed and index any new chapters.
	// Use GetMedia to check globally, then verify it's from the same library
	existingMedia, err := models.GetMedia(slug)
	if err != nil {
		log.Errorf("Failed to lookup media '%s': %s", slug, err)
	}

	if existingMedia != nil && existingMedia.LibrarySlug != librarySlug {
		log.Warnf("Media with slug '%s' already exists in a different library ('%s'), skipping indexing for library '%s'", slug, existingMedia.LibrarySlug, librarySlug)
		return "", nil
	}

	if existingMedia != nil {
		// Detect if this is a different folder being added to an existing media
		if existingMedia.Path != "" && existingMedia.Path != absolutePath {
			// Count chapters in the new folder
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
					cleanedName := utils.RemovePatterns(strings.TrimSuffix(name, filepath.Ext(name)))
					if containsNumber(cleanedName) {
						newCandidateCount++
					}
					return nil
				})
			}

			originalPath := existingMedia.Path

			// If the new folder has more chapters, prioritize it
			if newCandidateCount > existingMedia.FileCount {
				log.Infof("Prioritizing new folder with more chapters for media '%s': old='%s' (%d chapters), new='%s' (%d chapters)",
					slug, existingMedia.Path, existingMedia.FileCount, absolutePath, newCandidateCount)
				existingMedia.Path = absolutePath
				if err := models.UpdateMedia(existingMedia); err != nil {
					log.Errorf("Failed to update media path for '%s': %s", slug, err)
				}
			}

			// Ensure consistent ordering for the DB lookup
			fp1, fp2 := originalPath, absolutePath
			if fp1 > fp2 {
				fp1, fp2 = fp2, fp1
			}

			// Check if we've already recorded this duplicate; if so, skip logging/creating
			existingDup, err := models.GetMediaDuplicateByFolders(slug, fp1, fp2)
			if err != nil {
				// On DB error, fall back to logging and attempt to create (best-effort)
				log.Errorf("Failed to check existing media duplicate for '%s': %v", slug, err)
			}

			if existingDup == nil {
				// This is a new duplicate: log and record it
				log.Warnf("Detected duplicate folder for media '%s': existing='%s', new='%s'", 
					slug, originalPath, absolutePath)

				duplicate := models.MediaDuplicate{
					MediaSlug:   slug,
					LibrarySlug: librarySlug,
					FolderPath1: originalPath,
					FolderPath2: absolutePath,
				}

				if err := models.CreateMediaDuplicate(duplicate); err != nil {
					log.Errorf("Failed to record media duplicate for '%s': %v", slug, err)
				}
			}
			// Still index the chapters from this new folder
		}
		
		// Fast path 1: use stored file_count on the Media. If the number of
		// candidate files (files that look like chapters) matches the stored
		// FileCount, assume no changes and skip.
		if absolutePath != "" {
			var candidateCount int
			
			if isSingleFile {
				// Single file media always has exactly 1 chapter
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
			
			if candidateCount == existingMedia.FileCount {
				return slug, nil
			}
		}

		// Only update path if changed
		if existingMedia.Path == "" || existingMedia.Path != absolutePath {
			existingMedia.Path = absolutePath
			if err := models.UpdateMedia(existingMedia); err != nil {
				log.Errorf("Failed to update media path for '%s': %s", slug, err)
			}
		}

		// Check if directory contains EPUB files, if so, set type to novel
		if containsEPUBFiles(absolutePath) && existingMedia.Type != "novel" {
			originalType := existingMedia.Type
			existingMedia.Type = "novel"
			log.Debugf("Updated type to novel (was '%s') for existing media '%s' based on presence of EPUB files", originalType, slug)
			if err := models.UpdateMedia(existingMedia); err != nil {
				log.Errorf("Failed to update media type for '%s': %s", slug, err)
			}
		}

		// Index chapters recursively; returns added and deleted counts.
		added, deleted, newChapterSlugs, err := IndexChapters(slug, absolutePath)
		if err != nil {
			log.Errorf("Failed to index chapters for existing media '%s': %s", slug, err)
			return slug, err
		}
		
		// If new chapters were added, notify users
		if added > 0 && len(newChapterSlugs) > 0 {
			if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
				log.Errorf("Failed to create notifications for new chapters in media '%s': %s", slug, err)
			}
		}
		
		if added > 0 || deleted > 0 {
			// Update media updated_at to mark the index time
			if err := models.UpdateMedia(existingMedia); err != nil {
				log.Errorf("Failed to update media timestamp for '%s': %s", slug, err)
			}
			log.Infof("Indexed series: '%s' (added: %d deleted: %d)", cleanedName, added, deleted)
		}
		return slug, nil
	}

	// Media does not exist yet — fetch metadata, create it and index chapters
	config, err := models.GetAppConfig()
	var meta *metadata.MediaMetadata
	var provider metadata.Provider
	if err == nil {
		provider, err = metadata.GetProviderFromConfig(&config)
		if err == nil {
			meta, err = provider.FindBestMatch(cleanedName)
		if err != nil {
			log.Errorf("Failed to find metadata for '%s': %s", cleanedName, err.Error())
		}
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
		log.Debugf("No metadata cover found for new media '%s', attempting local poster generation", slug)
		cachedImageURL, _ = HandleLocalImages(slug, absolutePath)
		if cachedImageURL != "" {
			log.Debugf("Successfully generated poster from local images for new media '%s': %s", slug, cachedImageURL)
		} else {
			log.Debugf("Failed to generate poster from local images for new media '%s'", slug)
		}
	}

	newMedia := createMediaFromMetadata(meta, cleanedName, slug, librarySlug, absolutePath, cachedImageURL)

	// Check if directory contains EPUB files, if so, set type to novel
	if containsEPUBFiles(absolutePath) {
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
			newMedia.Type = "media"
			log.Debugf("Defaulting to media type for '%s' (no metadata type and not detected as webtoon)", slug)
		}
	}

	if err := models.CreateMedia(newMedia); err != nil {
		log.Errorf("Failed to create media: %s (%s)", slug, err.Error())
		return "", err
	}

	// Persist tags from metadata provider (if any)
	if meta != nil && len(meta.Tags) > 0 {
		if err := models.SetTagsForMedia(slug, meta.Tags); err != nil {
			log.Errorf("Failed to set tags for media '%s': %s", slug, err)
		}
	}

	added, deleted, newChapterSlugs, err := IndexChapters(slug, absolutePath)
	if err != nil {
		log.Errorf("Failed to index chapters: %s (%s)", slug, err.Error())
		return "", err
	}

	// If new chapters were added, check for users reading this media and notify them
	if added > 0 && len(newChapterSlugs) > 0 {
		if err := models.NotifyUsersOfNewChapters(slug, newChapterSlugs); err != nil {
			log.Errorf("Failed to create notifications for new chapters in media '%s': %s", slug, err)
		}
	}

	if added > 0 || deleted > 0 {
		if meta == nil {
			log.Infof("Indexed series: '%s' (added=%d deleted=%d, fetched from local metadata)", cleanedName, added, deleted)
		} else {
			log.Infof("Indexed series: '%s' (added=%d deleted=%d)", cleanedName, added, deleted)
		}
	}
	return slug, nil
}

func createMediaFromMetadata(meta *metadata.MediaMetadata, name, slug, librarySlug, path, coverURL string) models.Media {
	media := models.Media{
		Name:        name,
		Slug:        slug,
		LibrarySlug: librarySlug,
		Path:        path,
		CoverArtURL: coverURL,
	}
	
	if meta != nil {
		media.Description = meta.Description
		media.Year = meta.Year
		media.OriginalLanguage = meta.OriginalLanguage
		media.Type = meta.Type
		media.Status = meta.Status
		media.ContentRating = meta.ContentRating
		media.Author = meta.Author
	}
	
	return media
}

func HandleLocalImages(slug, absolutePath string) (string, error) {
	log.Debugf("Attempting to generate poster from local images for media '%s' at path '%s'", slug, absolutePath)
	
	// First, check for standalone poster/thumbnail images
	imageFiles := []string{"poster.jpg", "poster.jpeg", "poster.png", "thumbnail.jpg", "thumbnail.jpeg", "thumbnail.png"}

	for _, filename := range imageFiles {
		imagePath := filepath.Join(absolutePath, filename)
		if _, err := os.Stat(imagePath); err == nil {
			log.Debugf("Found standalone poster image '%s' for media '%s'", filename, slug)
			return processLocalImage(slug, imagePath)
		}
	}

	log.Debugf("No standalone poster images found for media '%s', checking archives", slug)
	
	// If no standalone image found, try to extract from archive files
	fileInfo, err := os.Stat(absolutePath)
	if err == nil && !fileInfo.IsDir() {
		// This is a file (likely an archive like .cbz, .cbr, .zip, .rar)
		lowerPath := strings.ToLower(absolutePath)
		if strings.HasSuffix(lowerPath, ".cbz") || strings.HasSuffix(lowerPath, ".cbr") ||
			strings.HasSuffix(lowerPath, ".zip") || strings.HasSuffix(lowerPath, ".rar") {
			log.Debugf("Extracting poster from single archive file '%s' for media '%s'", absolutePath, slug)
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
						log.Debugf("Extracting poster from archive '%s' in directory for media '%s'", entry.Name(), slug)
						return utils.ExtractAndCacheFirstImage(archivePath, slug, cacheDataDirectory)
					}
				}
			}
		}
		
		// If no archives found, try to find chapter directories with loose images
		log.Debugf("No archives found in directory for media '%s', checking for chapter directories with images", slug)
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
							return processLocalImage(slug, imagePath)
						}
					}
				}
			}
		}
	}

	log.Debugf("No local images found for poster generation for media '%s'", slug)
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
	log.Debugf("Attempting to download cover image for '%s' from URL: %s", slug, coverArtURL)
	
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

	log.Debugf("Successfully downloaded and cached cover image for '%s'", slug)
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
		return 0, 0, nil, fmt.Errorf("failed to load existing chapters for media '%s': %w", slug, err)
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
		// Single file media - treat the file itself as chapter 1
		fileName := filepath.Base(path)
		chapterName := utils.ExtractChapterName(fileName)
		chapterSlug := utils.Sluggify(chapterName)
		presentMap[chapterSlug] = presentInfo{Rel: fileName, Name: chapterName}
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
			chapterName := utils.ExtractChapterName(name)
			chapterSlug := utils.Sluggify(chapterName)
			relPath, err := filepath.Rel(path, p)
			if err != nil {
				relPath = name
			}
			presentMap[chapterSlug] = presentInfo{Rel: filepath.ToSlash(relPath), Name: chapterName}
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
				MediaSlug: slug,
			}
			if err := models.CreateChapter(chapter); err != nil {
				return addedCount, deletedCount, newChapterSlugs, fmt.Errorf("failed to create chapter '%s' for media '%s': %w", info.Name, slug, err)
			}
			addedCount++
			newChapterSlugs = append(newChapterSlugs, slugKey)
		}
	}

	// Delete chapters that are no longer present on disk
	for slugKey := range existingMap {
		if _, ok := presentMap[slugKey]; !ok {
			if err := models.DeleteChapter(slug, slugKey); err != nil {
				return addedCount, deletedCount, newChapterSlugs, fmt.Errorf("failed to delete missing chapter '%s' for media '%s': %w", slugKey, slug, err)
			}
			deletedCount++
		}
	}

	// Update media file count and timestamp
	m, err := models.GetMediaUnfiltered(slug)
	if err == nil && m != nil {
		m.FileCount = len(presentMap)
		if err := models.UpdateMedia(m); err != nil {
			log.Errorf("Failed to update media file_count for '%s': %s", slug, err)
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
			// Extract and cache the cover image
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
	slug := utils.Sluggify(strings.TrimSuffix(baseName, filepath.Ext(baseName)))

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

	// Cache the image
	cachePath := filepath.Join(cacheDataDirectory, fmt.Sprintf("%s%s", slug, ext))
	if err := os.WriteFile(cachePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to cache image: %w", err)
	}

	return fmt.Sprintf("/api/images/%s%s", slug, ext), nil
}
