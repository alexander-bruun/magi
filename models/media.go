package models

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

// Media represents the media table schema
type Media struct {
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Author           string    `json:"author"`
	Description      string    `json:"description"`
	Year             int       `json:"year"`
	OriginalLanguage string    `json:"original_language"`
	Type             string    `json:"type"`
	Status           string    `json:"status"`
	ContentRating    string    `json:"content_rating"`
	LibrarySlug      string    `json:"library_slug"`
	CoverArtURL      string    `json:"cover_art_url"`
	Path             string    `json:"path"`
	FileCount        int       `json:"file_count"`
	ReadCount        int       `json:"read_count"`
	VoteScore        int       `json:"vote_score"`
	Tags             []string  `json:"tags"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PremiumCountdown string    `json:"premium_countdown,omitempty"`
}

// EnrichedMedia extends Media with premium countdown information
type EnrichedMedia struct {
	Media
	PremiumCountdown  string
	LatestChapterSlug  string
	LatestChapterName  string
	AverageRating      float64
	ReviewCount        int
}

// CreateMedia adds a new media to the database
func CreateMedia(media Media) error {
	exists, err := MediaExists(media.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("media already exists")
	}

	timestamps := NewTimestamps()
	media.CreatedAt = timestamps.CreatedAt
	media.UpdatedAt = timestamps.UpdatedAt

	query := `
	INSERT INTO media (slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	createdAt, updatedAt := timestamps.UnixTimestamps()
	_, err = db.Exec(query, media.Slug, media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.LibrarySlug, media.CoverArtURL, media.Path, media.FileCount, createdAt, updatedAt)
	return err
}

// GetMedia retrieves a single media by slug
func GetMedia(slug string) (*Media, error) {
	return getMedia(slug, true)
}

// GetMediaUnfiltered retrieves a single media by slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetMediaUnfiltered(slug string) (*Media, error) {
	return getMedia(slug, false)
}

// GetMediaBySlugAndLibrary retrieves a single media by slug and library slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetMediaBySlugAndLibrary(slug, librarySlug string) (*Media, error) {
	return getMediaBySlugAndLibrary(slug, librarySlug, false)
}

// GetMediaBySlugAndLibraryFiltered retrieves a single media by slug and library slug with content rating filtering.
func GetMediaBySlugAndLibraryFiltered(slug, librarySlug string) (*Media, error) {
	return getMediaBySlugAndLibrary(slug, librarySlug, true)
}

// getMedia is the internal implementation that optionally applies content rating filtering
func getMedia(slug string, applyContentFilter bool) (*Media, error) {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = ?`

	row := db.QueryRow(query, slug)

	var m Media
	var createdAt, updatedAt int64
	err := row.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No media found
		}
		return nil, err
	}

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)
	
	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		cfg, err := GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for content rating check: %v", err)
			// On error, default to showing content
		} else if !IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			return nil, nil // Return nil to indicate media not found/accessible
		}
	}
	
	// Load tags for this media if any
	if tags, err := GetTagsForMedia(m.Slug); err == nil {
		m.Tags = tags
		log.Debugf("Loaded %d tags for media '%s': %v", len(tags), m.Slug, tags)
	} else {
		log.Errorf("Failed to load tags for media '%s': %v", m.Slug, err)
	}
	return &m, nil
}

// getMediaBySlugAndLibrary is the internal implementation for getting media by slug and library
func getMediaBySlugAndLibrary(slug, librarySlug string, applyContentFilter bool) (*Media, error) {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = ? AND library_slug = ?`

	row := db.QueryRow(query, slug, librarySlug)

	var m Media
	var createdAt, updatedAt int64
	err := row.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No media found
		}
		return nil, err
	}

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)
	
	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		cfg, err := GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for content rating check: %v", err)
			// On error, default to showing content
		} else if !IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			return nil, nil // Return nil to indicate media not found/accessible
		}
	}
	
	// Load tags for this media if any
	if tags, err := GetTagsForMedia(m.Slug); err == nil {
		m.Tags = tags
	}
	return &m, nil
}

// Setter methods to implement metadata.MediaUpdater interface
func (m *Media) SetName(name string)                   { m.Name = name }
func (m *Media) SetDescription(desc string)            { m.Description = desc }
func (m *Media) SetYear(year int)                      { m.Year = year }
func (m *Media) SetOriginalLanguage(lang string)       { m.OriginalLanguage = lang }
func (m *Media) SetStatus(status string)               { m.Status = status }
func (m *Media) SetContentRating(rating string)        { m.ContentRating = rating }
func (m *Media) SetType(mangaType string)              { m.Type = mangaType }
func (m *Media) SetCoverArtURL(url string)             { m.CoverArtURL = url }

// UpdateMedia modifies an existing media and always updates the updated_at timestamp to the current time
func UpdateMedia(media *Media) error {
	media.UpdatedAt = time.Now()

	query := `
	UPDATE media
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, type = ?, status = ?, content_rating = ?, library_slug = ?, cover_art_url = ?, path = ?, file_count = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.LibrarySlug, media.CoverArtURL, media.Path, media.FileCount, media.UpdatedAt.Unix(), media.Slug)
	if err != nil {
		return err
	}

	return nil
}

// UpdateMediaMetadata updates media metadata fields while preserving the original created_at timestamp.
// This is useful for refresh operations where we want to preserve the original creation date.
// It does not update the updated_at timestamp.
func UpdateMediaMetadata(media *Media) error {
	query := `
	UPDATE media
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, type = ?, status = ?, content_rating = ?, cover_art_url = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.CoverArtURL, media.Slug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMedia removes a media and its associated chapters
func DeleteMedia(slug string) error {
	// Delete poster images
	deletePosterImages(slug)

	// Delete associated chapters first
	if err := DeleteChaptersByMediaSlug(slug); err != nil {
		return err
	}

	// Delete associated tags
	if err := DeleteTagsByMediaSlug(slug); err != nil {
		return err
	}

	return DeleteRecord(`DELETE FROM media WHERE slug = ?`, slug)
}

// deletePosterImages deletes the poster image files for a media
func deletePosterImages(slug string) {
	cacheDir := utils.GetCacheDirectory()
	postersDir := filepath.Join(cacheDir, "posters")

	// Delete main poster image
	mainPath := filepath.Join(postersDir, fmt.Sprintf("%s.jpg", slug))
	if err := os.Remove(mainPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster image %s: %v", mainPath, err)
	}

	// Delete original image
	originalPath := filepath.Join(postersDir, fmt.Sprintf("%s_original.jpg", slug))
	if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster original image %s: %v", originalPath, err)
	}

	// Delete thumbnail
	thumbPath := filepath.Join(postersDir, fmt.Sprintf("%s_thumb.jpg", slug))
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster thumbnail %s: %v", thumbPath, err)
	}

	// Delete small image
	smallPath := filepath.Join(postersDir, fmt.Sprintf("%s_small.jpg", slug))
	if err := os.Remove(smallPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster small image %s: %v", smallPath, err)
	}
}

// SearchMedias filters, sorts, and paginates media based on provided criteria
func SearchMedias(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
	})
}

// SearchMediasWithTags extends SearchMedias to filter by selected tags (all must match)
func SearchMediasWithTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
		Tags:        selectedTags,
		TagMode:     "all",
	})
}

// SearchMediasWithAnyTags filters media to those that have at least one of the selected tags
func SearchMediasWithAnyTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
		Tags:        selectedTags,
		TagMode:     "any",
	})
}

// MediaExists checks if a media exists by slug
func MediaExists(slug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM media WHERE slug = ?`, slug)
}

// MediaCount counts the number of media based on filter criteria
func MediaCount(filterBy, filter string) (int, error) {
	var mediaList []Media
	if err := loadAllMedias(&mediaList); err != nil {
		return 0, err
	}

	count := 0
	for _, media := range mediaList {
		if filterBy != "" && filter != "" {
			value := reflect.ValueOf(media).FieldByName(filterBy).String()
			if strings.Contains(strings.ToLower(value), strings.ToLower(filter)) {
				count++
			}
		} else {
			count++
		}
	}
	return count, nil
}

// DeleteMediasByLibrarySlug removes all media associated with a specific library
func DeleteMediasByLibrarySlug(librarySlug string) error {
	query := `SELECT slug FROM media WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query media by librarySlug: %v", err)
		return err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			log.Errorf("Failed to scan row: %v", err)
			return err
		}
		slugs = append(slugs, slug)
	}

	for _, slug := range slugs {
		if err := DeleteMedia(slug); err != nil {
			log.Errorf("Failed to delete media with slug '%s': %s", slug, err.Error())
			return err
		}
	}

	return nil
}

// GetMediasBySlugs loads multiple media by slugs with their tags in batch to avoid N+1 queries
func GetMediasBySlugs(slugs []string) ([]Media, error) {
	if len(slugs) == 0 {
		return []Media{}, nil
	}

	// Build query with IN clause
	placeholders := strings.Repeat("?,", len(slugs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug IN (%s)`, placeholders)

	args := make([]interface{}, len(slugs))
	for i, slug := range slugs {
		args[i] = slug
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config for content rating check: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	var medias []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)

		// Apply content rating filter
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			medias = append(medias, m)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch load tags for all media
	if len(medias) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for media batch: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range medias {
				if tags, ok := tagMap[medias[i].Slug]; ok {
					medias[i].Tags = tags
				}
			}
		}
	}

	return medias, nil
}

// GetMediasByLibrarySlug returns all media that belong to a specific library
func GetMediasByLibrarySlug(librarySlug string) ([]Media, error) {
	var mediaList []Media
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query media by librarySlug: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config, defaulting to show all content: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &createdAt, &updatedAt); err != nil {
			log.Errorf("Failed to scan media row: %v", err)
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		
		// Filter based on content rating limit
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			mediaList = append(mediaList, m)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mediaList, nil
}

// GetTopMedias returns the top media ordered by vote score (descending).
// It joins the media table with aggregated votes and respects content rating limits.
func GetTopMedias(limit int) ([]Media, error) {
	cfg, err := GetAppConfig()
	if err != nil {
		// If we can't get config, default to showing all content
		cfg.ContentRatingLimit = 3
	}

	var allowedRatings []string
	switch cfg.ContentRatingLimit {
	case 0:
		allowedRatings = []string{"safe"}
	case 1:
		allowedRatings = []string{"safe", "suggestive"}
	case 2:
		allowedRatings = []string{"safe", "suggestive", "erotica"}
	default:
		allowedRatings = []string{"safe", "suggestive", "erotica", "pornographic"}
	}

	placeholders := strings.Repeat("?,", len(allowedRatings))
	placeholders = placeholders[:len(placeholders)-1] // remove trailing comma

	query := fmt.Sprintf(`
	SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count, m.created_at, m.updated_at
	FROM media m
	LEFT JOIN (
		SELECT media_slug, 
			CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
			THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
			ELSE 0 END as score
		FROM votes
		GROUP BY media_slug
	) v ON v.media_slug = m.slug
	WHERE m.content_rating IN (%s)
	ORDER BY v.score DESC
	LIMIT ?
	`, placeholders)

	args := make([]interface{}, len(allowedRatings)+1)
	for i, rating := range allowedRatings {
		args[i] = rating
	}
	args[len(allowedRatings)] = limit

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)

		mediaList = append(mediaList, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch load tags for all media
	if len(mediaList) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for top media: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range mediaList {
				if tags, ok := tagMap[mediaList[i].Slug]; ok {
					mediaList[i].Tags = tags
				}
			}
		}
	}

	return mediaList, nil
}

// Helper functions

func loadAllMedias(media *[]Media) error {
	return loadAllMediasWithTags(media, false)
}

// loadAllMediasWithTags loads all media with optional tag loading to avoid N+1 queries when tags are needed
func loadAllMediasWithTags(media *[]Media, loadTags bool) error {
	query := `SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count, 
		COALESCE(read_counts.read_count, 0) as read_count,
		COALESCE(vote_scores.score, 0) as vote_score,
		m.created_at, m.updated_at 
	FROM media m
	LEFT JOIN (
		SELECT media_slug, COUNT(*) as read_count
		FROM reading_states
		GROUP BY media_slug
	) read_counts ON m.slug = read_counts.media_slug
	LEFT JOIN (
		SELECT media_slug, 
			CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
			THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
			ELSE 0 END as score
		FROM votes
		GROUP BY media_slug
	) vote_scores ON m.slug = vote_scores.media_slug`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to get all media: %v", err)
		return err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config, defaulting to show all content: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var voteScore int
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt); err != nil {
			return err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore
		
		// Filter based on content rating limit
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			*media = append(*media, m)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Batch load tags for all media if requested
	if loadTags && len(*media) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for media batch: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range *media {
				if tags, ok := tagMap[(*media)[i].Slug]; ok {
					(*media)[i].Tags = tags
				}
			}
		}
	}

	return nil
}

// GetAllMediaTypes returns all distinct type values (lowercased) sorted ascending
func GetAllMediaTypes() ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT LOWER(TRIM(type)) FROM media WHERE type IS NOT NULL AND TRIM(type) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t != "" {
			types = append(types, t)
		}
	}
	sort.Strings(types)
	return types, nil
}

func applyBigramSearch(filter string, mediaList []Media) []Media {
	var mediaNames []string
	nameToMedia := make(map[string]Media, len(mediaList))

	for _, media := range mediaList {
		mediaNames = append(mediaNames, media.Name)
		nameToMedia[media.Name] = media
	}

	matchingNames := utils.BigramSearch(filter, mediaNames)

	filtered := make([]Media, 0, len(matchingNames))
	for _, name := range matchingNames {
		if media, ok := nameToMedia[name]; ok {
			filtered = append(filtered, media)
		}
	}

	return filtered
}

// sortMedias moved to sorting.go (exported as SortMedias) for reuse across account pages.

// Vote represents a user's vote on a media
type Vote struct {
	ID           int64
	UserUsername string
	MediaSlug    string
	Value        int // 1 for upvote, -1 for downvote
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetMediaVotes returns the aggregated score and counts for a media
func GetMediaVotes(mangaSlug string) (score int, upvotes int, downvotes int, err error) {
	// Use COALESCE so aggregates return 0 instead of NULL when there are no rows
	query := `SELECT 
		CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
		THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
		ELSE 0 END as score,
		COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) as upvotes, 
		COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) as downvotes 
		FROM votes WHERE media_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	if err := row.Scan(&score, &upvotes, &downvotes); err != nil {
		return 0, 0, 0, err
	}
	return score, upvotes, downvotes, nil
}

// GetUserVoteForMedia returns the vote value (1, -1) for a user on a media. If none, returns 0.
func GetUserVoteForMedia(username, mangaSlug string) (int, error) {
	query := `SELECT value FROM votes WHERE user_username = ? AND media_slug = ?`
	row := db.QueryRow(query, username, mangaSlug)
	var val int
	err := row.Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// SetVote inserts or updates a user's vote for a media. value must be 1 or -1.
func SetVote(username, mangaSlug string, value int) error {
	if value != 1 && value != -1 {
		return errors.New("invalid vote value")
	}
	now := time.Now().Unix()
	// Try update first
	res, err := db.Exec(`UPDATE votes SET value = ?, updated_at = ? WHERE user_username = ? AND media_slug = ?`, value, now, username, mangaSlug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// Insert
	_, err = db.Exec(`INSERT INTO votes (user_username, media_slug, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, username, mangaSlug, value, now, now)
	return err
}

// RemoveVote deletes a user's vote for a media
func RemoveVote(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM votes WHERE user_username = ? AND media_slug = ?`, username, mangaSlug)
	return err
}

// Favorite represents a user's favorite media
type Favorite struct {
	ID           int64
	UserUsername string
	MediaSlug    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SetFavorite inserts a favorite relationship for a user and media.
func SetFavorite(username, mangaSlug string) error {
	now := time.Now().Unix()
	// Try update first (in case row exists) - this keeps updated_at current
	res, err := db.Exec(`UPDATE favorites SET updated_at = ? WHERE user_username = ? AND media_slug = ?`, now, username, mangaSlug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = db.Exec(`INSERT INTO favorites (user_username, media_slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, mangaSlug, now, now)
	return err
}

// RemoveFavorite deletes a user's favorite for a media
func RemoveFavorite(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM favorites WHERE user_username = ? AND media_slug = ?`, username, mangaSlug)
	return err
}

// ToggleFavorite toggles the favorite status for a user and media
func ToggleFavorite(username, mangaSlug string) error {
	isFavorite, err := IsFavoriteForUser(username, mangaSlug)
	if err != nil {
		return err
	}

	if isFavorite {
		return RemoveFavorite(username, mangaSlug)
	} else {
		return SetFavorite(username, mangaSlug)
	}
}

// IsFavoriteForUser returns true if the user has favorited the media
func IsFavoriteForUser(username, mangaSlug string) (bool, error) {
	query := `SELECT 1 FROM favorites WHERE user_username = ? AND media_slug = ?`
	row := db.QueryRow(query, username, mangaSlug)
	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFavoritesCount returns the number of users who favorited the media
func GetFavoritesCount(mangaSlug string) (int, error) {
	query := `SELECT COUNT(*) FROM favorites WHERE media_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetFavoritesForUser returns media slugs favorited by the user ordered by most recent update
func GetFavoritesForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM favorites WHERE user_username = ? ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetReadingMediasForUser returns distinct media slugs that the user has reading state records for,
// ordered by most recent activity.
func GetReadingMediasForUser(username string) ([]string, error) {
	query := `SELECT DISTINCT media_slug FROM reading_states WHERE user_name = ? ORDER BY created_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetUpvotedMediasForUser returns media slugs the user has upvoted (value = 1), ordered by most recent vote
func GetUpvotedMediasForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM votes WHERE user_username = ? AND value = 1 ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetDownvotedMediasForUser returns media slugs the user has downvoted (value = -1), ordered by most recent vote
func GetDownvotedMediasForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM votes WHERE user_username = ? AND value = -1 ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// UserMediaListOptions defines parameters for user-specific media list queries (favorites, reading, etc.)
type UserMediaListOptions struct {
	Username            string
	Page                int
	PageSize            int
	SortBy              string
	SortOrder           string
	Tags                []string
	TagMode             string // "all" or "any"
	SearchFilter        string
	AccessibleLibraries []string // filter by accessible libraries for permission system
	Types               []string // filter by media types (any match)
}

// GetUserFavoritesWithOptions fetches, filters, sorts, and paginates a user's favorite media
func GetUserFavoritesWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetFavoritesForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserReadingWithOptions fetches, filters, sorts, and paginates a user's reading list
func GetUserReadingWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetReadingMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserUpvotedWithOptions fetches, filters, sorts, and paginates a user's upvoted media
func GetUserUpvotedWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetUpvotedMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserDownvotedWithOptions fetches, filters, sorts, and paginates a user's downvoted media
func GetUserDownvotedWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetDownvotedMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// processUserMediaList handles the common logic for filtering, sorting, and paginating user media lists
func processUserMediaList(slugs []string, opts UserMediaListOptions) ([]Media, int, error) {
	// Load all media from slugs in batch (with tags) to avoid N+1 queries
	allMedias, err := GetMediasBySlugs(slugs)
	if err != nil {
		return nil, 0, err
	}

	// Filter by accessible libraries (permission system)
	if len(opts.AccessibleLibraries) > 0 {
		librarySet := make(map[string]struct{}, len(opts.AccessibleLibraries))
		for _, lib := range opts.AccessibleLibraries {
			librarySet[lib] = struct{}{}
		}

		filtered := make([]Media, 0, len(allMedias))
		for _, m := range allMedias {
			if _, ok := librarySet[m.LibrarySlug]; ok {
				filtered = append(filtered, m)
			}
		}
		allMedias = filtered
	}

	// Filter by tags if specified
	if len(opts.Tags) > 0 {
		allMedias = FilterMediasByTags(allMedias, opts.Tags, opts.TagMode)
	}

	// Filter by search term if specified
	if opts.SearchFilter != "" {
		allMedias = FilterMediasBySearch(allMedias, opts.SearchFilter)
	}

	// Filter by types if specified
	if len(opts.Types) > 0 {
		typeSet := make(map[string]struct{}, len(opts.Types))
		for _, t := range opts.Types {
			typeSet[t] = struct{}{}
		}

		filtered := make([]Media, 0, len(allMedias))
		for _, m := range allMedias {
			if _, ok := typeSet[m.Type]; ok {
				filtered = append(filtered, m)
			}
		}
		allMedias = filtered
	}

	// Sort media
	SortMedias(allMedias, opts.SortBy, opts.SortOrder)

	// Calculate total before pagination
	total := len(allMedias)

	// Paginate
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start > len(allMedias) {
		start = len(allMedias)
	}
	if end > len(allMedias) {
		end = len(allMedias)
	}

	return allMedias[start:end], total, nil
}

// FilterMediasByTags filters a slice of media by selected tags
// tagMode can be "all" (all tags must match) or "any" (at least one tag must match)
// FilterMediasByTags filters a slice of media by selected tags
// This function assumes that media.Tags are already populated to avoid N+1 queries
func FilterMediasByTags(mediaList []Media, selectedTags []string, tagMode string) []Media {
	if len(selectedTags) == 0 {
		return mediaList
	}

	var filtered []Media
	for _, media := range mediaList {
		if tagMode == "any" {
			// At least one selected tag must be in media's tags
			for _, selTag := range selectedTags {
				for _, mTag := range media.Tags {
					if strings.EqualFold(selTag, mTag) {
						filtered = append(filtered, media)
						goto nextMedia
					}
				}
			}
		} else {
			// All selected tags must be in media's tags
			matchCount := 0
			for _, selTag := range selectedTags {
				for _, mTag := range media.Tags {
					if strings.EqualFold(selTag, mTag) {
						matchCount++
						break
					}
				}
			}
			if matchCount == len(selectedTags) {
				filtered = append(filtered, media)
			}
		}
	nextMedia:
	}
	return filtered
}

// FilterMediasBySearch filters a slice of media by search term using very lenient fuzzy matching
func FilterMediasBySearch(mediaList []Media, searchTerm string) []Media {
	if searchTerm == "" {
		return mediaList
	}

	// Aggressive normalization function
	normalize := func(s string) string {
		s = strings.ToLower(s)
		// Remove all non-alphanumeric characters except spaces
		var result strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
				result.WriteRune(r)
			} else if r >= 'A' && r <= 'Z' {
				result.WriteRune(r + 32) // Convert to lowercase
			} else {
				// Replace any other character with space
				result.WriteRune(' ')
			}
		}
		return result.String()
	}

	// Normalize and split search term
	normalizedSearch := normalize(searchTerm)
	searchWords := strings.Fields(normalizedSearch)
	if len(searchWords) == 0 {
		return mediaList
	}

	var filtered []Media
	for _, media := range mediaList {
		// Normalize media name
		normalizedName := normalize(media.Name)

		// Check if all search words match
		matched := true
		for _, searchWord := range searchWords {
			if searchWord == "" {
				continue
			}

			// First check: simple substring match
			if strings.Contains(normalizedName, searchWord) {
				continue
			}

			// Second check: word prefix match
			nameWords := strings.Fields(normalizedName)
			wordMatched := false
			for _, nameWord := range nameWords {
				if strings.HasPrefix(nameWord, searchWord) {
					wordMatched = true
					break
				}
				// Also check if the search word appears within the name word (substring)
				if strings.Contains(nameWord, searchWord) {
					wordMatched = true
					break
				}
			}

			if !wordMatched {
				matched = false
				break
			}
		}

		if matched {
			filtered = append(filtered, media)
		}
	}
	return filtered
}

// GetMediaAndChapters retrieves a media and its chapters in one call
func GetMediaAndChapters(mangaSlug string) (*Media, []Chapter, error) {
	media, err := GetMedia(mangaSlug)
	if err != nil {
		return nil, nil, err
	}
	if media == nil {
		return nil, nil, fmt.Errorf("media not found or access restricted")
	}

	chapters, err := GetChapters(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	return media, chapters, nil
}

// GetChapterImages generates URLs for all images in a chapter
func GetChapterImages(media *Media, chapter *Chapter) ([]string, error) {
	// Determine the actual chapter file path
	// For single-file media (cbz/cbr), media.Path is the file itself
	// For directory-based media, we need to join path and chapter file
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

	pageCount, err := utils.CountImageFiles(chapterFilePath)
	if err != nil {
		return nil, err
	}

	if pageCount <= 0 {
		return []string{}, nil
	}

	images := make([]string, pageCount)
	for i := 0; i < pageCount; i++ {
		// Generate one-time use token
		validityMinutes := GetImageTokenValidityMinutes()
		token := utils.GenerateImageAccessTokenWithValidity(media.Slug, chapter.Slug, i+1, validityMinutes)
		images[i] = fmt.Sprintf("/api/image?token=%s", token)
	}

	return images, nil
}

// GetFirstAndLastChapterSlugs returns the first and last chapter slugs for a media
func GetFirstAndLastChapterSlugs(chapters []Chapter) (firstSlug, lastSlug string) {
	if len(chapters) > 0 {
		firstSlug = chapters[len(chapters)-1].Slug
		lastSlug = chapters[0].Slug
	}
	return firstSlug, lastSlug
}

// SearchOptions defines parameters for media searches
type SearchOptions struct {
	Filter             string
	Page               int
	PageSize           int
	SortBy             string
	SortOrder          string
	FilterBy           string
	LibrarySlug        string
	Tags               []string
	TagMode            string // "all" or "any"
	Types              []string // filter by media types (any match)
	AccessibleLibraries []string // filter by accessible libraries for permission system
	ContentRatingLimit int // filter by content rating
}

// SearchMediasWithOptions performs a flexible media search using options
func SearchMediasWithOptions(opts SearchOptions) ([]Media, int64, error) {
	var mangas []Media
	if err := loadAllMedias(&mangas); err != nil {
		return nil, 0, err
	}

	// Filter by accessible libraries (permission system)
	if len(opts.AccessibleLibraries) > 0 {
		mangas = filterByAccessibleLibraries(mangas, opts.AccessibleLibraries)
	}

	// Filter by library
	if opts.LibrarySlug != "" {
		mangas = filterByLibrarySlug(mangas, opts.LibrarySlug)
	}

	// Filter by content rating
	mangas = filterByContentRating(mangas, opts.ContentRatingLimit)

	// Filter by tags if provided
	if len(opts.Tags) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			return nil, 0, err
		}

		if opts.TagMode == "any" {
			mangas = filterByAnyTag(mangas, opts.Tags, tagMap)
		} else {
			mangas = filterByAllTags(mangas, opts.Tags, tagMap)
		}
	}

	// Filter by types if provided (any match)
	if len(opts.Types) > 0 {
		typeSet := normalizeStringSet(opts.Types)
		filtered := make([]Media, 0, len(mangas))
		for _, m := range mangas {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(m.Type))]; ok {
				filtered = append(filtered, m)
			}
		}
		mangas = filtered
	}

	total := int64(len(mangas))

	// Apply text search filter
	if opts.Filter != "" {
		mangas = applyBigramSearch(opts.Filter, mangas)
		total = int64(len(mangas))
	}

	// Sort results
	key, ord := MediaSortConfig.NormalizeSort(opts.SortBy, opts.SortOrder)
	SortMedias(mangas, key, ord)

	// Paginate
	return paginateMedias(mangas, opts.Page, opts.PageSize), total, nil
}
func filterByAccessibleLibraries(mangas []Media, accessibleLibraries []string) []Media {
	if len(accessibleLibraries) == 0 {
		return []Media{} // No accessible libraries means no media
	}
	
	// Create a set for O(1) lookup
	librarySet := make(map[string]struct{}, len(accessibleLibraries))
	for _, slug := range accessibleLibraries {
		librarySet[slug] = struct{}{}
	}
	
	filtered := make([]Media, 0, len(mangas))
	for _, m := range mangas {
		if _, ok := librarySet[m.LibrarySlug]; ok {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByLibrarySlug filters mangas by library slug
func filterByLibrarySlug(mangas []Media, librarySlug string) []Media {
	filtered := make([]Media, 0, len(mangas))
	for _, m := range mangas {
		if m.LibrarySlug == librarySlug {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByAllTags keeps only mangas that have all selected tags
func filterByAllTags(mangas []Media, selectedTags []string, tagMap map[string][]string) []Media {
	selectedSet := normalizeTagSet(selectedTags)
	filtered := make([]Media, 0, len(mangas))
	
	for _, m := range mangas {
		if hasAllTags(tagMap[m.Slug], selectedSet) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByAnyTag keeps only mangas that have at least one of the selected tags
func filterByAnyTag(mangas []Media, selectedTags []string, tagMap map[string][]string) []Media {
	selectedSet := normalizeTagSet(selectedTags)
	filtered := make([]Media, 0, len(mangas))
	
	for _, m := range mangas {
		if hasAnyTag(tagMap[m.Slug], selectedSet) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// normalizeTagSet creates a set of normalized (trimmed, lowercase) tags
func normalizeTagSet(tags []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			set[t] = struct{}{}
		}
	}
	return set
}

// normalizeStringSet lowercases and trims a slice of strings into a set map
func normalizeStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			set[v] = struct{}{}
		}
	}
	return set
}

// hasAllTags checks if tags slice contains all required tags
func hasAllTags(tags []string, required map[string]struct{}) bool {
	if len(required) == 0 {
		return true
	}
	if len(tags) == 0 {
		return false
	}
	
	present := normalizeTagSet(tags)
	for t := range required {
		if _, ok := present[t]; !ok {
			return false
		}
	}
	return true
}

// hasAnyTag checks if tags slice contains at least one tag from the set
func hasAnyTag(tags []string, anySet map[string]struct{}) bool {
	if len(anySet) == 0 {
		return true
	}
	
	for _, t := range tags {
		lt := strings.TrimSpace(strings.ToLower(t))
		if lt == "" {
			continue
		}
		if _, ok := anySet[lt]; ok {
			return true
		}
	}
	return false
}

// filterByContentRating keeps only mangas that are allowed by content rating limit
func filterByContentRating(mangas []Media, contentRatingLimit int) []Media {
	if contentRatingLimit < 0 {
		return mangas // No filtering if limit is negative
	}
	filtered := make([]Media, 0, len(mangas))
	for _, m := range mangas {
		if IsContentRatingAllowed(m.ContentRating, contentRatingLimit) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// paginateMedias returns a paginated slice of mangas
func paginateMedias(mangas []Media, page, pageSize int) []Media {
	if pageSize <= 0 {
		return mangas
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if start > len(mangas) {
		return []Media{}
	}
	if end > len(mangas) {
		end = len(mangas)
	}
	return mangas[start:end]
}

// QueryParams holds parsed query parameters for media listings
type QueryParams struct {
	Page         int
	Sort         string
	Order        string
	Tags         []string
	TagMode      string
	Types        []string
	LibrarySlug  string
	SearchFilter string
}
