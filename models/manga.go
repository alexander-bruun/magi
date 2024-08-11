package models

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"go.etcd.io/bbolt"
)

type Manga struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Author           string `json:"author"`
	Description      string `json:"description"`
	Year             int    `json:"year"`
	OriginalLanguage string `json:"original_language"`
	Status           string `json:"status"`
	ContentRating    string `json:"content_rating"`
	LibrarySlug      string `json:"library_slug"`
	CoverArtURL      string `json:"cover_art_url"`
	Path             string `json:"path"`
}

func CreateManga(manga Manga) error {
	manga.Slug = utils.Sluggify(manga.Name)
	exists, err := MangaExists(manga.Slug)
	if err != nil {
		return err
	}
	if !exists {
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
	return update("mangas", manga.Slug, manga)
}

func DeleteManga(slug string) error {
	return delete("mangas", slug)
}

func SearchMangas(filter string, page int, pageSize int, sortBy string, sortOrder string, filterBy string, librarySlug string) ([]Manga, int64, error) {
	dataList, err := getAll("mangas")
	if err != nil {
		return nil, 0, err
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
	// Implement sorting logic here

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
	var manga Manga
	err := get("mangas", slug, &manga)
	if err == bbolt.ErrBucketNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func MangaCount(filterBy, filter string) (int, error) {
	dataList, err := getAll("mangas")
	if err != nil {
		return 0, err
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
	dataList, err := getAll("mangas")
	if err != nil {
		return err
	}

	count := 0
	for _, data := range dataList {
		var manga Manga
		if err := json.Unmarshal(data, &manga); err != nil {
			return err
		}
		if manga.LibrarySlug == librarySlug {
			err := delete("mangas", manga.Slug)
			if err != nil {
				return err
			}
			count++
		}
	}

	if count == 0 {
		return errors.New("no mangas found with the specified library slug")
	}

	return nil
}
