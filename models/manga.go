package models

import (
	"database/sql"
	"errors"
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
	Status           string    `json:"status"`
	ContentRating    string    `json:"content_rating"`
	LibrarySlug      string    `json:"library_slug"`
	CoverArtURL      string    `json:"cover_art_url"`
	Path             string    `json:"path"`
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

	now := time.Now()
	manga.CreatedAt = now
	manga.UpdatedAt = now

	query := `
	INSERT INTO mangas (slug, name, author, description, year, original_language, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, manga.Slug, manga.Name, manga.Author, manga.Description, manga.Year, manga.OriginalLanguage, manga.Status, manga.ContentRating, manga.LibrarySlug, manga.CoverArtURL, manga.Path, manga.CreatedAt.Unix(), manga.UpdatedAt.Unix())
	if err != nil {
		return err
	}

	return nil
}

// GetManga retrieves a single Manga by slug
func GetManga(slug string) (*Manga, error) {
	query := `SELECT slug, name, author, description, year, original_language, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at FROM mangas WHERE slug = ?`

	row := db.QueryRow(query, slug)

	var manga Manga
	var createdAt, updatedAt int64
	err := row.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No manga found
		}
		return nil, err
	}

	manga.CreatedAt = time.Unix(createdAt, 0)
	manga.UpdatedAt = time.Unix(updatedAt, 0)
	return &manga, nil
}

// UpdateManga modifies an existing Manga
func UpdateManga(manga *Manga) error {
	manga.UpdatedAt = time.Now()

	query := `
	UPDATE mangas
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, status = ?, content_rating = ?, library_slug = ?, cover_art_url = ?, path = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, manga.Name, manga.Author, manga.Description, manga.Year, manga.OriginalLanguage, manga.Status, manga.ContentRating, manga.LibrarySlug, manga.CoverArtURL, manga.Path, manga.UpdatedAt.Unix(), manga.Slug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteManga removes a Manga and its associated chapters
func DeleteManga(slug string) error {
	query := `DELETE FROM mangas WHERE slug = ?`

	_, err := db.Exec(query, slug)
	if err != nil {
		return err
	}

	return DeleteChaptersByMangaSlug(slug)
}

// SearchMangas filters, sorts, and paginates mangas based on provided criteria
func SearchMangas(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string) ([]Manga, int64, error) {
	var mangas []Manga
	if err := loadAllMangas(&mangas); err != nil {
		return nil, 0, err
	}

	// Filter by librarySlug
	if librarySlug != "" {
		mangas = filterByLibrarySlug(mangas, librarySlug)
	}

	total := int64(len(mangas))

	// Apply bigram search if filter is provided
	if filter != "" {
		mangas = applyBigramSearch(filter, mangas)
		total = int64(len(mangas))
	}

	// Sort mangas based on sortBy and sortOrder
	sortMangas(mangas, sortBy, sortOrder)

	// Apply pagination
	return paginateMangas(mangas, page, pageSize), total, nil
}

// MangaExists checks if a Manga exists by slug
func MangaExists(slug string) (bool, error) {
	query := `SELECT 1 FROM mangas WHERE slug = ?`

	row := db.QueryRow(query, slug)
	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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

// Helper functions

func loadAllMangas(mangas *[]Manga) error {
	query := `SELECT slug, name, author, description, year, original_language, status, content_rating, library_slug, cover_art_url, path, created_at, updated_at FROM mangas`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to get all mangas: %v", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var manga Manga
		var createdAt, updatedAt int64
		if err := rows.Scan(&manga.Slug, &manga.Name, &manga.Author, &manga.Description, &manga.Year, &manga.OriginalLanguage, &manga.Status, &manga.ContentRating, &manga.LibrarySlug, &manga.CoverArtURL, &manga.Path, &createdAt, &updatedAt); err != nil {
			return err
		}
		manga.CreatedAt = time.Unix(createdAt, 0)
		manga.UpdatedAt = time.Unix(updatedAt, 0)
		*mangas = append(*mangas, manga)
	}
	return nil
}

func filterByLibrarySlug(mangas []Manga, librarySlug string) []Manga {
	var filteredMangas []Manga
	for _, manga := range mangas {
		if manga.LibrarySlug == librarySlug {
			filteredMangas = append(filteredMangas, manga)
		}
	}
	return filteredMangas
}

func applyBigramSearch(filter string, mangas []Manga) []Manga {
	var mangaNames []string
	nameToManga := make(map[string]Manga)

	for _, manga := range mangas {
		mangaNames = append(mangaNames, manga.Name)
		nameToManga[manga.Name] = manga
	}

	matchingNames := utils.BigramSearch(filter, mangaNames)

	var filteredMangas []Manga
	for _, name := range matchingNames {
		if manga, ok := nameToManga[name]; ok {
			filteredMangas = append(filteredMangas, manga)
		}
	}

	return filteredMangas
}

func paginateMangas(mangas []Manga, page, pageSize int) []Manga {
	start := (page - 1) * pageSize
	end := start + pageSize
	if start < len(mangas) {
		if end > len(mangas) {
			end = len(mangas)
		}
		return mangas[start:end]
	}
	return []Manga{}
}

func sortMangas(mangas []Manga, sortBy, sortOrder string) {
	switch sortBy {
	case "created_at":
		if sortOrder == "asc" {
			sort.Slice(mangas, func(i, j int) bool {
				return mangas[i].CreatedAt.Before(mangas[j].CreatedAt)
			})
		} else {
			sort.Slice(mangas, func(i, j int) bool {
				return mangas[i].CreatedAt.After(mangas[j].CreatedAt)
			})
		}
	case "updated_at":
		if sortOrder == "asc" {
			sort.Slice(mangas, func(i, j int) bool {
				return mangas[i].UpdatedAt.Before(mangas[j].UpdatedAt)
			})
		} else {
			sort.Slice(mangas, func(i, j int) bool {
				return mangas[i].UpdatedAt.After(mangas[j].UpdatedAt)
			})
		}
	default:
		// No sorting applied
	}
}
