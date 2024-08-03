package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alexander-bruun/magi/utils"
	"gorm.io/gorm"
)

type Manga struct {
	gorm.Model
	Name             string `gorm:"not null"`
	Author           string
	Slug             string `gorm:"unique;not null"`
	Description      string
	Year             int
	OriginalLanguage string
	Status           string
	ContentRating    string
	LibraryID        uint
	Chapters         []Chapter `gorm:"foreignKey:MangaID"`
	CoverArtURL      string
	Path             string `gorm:"unique;not null"`
}

// CreateManga creates a new manga record in the database
func CreateManga(manga Manga) (uint, error) {
	manga.Slug = utils.Sluggify(manga.Name)
	exists, err := MangaExists(manga.Slug)
	if err != nil {
		return 0, err
	}
	if !exists {
		if err := db.Create(&manga).Error; err != nil {
			return 0, err
		}
		return manga.ID, nil // Return the newly created manga's ID
	} else {
		return 0, errors.New("manga already exists")
	}
}

// GetManga retrieves a manga record by ID
func GetManga(id uint) (*Manga, error) {
	var manga Manga
	err := db.First(&manga, id).Error
	if err != nil {
		return nil, err
	}
	return &manga, nil
}

// UpdateManga updates an existing manga record
func UpdateManga(manga *Manga) error {
	err := db.Session(&gorm.Session{FullSaveAssociations: true}).Save(&manga).Error
	if err != nil {
		return err
	}
	return nil
}

// DeleteManga deletes a manga record by ID
func DeleteManga(id uint) error {
	err := db.Unscoped().Delete(&Manga{}, id).Error
	if err != nil {
		return err
	}
	return nil
}

// SearchMangas performs a search on manga names with pagination, sorting, and filtering
func SearchMangas(filter string, page int, pageSize int, sortBy string, sortOrder string, filterBy string, libraryID uint) ([]Manga, int64, error) {
	var mangas []Manga
	var total int64
	var err error

	// Create a base query
	baseQuery := db.Model(&Manga{})

	// Apply LibraryID filter if provided
	if libraryID != 0 {
		baseQuery = baseQuery.Where("library_id = ?", libraryID)
	}

	// Count total number of results with filters applied
	err = baseQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if sortBy != "" && sortOrder != "" {
		baseQuery = baseQuery.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))
	}

	// Fetch all mangas (without pagination)
	err = baseQuery.Find(&mangas).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply bigram search if filter is provided
	if filter != "" {
		filteredMangas := applyBigramSearch(filter, mangas)
		total = int64(len(filteredMangas))

		// Apply pagination to filtered results
		start := (page - 1) * pageSize
		end := start + pageSize
		if start < len(filteredMangas) {
			if end > len(filteredMangas) {
				end = len(filteredMangas)
			}
			return filteredMangas[start:end], total, nil
		}
		return []Manga{}, total, nil
	}

	// Apply pagination if no filter
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

// applyBigramSearch uses the existing BigramSearch function to filter mangas
func applyBigramSearch(filter string, mangas []Manga) []Manga {
	var mangaNames []string
	nameToManga := make(map[string]Manga)

	// Extract manga names and create a map for lookup
	for _, manga := range mangas {
		mangaNames = append(mangaNames, manga.Name)
		nameToManga[manga.Name] = manga
	}

	// Use the existing BigramSearch function
	matchingNames := utils.BigramSearch(filter, mangaNames)

	// Create the filtered manga slice
	var filteredMangas []Manga
	for _, name := range matchingNames {
		if manga, ok := nameToManga[name]; ok {
			filteredMangas = append(filteredMangas, manga)
		}
	}

	return filteredMangas
}

// MangaExists checks if a manga already exists with the given slug
func MangaExists(slug string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM mangas WHERE slug = ?)`
	if err := db.Raw(query, slug).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// GetMangasCount returns the total number of mangas in the database
func MangaCount(filterBy, filter string) (int, error) {
	var count int64

	query := db.Model(&Manga{})

	if filterBy != "" && filter != "" {
		query = query.Where("LOWER("+filterBy+") LIKE ?", "%"+strings.ToLower(filter)+"%")
	}

	result := query.Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}

	return int(count), nil
}

// GetMangaIDBySlug retrieves the ID of a manga record by its slug
func GetMangaIDBySlug(slug string) (uint, error) {
	var manga Manga
	err := db.Select("id").Where("slug = ?", slug).First(&manga).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("manga not found")
		}
		return 0, err
	}
	return manga.ID, nil
}

// DeleteMangasByLibraryID deletes all mangas with the specified library ID
func DeleteMangasByLibraryID(libraryID uint) error {
	// Check if any mangas exist with the given library ID
	var mangas []Manga
	if err := db.Where("library_id = ?", libraryID).Find(&mangas).Error; err != nil {
		return err
	}

	// If no mangas are found, return an error
	if len(mangas) == 0 {
		return errors.New("no mangas found with the specified library ID")
	}

	// Delete the mangas with the given library ID
	if err := db.Unscoped().Where("library_id = ?", libraryID).Delete(&Manga{}).Error; err != nil {
		return err
	}

	return nil
}
