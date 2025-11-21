package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

// LightNovel represents the light_novel table schema
type LightNovel struct {
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
	Tags             []string  `json:"tags"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateLightNovel adds a new LightNovel to the database
func CreateLightNovel(lightNovel LightNovel) error {
	lightNovel.Slug = utils.Sluggify(lightNovel.Name)
	exists, err := LightNovelExists(lightNovel.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("light novel already exists")
	}

	timestamps := NewTimestamps()
	lightNovel.CreatedAt = timestamps.CreatedAt
	lightNovel.UpdatedAt = timestamps.UpdatedAt

	query := `
	INSERT INTO light_novels (slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	createdAt, updatedAt := timestamps.UnixTimestamps()
	_, err = db.Exec(query, lightNovel.Slug, lightNovel.Name, lightNovel.Author, lightNovel.Description, lightNovel.Year, lightNovel.OriginalLanguage, lightNovel.Type, lightNovel.Status, lightNovel.ContentRating, lightNovel.LibrarySlug, lightNovel.CoverArtURL, lightNovel.Path, createdAt, updatedAt)
	return err
}

// GetLightNovel retrieves a single LightNovel by slug
func GetLightNovel(slug string) (*LightNovel, error) {
	return getLightNovel(slug, true)
}

// GetLightNovelUnfiltered retrieves a single LightNovel by slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetLightNovelUnfiltered(slug string) (*LightNovel, error) {
	return getLightNovel(slug, false)
}

// getLightNovel is the internal implementation that optionally applies content rating filtering
func getLightNovel(slug string, applyContentFilter bool) (*LightNovel, error) {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at FROM light_novels WHERE slug = ?`

	row := db.QueryRow(query, slug)

	var lightNovel LightNovel
	var createdAt, updatedAt int64
	err := row.Scan(&lightNovel.Slug, &lightNovel.Name, &lightNovel.Author, &lightNovel.Description, &lightNovel.Year, &lightNovel.OriginalLanguage, &lightNovel.Type, &lightNovel.Status, &lightNovel.ContentRating, &lightNovel.LibrarySlug, &lightNovel.CoverArtURL, &lightNovel.Path, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No light novel found
		}
		return nil, err
	}

	lightNovel.CreatedAt = time.Unix(createdAt, 0)
	lightNovel.UpdatedAt = time.Unix(updatedAt, 0)

	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		cfg, err := GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for content rating check: %v", err)
			// On error, default to showing content
		} else if !IsContentRatingAllowed(lightNovel.ContentRating, cfg.ContentRatingLimit) {
			return nil, nil // Return nil to indicate light novel not found/accessible
		}
	}

	// Load tags
	lightNovel.Tags, err = GetTagsForLightNovel(lightNovel.Slug)
	if err != nil {
		log.Errorf("Failed to load tags for light novel %s: %v", lightNovel.Slug, err)
	}

	return &lightNovel, nil
}

// UpdateLightNovel updates an existing LightNovel in the database
func UpdateLightNovel(lightNovel LightNovel) error {
	timestamps := NewTimestamps()
	lightNovel.UpdatedAt = timestamps.UpdatedAt

	query := `
	UPDATE light_novels
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, type = ?, status = ?, content_rating = ?, library_slug = ?, cover_art_url = ?, path = ?, updated_at = ?
	WHERE slug = ?
	`

	updatedAt := timestamps.UpdatedAt.Unix()
	_, err := db.Exec(query, lightNovel.Name, lightNovel.Author, lightNovel.Description, lightNovel.Year, lightNovel.OriginalLanguage, lightNovel.Type, lightNovel.Status, lightNovel.ContentRating, lightNovel.LibrarySlug, lightNovel.CoverArtURL, lightNovel.Path, updatedAt, lightNovel.Slug)
	return err
}

// DeleteLightNovel removes a LightNovel from the database
func DeleteLightNovel(slug string) error {
	// Delete associated chapters first
	if err := DeleteChaptersByMangaSlug(slug); err != nil {
		return err
	}

	// Delete associated tags
	if err := DeleteTagsByLightNovelSlug(slug); err != nil {
		return err
	}

	query := `DELETE FROM light_novels WHERE slug = ?`
	_, err := db.Exec(query, slug)
	return err
}

// DeleteLightNovelsByLibrarySlug deletes all light novels and their chapters for a given library
func DeleteLightNovelsByLibrarySlug(librarySlug string) error {
	query := `SELECT slug FROM light_novels WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query light novels by librarySlug: %v", err)
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
		if err := DeleteLightNovel(slug); err != nil {
			log.Errorf("Failed to delete light novel with slug '%s': %s", slug, err.Error())
			return err
		}
	}

	return nil
}

// LightNovelExists checks if a LightNovel with the given slug exists
func LightNovelExists(slug string) (bool, error) {
	query := `SELECT COUNT(*) FROM light_novels WHERE slug = ?`
	row := db.QueryRow(query, slug)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetLightNovels retrieves all LightNovels with optional filtering and pagination
func GetLightNovels(limit, offset int, librarySlug, sortBy, sortOrder string, contentRatingLimit int) ([]LightNovel, error) {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at FROM light_novels`

	var args []interface{}
	var conditions []string

	if librarySlug != "" {
		conditions = append(conditions, "library_slug = ?")
		args = append(args, librarySlug)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Sorting
	if sortBy != "" {
		query += " ORDER BY " + sortBy
		if sortOrder == "desc" {
			query += " DESC"
		} else {
			query += " ASC"
		}
	}

	// Pagination
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lightNovels []LightNovel
	for rows.Next() {
		var lightNovel LightNovel
		var createdAt, updatedAt int64
		err := rows.Scan(&lightNovel.Slug, &lightNovel.Name, &lightNovel.Author, &lightNovel.Description, &lightNovel.Year, &lightNovel.OriginalLanguage, &lightNovel.Type, &lightNovel.Status, &lightNovel.ContentRating, &lightNovel.LibrarySlug, &lightNovel.CoverArtURL, &lightNovel.Path, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		lightNovel.CreatedAt = time.Unix(createdAt, 0)
		lightNovel.UpdatedAt = time.Unix(updatedAt, 0)

		// Filter based on content rating limit
		if IsContentRatingAllowed(lightNovel.ContentRating, contentRatingLimit) {
			// Load tags
			lightNovel.Tags, err = GetTagsForLightNovel(lightNovel.Slug)
			if err != nil {
				log.Errorf("Failed to load tags for light novel %s: %v", lightNovel.Slug, err)
			}
			lightNovels = append(lightNovels, lightNovel)
		}
	}

	return lightNovels, nil
}

// CountLightNovels returns the total number of LightNovels
func CountLightNovels(librarySlug string, contentRatingLimit int) (int, error) {
	query := `SELECT COUNT(*) FROM light_novels`

	var args []interface{}
	var conditions []string

	if librarySlug != "" {
		conditions = append(conditions, "library_slug = ?")
		args = append(args, librarySlug)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	row := db.QueryRow(query, args...)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}

	// Note: Content rating filtering is not applied here for simplicity
	// In a full implementation, you might want to count only accessible light novels
	return count, nil
}

// SearchLightNovels searches for LightNovels by name or author
func SearchLightNovels(query string, limit, offset int, librarySlug string, contentRatingLimit int) ([]LightNovel, error) {
	sqlQuery := `
		SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at
		FROM light_novels
		WHERE (name LIKE ? OR author LIKE ?)
	`

	var args []interface{}
	args = append(args, "%"+query+"%", "%"+query+"%")

	if librarySlug != "" {
		sqlQuery += " AND library_slug = ?"
		args = append(args, librarySlug)
	}

	sqlQuery += " ORDER BY name LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lightNovels []LightNovel
	for rows.Next() {
		var lightNovel LightNovel
		var createdAt, updatedAt int64
		err := rows.Scan(&lightNovel.Slug, &lightNovel.Name, &lightNovel.Author, &lightNovel.Description, &lightNovel.Year, &lightNovel.OriginalLanguage, &lightNovel.Type, &lightNovel.Status, &lightNovel.ContentRating, &lightNovel.LibrarySlug, &lightNovel.CoverArtURL, &lightNovel.Path, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		lightNovel.CreatedAt = time.Unix(createdAt, 0)
		lightNovel.UpdatedAt = time.Unix(updatedAt, 0)

		// Filter based on content rating limit
		if IsContentRatingAllowed(lightNovel.ContentRating, contentRatingLimit) {
			// Load tags
			lightNovel.Tags, err = GetTagsForLightNovel(lightNovel.Slug)
			if err != nil {
				log.Errorf("Failed to load tags for light novel %s: %v", lightNovel.Slug, err)
			}
			lightNovels = append(lightNovels, lightNovel)
		}
	}

	return lightNovels, nil
}

// GetTagsForLightNovel retrieves tags for a specific LightNovel
func GetTagsForLightNovel(slug string) ([]string, error) {
	query := `SELECT tag FROM light_novel_tags WHERE light_novel_slug = ? ORDER BY tag`
	rows, err := db.Query(query, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		err := rows.Scan(&tag)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// UpdateTagsForLightNovel updates the tags for a LightNovel
func UpdateTagsForLightNovel(slug string, tags []string) error {
	// Remove existing tags
	_, err := db.Exec(`DELETE FROM light_novel_tags WHERE light_novel_slug = ?`, slug)
	if err != nil {
		return err
	}

	// Insert new tags
	for _, tag := range tags {
		_, err = db.Exec(`INSERT INTO light_novel_tags (light_novel_slug, tag) VALUES (?, ?)`, slug, tag)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteTagsByLightNovelSlug deletes all tags associated with a light novel
func DeleteTagsByLightNovelSlug(slug string) error {
	query := `DELETE FROM light_novel_tags WHERE light_novel_slug = ?`
	_, err := db.Exec(query, slug)
	return err
}

// LightNovelSearchOptions defines parameters for light novel searches
type LightNovelSearchOptions struct {
	Filter             string
	Page               int
	PageSize           int
	SortBy             string
	SortOrder          string
	LibrarySlug        string
	AccessibleLibraries []string // filter by accessible libraries for permission system
}

// SearchLightNovelsWithOptions performs a flexible light novel search using options
func SearchLightNovelsWithOptions(opts LightNovelSearchOptions, contentRatingLimit int) ([]LightNovel, int64, error) {
	var lightNovels []LightNovel
	if err := loadAllLightNovels(&lightNovels); err != nil {
		return nil, 0, err
	}

	// Filter by accessible libraries (permission system)
	if len(opts.AccessibleLibraries) > 0 {
		lightNovels = filterLightNovelsByAccessibleLibraries(lightNovels, opts.AccessibleLibraries)
	}

	// Filter by library
	if opts.LibrarySlug != "" {
		lightNovels = filterLightNovelsByLibrarySlug(lightNovels, opts.LibrarySlug)
	}

	// Filter by content rating
	lightNovels = filterLightNovelsByContentRating(lightNovels, contentRatingLimit)

	// Filter by search term
	if opts.Filter != "" {
		lightNovels = filterLightNovelsBySearch(lightNovels, opts.Filter)
	}

	// Sort the results
	key, order := LightNovelSortConfig.NormalizeSort(opts.SortBy, opts.SortOrder)
	SortLightNovels(lightNovels, key, order)

	// Paginate
	totalCount := int64(len(lightNovels))
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start > len(lightNovels) {
		return []LightNovel{}, totalCount, nil
	}
	if end > len(lightNovels) {
		end = len(lightNovels)
	}

	return lightNovels[start:end], totalCount, nil
}

// Helper functions for filtering light novels

func filterLightNovelsByAccessibleLibraries(lightNovels []LightNovel, accessibleLibraries []string) []LightNovel {
	// If no accessible libraries specified, return all
	if len(accessibleLibraries) == 0 {
		return lightNovels
	}
	
	var filtered []LightNovel
	for _, ln := range lightNovels {
		for _, lib := range accessibleLibraries {
			if ln.LibrarySlug == lib {
				filtered = append(filtered, ln)
				break
			}
		}
	}
	return filtered
}

func filterLightNovelsByLibrarySlug(lightNovels []LightNovel, librarySlug string) []LightNovel {
	if librarySlug == "" {
		return lightNovels
	}
	var filtered []LightNovel
	for _, ln := range lightNovels {
		if ln.LibrarySlug == librarySlug {
			filtered = append(filtered, ln)
		}
	}
	return filtered
}

func filterLightNovelsByContentRating(lightNovels []LightNovel, contentRatingLimit int) []LightNovel {
	var filtered []LightNovel
	for _, ln := range lightNovels {
		if IsContentRatingAllowed(ln.ContentRating, contentRatingLimit) {
			filtered = append(filtered, ln)
		}
	}
	return filtered
}

func filterLightNovelsBySearch(lightNovels []LightNovel, filter string) []LightNovel {
	var filtered []LightNovel
	filterLower := strings.ToLower(filter)
	for _, ln := range lightNovels {
		if strings.Contains(strings.ToLower(ln.Name), filterLower) ||
		   strings.Contains(strings.ToLower(ln.Author), filterLower) ||
		   strings.Contains(strings.ToLower(ln.Description), filterLower) {
			filtered = append(filtered, ln)
		}
	}
	return filtered
}

// loadAllLightNovels loads all light novels from the database
func loadAllLightNovels(lightNovels *[]LightNovel) error {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at FROM light_novels`

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var ln LightNovel
		var createdAt, updatedAt int64
		if err := rows.Scan(&ln.Slug, &ln.Name, &ln.Author, &ln.Description, &ln.Year, &ln.OriginalLanguage, &ln.Type, &ln.Status, &ln.ContentRating, &ln.LibrarySlug, &ln.CoverArtURL, &ln.Path, &createdAt, &updatedAt); err != nil {
			return err
		}
		ln.CreatedAt = time.Unix(createdAt, 0)
		ln.UpdatedAt = time.Unix(updatedAt, 0)
		*lightNovels = append(*lightNovels, ln)
	}
	return nil
}