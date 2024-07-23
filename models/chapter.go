package models

import (
	"errors"
	"sort"

	"github.com/alexander-bruun/magi/utils"
	"gorm.io/gorm"
)

type Chapter struct {
	gorm.Model
	Name            string `gorm:"not null"`
	Slug            string `gorm:"not null"`
	Type            string
	File            string
	ChapterCoverURL string
	MangaID         uint
}

// CreateChapter creates a new chapter record in the database
func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug, chapter.MangaID)
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

// GetChapters retrieves all chapters for a specific manga ID and returns them sorted by the chapter name
func GetChapters(mangaID uint) ([]Chapter, error) {
	var chapters []Chapter
	err := db.Where("manga_id = ?", mangaID).Find(&chapters).Error
	if err != nil {
		return nil, err
	}

	// Sort the chapters by the number in the name
	sort.Slice(chapters, func(i, j int) bool {
		numI, errI := utils.ExtractNumber(chapters[i].Name)
		numJ, errJ := utils.ExtractNumber(chapters[j].Name)
		if errI != nil || errJ != nil {
			return chapters[i].Name < chapters[j].Name
		}
		return numI < numJ
	})

	return chapters, nil
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

// ChapterExists checks if a chapter already exists with the given slug and manga ID
func ChapterExists(slug string, mangaID uint) (bool, error) {
	var count int64
	err := db.Model(&Chapter{}).Where("slug = ? AND manga_id = ?", slug, mangaID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetChapterIDBySlug retrieves the ID of a chapter record by its slug
func GetChapterIDBySlug(slug string, mangaID uint) (uint, error) {
	var chapter Chapter
	err := db.Select("id").Where("slug = ? AND manga_id = ?", slug, mangaID).First(&chapter).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("manga not found")
		}
		return 0, err
	}
	return chapter.ID, nil
}

// GetAdjacentChapters returns the previous and next chapter slugs for a given chapter based on its order
func GetAdjacentChapters(chapterSlug string, mangaID uint) (prevSlug, nextSlug string, err error) {
	// Get all chapters for the manga and sort them
	chapters, err := GetChapters(mangaID)
	if err != nil {
		return "", "", err
	}

	// Find the position of the given chapter slug in the sorted list
	var currentIndex int
	found := false
	for i, chapter := range chapters {
		if chapter.Slug == chapterSlug {
			currentIndex = i
			found = true
			break
		}
	}

	if !found {
		return "", "", errors.New("chapter not found")
	}

	// Determine the previous and next chapter slugs
	if currentIndex > 0 {
		prevSlug = chapters[currentIndex-1].Slug
	}
	if currentIndex < len(chapters)-1 {
		nextSlug = chapters[currentIndex+1].Slug
	}

	return prevSlug, nextSlug, nil
}
