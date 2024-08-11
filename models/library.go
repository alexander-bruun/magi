package models

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"go.etcd.io/bbolt"
)

type Library struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Cron        string   `json:"cron"`
	Folders     []string `json:"folders"`
}

func (l *Library) GetFolderNames() string {
	return strings.Join(l.Folders, ", ")
}

func (l *Library) Validate() error {
	if l.Name == "" {
		return errors.New("library name cannot be empty")
	}
	l.Slug = utils.Sluggify(l.Name)
	if l.Description == "" {
		return errors.New("library description cannot be empty")
	}
	if l.Cron == "" {
		return errors.New("library cron cannot be empty")
	}
	return nil
}

func CreateLibrary(library Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	exists, err := LibraryExists(library.Slug)
	if err != nil {
		return err
	}
	if !exists {
		NotifyListeners(Notification{Type: "library_created", Payload: library})
		return create("libraries", library.Slug, library)
	} else {
		return errors.New("library already exists")
	}
}

func GetLibraries() ([]Library, error) {
	dataList, err := getAll("libraries")
	if err != nil {
		return nil, err
	}
	var libraries []Library
	for _, data := range dataList {
		var library Library
		if err := json.Unmarshal(data, &library); err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
	}
	return libraries, nil
}

func GetLibrary(slug string) (*Library, error) {
	var library Library
	err := get("libraries", slug, &library)
	if err != nil {
		return nil, err
	}
	return &library, nil
}

func UpdateLibrary(library *Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	NotifyListeners(Notification{Type: "library_updated", Payload: *library})
	return update("libraries", library.Slug, library)
}

func DeleteLibrary(slug string) error {
	library, _ := GetLibrary(slug)

	err := delete("libraries", slug)
	if err != nil {
		return err
	}
	NotifyListeners(Notification{Type: "library_deleted", Payload: *library})
	return DeleteMangasByLibrarySlug(slug)
}

func SearchLibraries(keyword string, page int, pageSize int, sortBy string, sortOrder string) ([]Library, error) {
	libraries, err := GetLibraries()
	if err != nil {
		return nil, err
	}

	if keyword != "" {
		var libraryNames []string
		for _, lib := range libraries {
			libraryNames = append(libraryNames, lib.Name)
		}
		matchingNames := utils.BigramSearch(keyword, libraryNames)
		var filteredLibraries []Library
		for _, lib := range libraries {
			for _, name := range matchingNames {
				if lib.Name == name {
					filteredLibraries = append(filteredLibraries, lib)
					break
				}
			}
		}
		libraries = filteredLibraries
	}

	// Sort libraries based on sortBy and sortOrder
	// Implement sorting logic here

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start < len(libraries) {
		if end > len(libraries) {
			end = len(libraries)
		}
		return libraries[start:end], nil
	}
	return []Library{}, nil
}

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
