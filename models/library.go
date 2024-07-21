package models

import (
	"errors"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"gorm.io/gorm"
)

// Define the Library model using the custom type
type Library struct {
	gorm.Model
	Name        string      `gorm:"unique;not null"`
	Slug        string      `gorm:"unique;not null"`
	Description string      `gorm:"not null"`
	Cron        string      `gorm:"not null"`
	Mangas      []Manga     `gorm:"foreignKey:LibraryID"`
	Folders     StringArray `gorm:"type:text"`
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

// CreateLibrary creates a new library record in the database
func CreateLibrary(library Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	exists, err := LibraryExists(library.Slug)
	if err != nil {
		return err
	}
	if !exists {
		if err := db.Create(&library).Error; err != nil {
			return err
		}
		NotifyListeners(Notification{Type: "library_created", Payload: library})
	} else {
		return errors.New("library already exists")
	}
	return nil
}

// GetLibraries retrieves all library records with their associated Mangas and Folders
func GetLibraries() ([]Library, error) {
	var libraries []Library
	err := db.Find(&libraries).Error
	if err != nil {
		return nil, err
	}
	return libraries, nil
}

// GetLibrary retrieves a library record by ID
func GetLibrary(id uint) (*Library, error) {
	var library Library
	err := db.Find(&library, id).Error
	if err != nil {
		return nil, err
	}
	return &library, nil
}

func UpdateLibrary(library *Library) error {
	if err := library.Validate(); err != nil {
		return err
	}

	// Save the updated library with FullSaveAssociations but omit CreatedAt and UpdatedAt
	if err := db.Session(&gorm.Session{FullSaveAssociations: true}).Omit("CreatedAt", "UpdatedAt").Save(library).Error; err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_updated", Payload: *library})
	return nil
}

// DeleteLibrary deletes a library record by ID
func DeleteLibrary(id uint) error {
	library, err := GetLibrary(id)
	if err != nil {
		return err
	}

	err = db.Unscoped().Delete(&Library{}, id).Error
	if err != nil {
		return err
	}

	DeleteMangasByLibraryID(id)

	NotifyListeners(Notification{Type: "library_deleted", Payload: *library})
	return nil
}

// SearchLibraries performs a bigram search on library names with pagination and sorting
func SearchLibraries(keyword string, page int, pageSize int, sortBy string, sortOrder string) ([]Library, error) {
	var libraries []Library
	var err error

	// Bigram search
	if keyword != "" {
		var libraryNames []string
		err = db.Model(&Library{}).Pluck("name", &libraryNames).Error
		if err != nil {
			return nil, err
		}

		matchingNames := utils.BigramSearch(keyword, libraryNames)
		err = db.Where("name IN (?)", matchingNames).
			Order(sortBy + " " + sortOrder).
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&libraries).Error
	} else {
		err = db.Order(sortBy + " " + sortOrder).
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&libraries).Error
	}

	if err != nil {
		return nil, err
	}
	return libraries, nil
}

// LibraryExists checks if a library already exists with the given slug
func LibraryExists(slug string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM libraries WHERE slug = ?)`
	if err := db.Raw(query, slug).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// GetLibraryIDBySlug retrieves the ID of a library record by its slug
func GetLibraryIDBySlug(slug string) (uint, error) {
	var library Library
	err := db.Select("id").Where("slug = ?", slug).First(&library).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("library not found")
		}
		return 0, err
	}
	return library.ID, nil
}
