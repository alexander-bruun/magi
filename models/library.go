package models

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
	"go.etcd.io/bbolt"
)

type Library struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Cron        string   `json:"cron"`
	Folders     []string `json:"folders"`
	CreatedAt   int64    `json:"created_at"` // Unix timestamp
	UpdatedAt   int64    `json:"updated_at"` // Unix timestamp
}

// GetFolderNames returns a comma-separated string of folder names
func (l *Library) GetFolderNames() string {
	return strings.Join(l.Folders, ", ")
}

// Validate checks if the Library has valid values
func (l *Library) Validate() error {
	if l.Name == "" {
		return errors.New("library name cannot be empty")
	}
	if l.Description == "" {
		return errors.New("library description cannot be empty")
	}
	if l.Cron == "" {
		return errors.New("library cron cannot be empty")
	}
	l.Slug = utils.Sluggify(l.Name)
	return nil
}

// CreateLibrary adds a new Library to the database
func CreateLibrary(library Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	exists, err := LibraryExists(library.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("library already exists")
	}

	// Set CreatedAt and UpdatedAt fields to current time
	now := time.Now().Unix()
	library.CreatedAt = now
	library.UpdatedAt = now

	if err := create("libraries", library.Slug, library); err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_created", Payload: library})
	return nil
}

// GetLibraries retrieves all Libraries from the database
func GetLibraries() ([]Library, error) {
	var dataList [][]byte
	if err := getAll("libraries", &dataList); err != nil {
		log.Errorf("Failed to get all libraries: %v", err)
		return nil, err
	}

	var libraries []Library
	for _, data := range dataList {
		var library Library
		if err := json.Unmarshal(data, &library); err != nil {
			log.Errorf("Failed to unmarshal library data: %v", err)
			continue
		}
		libraries = append(libraries, library)
	}
	return libraries, nil
}

// GetLibrary retrieves a single Library by slug
func GetLibrary(slug string) (*Library, error) {
	var library Library
	if err := get("libraries", slug, &library); err != nil {
		return nil, err
	}
	return &library, nil
}

// UpdateLibrary modifies an existing Library
func UpdateLibrary(library *Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	library.UpdatedAt = time.Now().Unix() // Update the timestamp

	if err := update("libraries", library.Slug, library); err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_updated", Payload: *library})
	return nil
}

// DeleteLibrary removes a Library and its associated mangas
func DeleteLibrary(slug string) error {
	library, err := GetLibrary(slug)
	if err != nil {
		return err
	}

	if err := delete("libraries", slug); err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_deleted", Payload: *library})
	if err := DeleteMangasByLibrarySlug(slug); err != nil {
		return err
	}
	return nil
}

// SearchLibraries finds Libraries matching the keyword and applies pagination and sorting
func SearchLibraries(keyword string, page, pageSize int, sortBy, sortOrder string) ([]Library, int64, error) {
	libraries, err := GetLibraries()
	if err != nil {
		return nil, 0, err
	}

	if keyword != "" {
		libraries = filterLibrariesByKeyword(libraries, keyword)
	}

	// Apply sorting
	sortLibraries(libraries, sortBy, sortOrder)

	total := int64(len(libraries))
	return paginateLibraries(libraries, page, pageSize), total, nil
}

// LibraryExists checks if a Library exists by slug
func LibraryExists(slug string) (bool, error) {
	var library Library
	err := get("libraries", slug, &library)
	if err == bbolt.ErrBucketNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// filterLibrariesByKeyword filters the libraries based on the keyword
func filterLibrariesByKeyword(libraries []Library, keyword string) []Library {
	var libraryNames []string
	nameToLibrary := make(map[string]Library)

	for _, lib := range libraries {
		libraryNames = append(libraryNames, lib.Name)
		nameToLibrary[lib.Name] = lib
	}

	matchingNames := utils.BigramSearch(keyword, libraryNames)

	var filteredLibraries []Library
	for _, name := range matchingNames {
		if lib, ok := nameToLibrary[name]; ok {
			filteredLibraries = append(filteredLibraries, lib)
		}
	}
	return filteredLibraries
}

// paginateLibraries applies pagination to the libraries slice
func paginateLibraries(libraries []Library, page, pageSize int) []Library {
	start := (page - 1) * pageSize
	end := start + pageSize
	if start < len(libraries) {
		if end > len(libraries) {
			end = len(libraries)
		}
		return libraries[start:end]
	}
	return []Library{}
}

// sortLibraries sorts libraries based on the given sortBy and sortOrder
func sortLibraries(libraries []Library, sortBy, sortOrder string) {
	switch sortBy {
	case "name":
		if sortOrder == "asc" {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].Name < libraries[j].Name
			})
		} else {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].Name > libraries[j].Name
			})
		}
	case "created_at":
		if sortOrder == "asc" {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].CreatedAt < libraries[j].CreatedAt
			})
		} else {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].CreatedAt > libraries[j].CreatedAt
			})
		}
	case "updated_at":
		if sortOrder == "asc" {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].UpdatedAt < libraries[j].UpdatedAt
			})
		} else {
			sort.Slice(libraries, func(i, j int) bool {
				return libraries[i].UpdatedAt > libraries[j].UpdatedAt
			})
		}
	default:
		// Default or no sorting
	}
}
