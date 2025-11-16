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

// Manga represents the manga table schema
type Manga struct {
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
	Tags             []string  `json:"tags"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateManga adds a new Manga to the database
func CreateManga(manga Manga) error {
	manga.Slug = utils.Sluggify(manga.Name)
	exists, err := MangaExists(manga.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("manga already exists")
	}

	timestamps := NewTimestamps()
	manga.CreatedAt = timestamps.CreatedAt
	manga.UpdatedAt = timestamps.UpdatedAt

	query := `
	INSERT INTO mangas (slug, name, author, description, year, original_language, manga_type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	createdAt, updatedAt := timestamps.UnixTimestamps()
	_, err = db.Exec(query, manga.Slug, manga.Name, manga.Author, manga.Description, manga.Year, manga.OriginalLanguage, manga.Type, manga.Status, manga.ContentRating, manga.LibrarySlug, manga.CoverArtURL, manga.Path, manga.FileCount, createdAt, updatedAt)
	return err
}

// GetManga retrieves a single Manga by slug
func GetManga(slug string) (*Manga, error) {
	return getManga(slug, true)
}

// GetMangaUnfiltered retrieves a single Manga by slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetMangaUnfiltered(slug string) (*Manga, error) {
	return getManga(slug, false)
}

// getManga is the internal implementation that optionally applies content rating filtering
func getManga(slug string, applyContentFilter bool) (*Manga, error) {
	query := `SELECT slug, name, author, description, year, original_language, manga_type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM mangas WHERE slug = ?`

	row := db.QueryRow(query, slug)

	var manga Manga
	var createdAt, updatedAt int64
	err := row.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Type, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &manga.FileCount, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No manga found
		}
		return nil, err
	}

	manga.CreatedAt = time.Unix(createdAt, 0)
	manga.UpdatedAt = time.Unix(updatedAt, 0)
	
	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		cfg, err := GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for content rating check: %v", err)
			// On error, default to showing content
		} else if !IsContentRatingAllowed(manga.ContentRating, cfg.ContentRatingLimit) {
			return nil, nil // Return nil to indicate manga not found/accessible
		}
	}
	
	// Load tags for this manga if any
	if tags, err := GetTagsForManga(manga.Slug); err == nil {
		manga.Tags = tags
	}
	return &manga, nil
}

// UpdateManga modifies an existing Manga
func UpdateManga(manga *Manga) error {
	manga.UpdatedAt = time.Now()

	query := `
	UPDATE mangas
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, manga_type = ?, status = ?, content_rating = ?, library_slug = ?, cover_art_url = ?, path = ?, file_count = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, manga.Name, manga.Author, manga.Description, manga.Year, manga.OriginalLanguage, manga.Type, manga.Status, manga.ContentRating, manga.LibrarySlug, manga.CoverArtURL, manga.Path, manga.FileCount, manga.UpdatedAt.Unix(), manga.Slug)
	if err != nil {
		return err
	}

	return nil
}

// UpdateMangaMetadata updates manga metadata fields while preserving the original created_at timestamp.
// This is useful for refresh operations where we want to preserve the original creation date.
func UpdateMangaMetadata(manga *Manga) error {
	manga.UpdatedAt = time.Now()

	query := `
	UPDATE mangas
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, manga_type = ?, status = ?, content_rating = ?, cover_art_url = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, manga.Name, manga.Author, manga.Description, manga.Year, manga.OriginalLanguage, manga.Type, manga.Status, manga.ContentRating, manga.CoverArtURL, manga.UpdatedAt.Unix(), manga.Slug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteManga removes a Manga and its associated chapters
func DeleteManga(slug string) error {
	// Delete associated chapters first
	if err := DeleteChaptersByMangaSlug(slug); err != nil {
		return err
	}

	// Delete associated tags
	if err := DeleteTagsByMangaSlug(slug); err != nil {
		return err
	}

	return DeleteRecord(`DELETE FROM mangas WHERE slug = ?`, slug)
}

// SearchMangas filters, sorts, and paginates mangas based on provided criteria
func SearchMangas(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string) ([]Manga, int64, error) {
	return SearchMangasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
	})
}

// SearchMangasWithTags extends SearchMangas to filter by selected tags (all must match)
func SearchMangasWithTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Manga, int64, error) {
	return SearchMangasWithOptions(SearchOptions{
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

// SearchMangasWithAnyTags filters mangas to those that have at least one of the selected tags
func SearchMangasWithAnyTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Manga, int64, error) {
	return SearchMangasWithOptions(SearchOptions{
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

// MangaExists checks if a Manga exists by slug
func MangaExists(slug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM mangas WHERE slug = ?`, slug)
}

// MangaCount counts the number of mangas based on filter criteria
func MangaCount(filterBy, filter string) (int, error) {
	var mangas []Manga
	if err := loadAllMangas(&mangas); err != nil {
		return 0, err
	}

	count := 0
	for _, manga := range mangas {
		if filterBy != "" && filter != "" {
			value := reflect.ValueOf(manga).FieldByName(filterBy).String()
			if strings.Contains(strings.ToLower(value), strings.ToLower(filter)) {
				count++
			}
		} else {
			count++
		}
	}
	return count, nil
}

// DeleteMangasByLibrarySlug removes all mangas associated with a specific library
func DeleteMangasByLibrarySlug(librarySlug string) error {
	query := `SELECT slug FROM mangas WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query mangas by librarySlug: %v", err)
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
		if err := DeleteManga(slug); err != nil {
			log.Errorf("Failed to delete manga with slug '%s': %s", slug, err.Error())
			return err
		}
	}

	return nil
}

// GetMangasByLibrarySlug returns all mangas that belong to a specific library
func GetMangasByLibrarySlug(librarySlug string) ([]Manga, error) {
	var mangas []Manga
	query := `SELECT slug, name, author, description, year, original_language, manga_type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM mangas WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query mangas by librarySlug: %v", err)
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
		var manga Manga
		var createdAt, updatedAt int64
		if err := rows.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Type, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &manga.FileCount, &createdAt, &updatedAt); err != nil {
			log.Errorf("Failed to scan manga row: %v", err)
			return nil, err
		}
		manga.CreatedAt = time.Unix(createdAt, 0)
		manga.UpdatedAt = time.Unix(updatedAt, 0)
		
		// Filter based on content rating limit
		if IsContentRatingAllowed(manga.ContentRating, cfg.ContentRatingLimit) {
			mangas = append(mangas, manga)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mangas, nil
}

// GetTopMangas returns the top mangas ordered by vote score (descending).
// It joins the mangas table with aggregated votes and respects content rating limits.
func GetTopMangas(limit int) ([]Manga, error) {
	query := `
	SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.manga_type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count, m.created_at, m.updated_at
	FROM mangas m
	LEFT JOIN (
		SELECT manga_slug, COALESCE(SUM(value),0) as score
		FROM votes
		GROUP BY manga_slug
	) v ON v.manga_slug = m.slug
	ORDER BY v.score DESC
	LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cfg, err := GetAppConfig()
	if err != nil {
		// If we can't get config, default to showing all content
		cfg.ContentRatingLimit = 3
	}

	var mangas []Manga
	for rows.Next() {
		var manga Manga
		var createdAt, updatedAt int64
		if err := rows.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Type, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &manga.FileCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		manga.CreatedAt = time.Unix(createdAt, 0)
		manga.UpdatedAt = time.Unix(updatedAt, 0)

		if IsContentRatingAllowed(manga.ContentRating, cfg.ContentRatingLimit) {
			mangas = append(mangas, manga)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mangas, nil
}

// Helper functions

func loadAllMangas(mangas *[]Manga) error {
	query := `SELECT slug, name, author, description, year, original_language, manga_type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM mangas`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to get all mangas: %v", err)
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
		var manga Manga
		var createdAt, updatedAt int64
		if err := rows.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Type, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &manga.FileCount, &createdAt, &updatedAt); err != nil {
			return err
		}
		manga.CreatedAt = time.Unix(createdAt, 0)
		manga.UpdatedAt = time.Unix(updatedAt, 0)
		
		// Filter based on content rating limit
		if IsContentRatingAllowed(manga.ContentRating, cfg.ContentRatingLimit) {
			*mangas = append(*mangas, manga)
		}
	}
	return nil
}

// GetAllMangaTypes returns all distinct manga_type values (lowercased) sorted ascending
func GetAllMangaTypes() ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT LOWER(TRIM(manga_type)) FROM mangas WHERE manga_type IS NOT NULL AND TRIM(manga_type) <> ''`)
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

func applyBigramSearch(filter string, mangas []Manga) []Manga {
	var mangaNames []string
	nameToManga := make(map[string]Manga, len(mangas))

	for _, manga := range mangas {
		mangaNames = append(mangaNames, manga.Name)
		nameToManga[manga.Name] = manga
	}

	matchingNames := utils.BigramSearch(filter, mangaNames)

	filteredMangas := make([]Manga, 0, len(matchingNames))
	for _, name := range matchingNames {
		if manga, ok := nameToManga[name]; ok {
			filteredMangas = append(filteredMangas, manga)
		}
	}

	return filteredMangas
}

// sortMangas moved to sorting.go (exported as SortMangas) for reuse across account pages.

// Vote represents a user's vote on a manga
type Vote struct {
	ID           int64
	UserUsername string
	MangaSlug    string
	Value        int // 1 for upvote, -1 for downvote
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetMangaVotes returns the aggregated score and counts for a manga
func GetMangaVotes(mangaSlug string) (score int, upvotes int, downvotes int, err error) {
	// Use COALESCE so aggregates return 0 instead of NULL when there are no rows
	query := `SELECT COALESCE(SUM(value),0) as score, COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) as upvotes, COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) as downvotes FROM votes WHERE manga_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	if err := row.Scan(&score, &upvotes, &downvotes); err != nil {
		return 0, 0, 0, err
	}
	return score, upvotes, downvotes, nil
}

// GetUserVoteForManga returns the vote value (1, -1) for a user on a manga. If none, returns 0.
func GetUserVoteForManga(username, mangaSlug string) (int, error) {
	query := `SELECT value FROM votes WHERE user_username = ? AND manga_slug = ?`
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

// SetVote inserts or updates a user's vote for a manga. value must be 1 or -1.
func SetVote(username, mangaSlug string, value int) error {
	if value != 1 && value != -1 {
		return errors.New("invalid vote value")
	}
	now := time.Now().Unix()
	// Try update first
	res, err := db.Exec(`UPDATE votes SET value = ?, updated_at = ? WHERE user_username = ? AND manga_slug = ?`, value, now, username, mangaSlug)
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
	_, err = db.Exec(`INSERT INTO votes (user_username, manga_slug, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, username, mangaSlug, value, now, now)
	return err
}

// RemoveVote deletes a user's vote for a manga
func RemoveVote(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM votes WHERE user_username = ? AND manga_slug = ?`, username, mangaSlug)
	return err
}

// Favorite represents a user's favorite manga
type Favorite struct {
	ID           int64
	UserUsername string
	MangaSlug    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SetFavorite inserts a favorite relationship for a user and manga.
func SetFavorite(username, mangaSlug string) error {
	now := time.Now().Unix()
	// Try update first (in case row exists) - this keeps updated_at current
	res, err := db.Exec(`UPDATE favorites SET updated_at = ? WHERE user_username = ? AND manga_slug = ?`, now, username, mangaSlug)
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
	_, err = db.Exec(`INSERT INTO favorites (user_username, manga_slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, mangaSlug, now, now)
	return err
}

// RemoveFavorite deletes a user's favorite for a manga
func RemoveFavorite(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM favorites WHERE user_username = ? AND manga_slug = ?`, username, mangaSlug)
	return err
}

// IsFavoriteForUser returns true if the user has favorited the manga
func IsFavoriteForUser(username, mangaSlug string) (bool, error) {
	query := `SELECT 1 FROM favorites WHERE user_username = ? AND manga_slug = ?`
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

// GetFavoritesCount returns the number of users who favorited the manga
func GetFavoritesCount(mangaSlug string) (int, error) {
	query := `SELECT COUNT(*) FROM favorites WHERE manga_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetFavoritesForUser returns manga slugs favorited by the user ordered by most recent update
func GetFavoritesForUser(username string) ([]string, error) {
	query := `SELECT manga_slug FROM favorites WHERE user_username = ? ORDER BY updated_at DESC`
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

// GetReadingMangasForUser returns distinct manga slugs that the user has reading state records for,
// ordered by most recent activity.
func GetReadingMangasForUser(username string) ([]string, error) {
	query := `SELECT DISTINCT manga_slug FROM reading_states WHERE user_name = ? ORDER BY created_at DESC`
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

// GetUpvotedMangasForUser returns manga slugs the user has upvoted (value = 1), ordered by most recent vote
func GetUpvotedMangasForUser(username string) ([]string, error) {
	query := `SELECT manga_slug FROM votes WHERE user_username = ? AND value = 1 ORDER BY updated_at DESC`
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

// GetDownvotedMangasForUser returns manga slugs the user has downvoted (value = -1), ordered by most recent vote
func GetDownvotedMangasForUser(username string) ([]string, error) {
	query := `SELECT manga_slug FROM votes WHERE user_username = ? AND value = -1 ORDER BY updated_at DESC`
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

// UserMangaListOptions defines parameters for user-specific manga list queries (favorites, reading, etc.)
type UserMangaListOptions struct {
	Username     string
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
	Tags         []string
	TagMode      string // "all" or "any"
	SearchFilter string
}

// GetUserFavoritesWithOptions fetches, filters, sorts, and paginates a user's favorite mangas
func GetUserFavoritesWithOptions(opts UserMangaListOptions) ([]Manga, int, error) {
	slugs, err := GetFavoritesForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMangaList(slugs, opts)
}

// GetUserReadingWithOptions fetches, filters, sorts, and paginates a user's reading list
func GetUserReadingWithOptions(opts UserMangaListOptions) ([]Manga, int, error) {
	slugs, err := GetReadingMangasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMangaList(slugs, opts)
}

// GetUserUpvotedWithOptions fetches, filters, sorts, and paginates a user's upvoted mangas
func GetUserUpvotedWithOptions(opts UserMangaListOptions) ([]Manga, int, error) {
	slugs, err := GetUpvotedMangasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMangaList(slugs, opts)
}

// GetUserDownvotedWithOptions fetches, filters, sorts, and paginates a user's downvoted mangas
func GetUserDownvotedWithOptions(opts UserMangaListOptions) ([]Manga, int, error) {
	slugs, err := GetDownvotedMangasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMangaList(slugs, opts)
}

// processUserMangaList handles the common logic for filtering, sorting, and paginating user manga lists
func processUserMangaList(slugs []string, opts UserMangaListOptions) ([]Manga, int, error) {
	// Load all mangas from slugs
	var allMangas []Manga
	for _, slug := range slugs {
		if m, err := GetManga(slug); err == nil && m != nil {
			allMangas = append(allMangas, *m)
		}
	}

	// Filter by tags if specified
	if len(opts.Tags) > 0 {
		allMangas = FilterMangasByTags(allMangas, opts.Tags, opts.TagMode)
	}

	// Filter by search term if specified
	if opts.SearchFilter != "" {
		allMangas = FilterMangasBySearch(allMangas, opts.SearchFilter)
	}

	// Sort mangas
	SortMangas(allMangas, opts.SortBy, opts.SortOrder)

	// Calculate total before pagination
	total := len(allMangas)

	// Paginate
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start > len(allMangas) {
		start = len(allMangas)
	}
	if end > len(allMangas) {
		end = len(allMangas)
	}

	return allMangas[start:end], total, nil
}

// FilterMangasByTags filters a slice of mangas by selected tags
// tagMode can be "all" (all tags must match) or "any" (at least one tag must match)
func FilterMangasByTags(mangas []Manga, selectedTags []string, tagMode string) []Manga {
	if len(selectedTags) == 0 {
		return mangas
	}

	var filtered []Manga
	for _, manga := range mangas {
		mangaTags, err := GetTagsForManga(manga.Slug)
		if err != nil {
			continue
		}

		if tagMode == "any" {
			// At least one selected tag must be in manga's tags
			for _, selTag := range selectedTags {
				for _, mTag := range mangaTags {
					if strings.EqualFold(selTag, mTag) {
						filtered = append(filtered, manga)
						goto nextManga
					}
				}
			}
		} else {
			// All selected tags must be in manga's tags
			matchCount := 0
			for _, selTag := range selectedTags {
				for _, mTag := range mangaTags {
					if strings.EqualFold(selTag, mTag) {
						matchCount++
						break
					}
				}
			}
			if matchCount == len(selectedTags) {
				filtered = append(filtered, manga)
			}
		}
	nextManga:
	}
	return filtered
}

// FilterMangasBySearch filters a slice of mangas by search term using very lenient fuzzy matching
func FilterMangasBySearch(mangas []Manga, searchTerm string) []Manga {
	if searchTerm == "" {
		return mangas
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
		return mangas
	}

	var filtered []Manga
	for _, manga := range mangas {
		// Normalize manga name
		normalizedName := normalize(manga.Name)

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
			filtered = append(filtered, manga)
		}
	}
	return filtered
}

// GetMangaAndChapters retrieves a manga and its chapters in one call
func GetMangaAndChapters(mangaSlug string) (*Manga, []Chapter, error) {
	manga, err := GetManga(mangaSlug)
	if err != nil {
		return nil, nil, err
	}
	if manga == nil {
		return nil, nil, fmt.Errorf("manga not found or access restricted")
	}

	chapters, err := GetChapters(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	return manga, chapters, nil
}

// GetChapterImages generates URLs for all images in a chapter
func GetChapterImages(manga *Manga, chapter *Chapter) ([]string, error) {
	// Determine the actual chapter file path
	// For single-file manga (cbz/cbr), manga.Path is the file itself
	// For directory-based manga, we need to join path and chapter file
	chapterFilePath := manga.Path
	if fileInfo, err := os.Stat(manga.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(manga.Path, chapter.File)
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
		images[i] = fmt.Sprintf("/api/comic?manga=%s&chapter=%s&page=%d", manga.Slug, chapter.Slug, i+1)
	}

	return images, nil
}

// GetFirstAndLastChapterSlugs returns the first and last chapter slugs for a manga
func GetFirstAndLastChapterSlugs(chapters []Chapter) (firstSlug, lastSlug string) {
	if len(chapters) > 0 {
		firstSlug = chapters[0].Slug
		lastSlug = chapters[len(chapters)-1].Slug
	}
	return firstSlug, lastSlug
}
