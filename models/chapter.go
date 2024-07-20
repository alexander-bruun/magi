package models

import (
	"errors"

	"github.com/alexander-bruun/magi/utils"
	"gorm.io/gorm"
)

type Chapter struct {
	gorm.Model
	Name            string `gorm:"not null"`
	Slug            string `gorm:"unique;not null"`
	Order           int    `gorm:"not null"`
	Type            string `gorm:"not null"`
	File            string
	ChapterCoverURL string
	MangaID         uint
}

// CreateChapter creates a new chapter record in the database
func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug)
	if err != nil {
		return err
	}
	if !exists {
		if err := db.Create(&chapter).Error; err != nil {
			return err
		}
	} else {
		return errors.New("chapter already exists")
	}
	return nil
}

// GetChapter retrieves a chapter record by ID
func GetChapter(id uint) (*Chapter, error) {
	var chapter Chapter
	err := db.First(&chapter, id).Error
	if err != nil {
		return nil, err
	}
	return &chapter, nil
}

// UpdateChapter updates an existing chapter record
func UpdateChapter(chapter *Chapter) error {
	err := db.Session(&gorm.Session{FullSaveAssociations: true}).Save(&chapter).Error
	if err != nil {
		return err
	}
	return nil
}

// DeleteChapter deletes a chapter record by ID
func DeleteChapter(id uint) error {
	chapter, err := GetChapter(id)
	if err != nil {
		return err
	}
	err = db.Unscoped().Delete(chapter).Error
	if err != nil {
		return err
	}
	return nil
}

// SearchChapters performs a bigram search on chapter volume names with pagination and sorting
func SearchChapters(keyword string, page int, pageSize int, sortBy string, sortOrder string) ([]Chapter, error) {
	var chapters []Chapter
	var err error

	// Bigram search
	if keyword != "" {
		var chapterTitles []string
		err = db.Model(&Chapter{}).Pluck("volume_name", &chapterTitles).Error
		if err != nil {
			return nil, err
		}

		matchingTitles := utils.BigramSearch(keyword, chapterTitles)
		err = db.Where("volume_name IN (?)", matchingTitles).
			Order(sortBy + " " + sortOrder).
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&chapters).Error
	} else {
		err = db.Order(sortBy + " " + sortOrder).
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&chapters).Error
	}

	if err != nil {
		return nil, err
	}
	return chapters, nil
}

// ChapterExists checks if a chapter already exists with the given slug
func ChapterExists(slug string) (bool, error) {
	var count int64
	err := db.Model(&Chapter{}).Where("slug = ?", slug).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
