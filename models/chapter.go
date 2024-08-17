package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/alexander-bruun/magi/utils"
	"go.etcd.io/bbolt"
)

type Chapter struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	File            string `json:"file"`
	ChapterCoverURL string `json:"chapter_cover_url"`
	MangaSlug       string `json:"manga_slug"`
}

// CreateChapter adds a new chapter if it does not already exist
func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	if exists, err := ChapterExists(chapter.Slug, chapter.MangaSlug); err != nil {
		return err
	} else if exists {
		return errors.New("chapter already exists")
	}

	return create("chapters", chapterKey(chapter.MangaSlug, chapter.Slug), chapter)
}

// GetChapters retrieves all chapters for a specific manga, sorted by name
func GetChapters(mangaSlug string) ([]Chapter, error) {
	var chapters []Chapter
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("chapters"))
		cursor := bucket.Cursor()
		prefix := []byte(mangaSlug + ":")

		for k, v := cursor.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = cursor.Next() {
			var chapter Chapter
			if err := json.Unmarshal(v, &chapter); err != nil {
				return err
			}
			chapters = append(chapters, chapter)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sortChaptersByNumber(chapters)
	return chapters, nil
}

// GetChapter retrieves a specific chapter by its slug
func GetChapter(mangaSlug, chapterSlug string) (*Chapter, error) {
	var chapter Chapter
	err := get("chapters", chapterKey(mangaSlug, chapterSlug), &chapter)
	if err != nil {
		return nil, err
	}
	return &chapter, nil
}

// UpdateChapter modifies an existing chapter
func UpdateChapter(chapter *Chapter) error {
	return update("chapters", chapterKey(chapter.MangaSlug, chapter.Slug), chapter)
}

// DeleteChapter removes a specific chapter
func DeleteChapter(mangaSlug, chapterSlug string) error {
	return delete("chapters", chapterKey(mangaSlug, chapterSlug))
}

// DeleteChaptersByMangaSlug removes all chapters for a specific manga
func DeleteChaptersByMangaSlug(mangaSlug string) error {
	return deleteKeysWithPattern("chapters", mangaSlug+"*")
}

// ChapterExists checks if a chapter already exists
func ChapterExists(chapterSlug, mangaSlug string) (bool, error) {
	var chapter Chapter
	err := get("chapters", chapterKey(mangaSlug, chapterSlug), &chapter)
	if err == bbolt.ErrBucketNotFound {
		return false, nil
	}
	return err == nil, err
}

// GetAdjacentChapters finds the previous and next chapters based on the current chapter slug
func GetAdjacentChapters(chapterSlug, mangaSlug string) (prevSlug, nextSlug string, err error) {
	chapters, err := GetChapters(mangaSlug)
	if err != nil {
		return "", "", err
	}

	currentIndex := indexOfChapter(chapters, chapterSlug)
	if currentIndex == -1 {
		return "", "", errors.New("chapter not found")
	}

	if currentIndex > 0 {
		prevSlug = chapters[currentIndex-1].Slug
	}
	if currentIndex < len(chapters)-1 {
		nextSlug = chapters[currentIndex+1].Slug
	}

	return prevSlug, nextSlug, nil
}

// Helper functions

func chapterKey(mangaSlug, chapterSlug string) string {
	return fmt.Sprintf("%s:%s", mangaSlug, chapterSlug)
}

func sortChaptersByNumber(chapters []Chapter) {
	sort.Slice(chapters, func(i, j int) bool {
		numI, errI := utils.ExtractNumber(chapters[i].Name)
		numJ, errJ := utils.ExtractNumber(chapters[j].Name)
		if errI != nil || errJ != nil {
			return chapters[i].Name < chapters[j].Name
		}
		return numI < numJ
	})
}

func indexOfChapter(chapters []Chapter, chapterSlug string) int {
	for i, chapter := range chapters {
		if chapter.Slug == chapterSlug {
			return i
		}
	}
	return -1
}
