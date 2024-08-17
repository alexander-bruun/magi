package models

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

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
	return create("mangas", manga.Slug, manga)
}

// GetManga retrieves a single Manga by slug
func GetManga(slug string) (*Manga, error) {
	var manga Manga
	if err := get("mangas", slug, &manga); err != nil {
		return nil, err
	}
	return &manga, nil
}

// UpdateManga modifies an existing Manga
func UpdateManga(manga *Manga) error {
	manga.UpdatedAt = time.Now()
	return update("mangas", manga.Slug, manga)
}

// DeleteManga removes a Manga and its associated chapters
func DeleteManga(slug string) error {
	if err := delete("mangas", slug); err != nil {
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
	return exists("mangas", slug)
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
	keys, err := getAllKeys("mangas")
	if err != nil {
		log.Errorf("Failed to get all keys: %v", err)
		return err
	}

	for _, key := range keys {
		var manga Manga
		if err := get("mangas", key, &manga); err != nil {
			log.Errorf("Failed to get manga with key: %s", key)
			return err
		}

		if manga.LibrarySlug == librarySlug {
			if err := DeleteChaptersByMangaSlug(manga.Slug); err != nil {
				log.Errorf("Failed to delete chapters for manga slug '%s': %s", manga.Slug, err.Error())
				return err
			}
			log.Infof("Deleted chapters for manga: '%s'", manga.Slug)

			if err := delete("mangas", manga.Slug); err != nil {
				log.Errorf("Failed to delete manga with slug '%s': %s", manga.Slug, err.Error())
				return err
			}
			log.Infof("Deleted manga with slug '%s'", manga.Slug)
		}
	}

	return nil
}

// Helper functions

func loadAllMangas(mangas *[]Manga) error {
	var dataList [][]byte
	if err := getAll("mangas", &dataList); err != nil {
		log.Fatalf("Failed to get all data: %v", err)
		return err
	}

	for _, data := range dataList {
		var manga Manga
		if err := json.Unmarshal(data, &manga); err != nil {
			return err
		}
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
