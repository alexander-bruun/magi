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
	"go.etcd.io/bbolt"
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

func CreateManga(manga Manga) error {
	manga.Slug = utils.Sluggify(manga.Name)
	exists, err := MangaExists(manga.Slug)
	if err != nil {
		return err
	}
	if !exists {
		timeNow := time.Now()
		manga.CreatedAt = timeNow
		manga.UpdatedAt = timeNow
		return create("mangas", manga.Slug, manga)
	} else {
		return errors.New("manga already exists")
	}
}

func GetManga(slug string) (*Manga, error) {
	var manga Manga
	err := get("mangas", slug, &manga)
	if err != nil {
		return nil, err
	}
	return &manga, nil
}

func UpdateManga(manga *Manga) error {
	manga.UpdatedAt = time.Now()
	return update("mangas", manga.Slug, manga)
}

func DeleteManga(slug string) error {
	err := delete("mangas", slug)
	if err != nil {
		return err
	}

	err = DeleteChaptersByMangaSlug(slug)
	if err != nil {
		return err
	}

	return nil
}

func SearchMangas(filter string, page int, pageSize int, sortBy string, sortOrder string, filterBy string, librarySlug string) ([]Manga, int64, error) {
	var dataList [][]byte
	if err := getAll("mangas", &dataList); err != nil {
		log.Fatalf("Failed to get all data: %v", err)
	}

	var mangas []Manga
	for _, data := range dataList {
		var manga Manga
		if err := json.Unmarshal(data, &manga); err != nil {
			return nil, 0, err
		}
		mangas = append(mangas, manga)
	}

	// Filter by librarySlug
	if librarySlug != "" {
		var filteredMangas []Manga
		for _, manga := range mangas {
			if manga.LibrarySlug == librarySlug {
				filteredMangas = append(filteredMangas, manga)
			}
		}
		mangas = filteredMangas
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
	start := (page - 1) * pageSize
	end := start + pageSize
	if start < len(mangas) {
		if end > len(mangas) {
			end = len(mangas)
		}
		return mangas[start:end], total, nil
	}
	return []Manga{}, total, nil
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

func MangaExists(slug string) (bool, error) {
	exists, err := exists("mangas", slug)
	if err == bbolt.ErrBucketNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists, nil
}

func MangaCount(filterBy, filter string) (int, error) {
	var dataList [][]byte
	if err := getAll("mangas", &dataList); err != nil {
		log.Fatalf("Failed to get all data: %v", err)
	}

	count := 0
	for _, data := range dataList {
		var manga Manga
		if err := json.Unmarshal(data, &manga); err != nil {
			return 0, err
		}
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

func DeleteMangasByLibrarySlug(librarySlug string) error {
	keys, err := getAllKeys("mangas")
	if err != nil {
		log.Errorf("Failed to get all data: %v", err)
		return err
	}

	for _, key := range keys {
		var manga Manga
		err := get("mangas", key, &manga)
		if err != nil {
			log.Errorf("Failed to get key: %s", key)
			return err
		}

		if manga.LibrarySlug == librarySlug {
			err := DeleteChaptersByMangaSlug(manga.Slug)
			if err != nil {
				log.Errorf("Failed to delete chapters for manga slug '%s': %s", manga.Slug, err.Error())
				return err
			}
			log.Infof("Deleted chapters for manga: '%s'", manga.Slug)

			err = delete("mangas", manga.Slug)
			if err != nil {
				log.Errorf("Failed to delete manga with slug '%s': %s", manga.Slug, err.Error())
				return err
			}
			log.Infof("Deleted manga with slug '%s'", manga.Slug)
		}
	}

	return nil
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
		// Handle unknown sortBy or no sorting
	}
}
