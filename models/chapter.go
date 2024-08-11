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

func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug, chapter.MangaSlug)
	if err != nil {
		return err
	}
	if !exists {
		return create("chapters", fmt.Sprintf("%s:%s", chapter.MangaSlug, chapter.Slug), chapter)
	} else {
		return errors.New("chapter already exists")
	}
}

func GetChapters(mangaSlug string) ([]Chapter, error) {
	var chapters []Chapter
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("chapters"))
		c := b.Cursor()
		prefix := []byte(fmt.Sprintf("%s:", mangaSlug))
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
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

func GetChapter(mangaSlug, chapterSlug string) (*Chapter, error) {
	var chapter Chapter
	err := get("chapters", fmt.Sprintf("%s:%s", mangaSlug, chapterSlug), &chapter)
	if err != nil {
		return nil, err
	}
	return &chapter, nil
}

func UpdateChapter(chapter *Chapter) error {
	return update("chapters", fmt.Sprintf("%s:%s", chapter.MangaSlug, chapter.Slug), chapter)
}

func DeleteChapter(mangaSlug, chapterSlug string) error {
	return delete("chapters", fmt.Sprintf("%s:%s", mangaSlug, chapterSlug))
}

func DeleteChaptersByMangaSlug(mangaSlug string) error {
	dataList, err := getAll("chapters")
	if err != nil {
		return err
	}

	for _, data := range dataList {
		var chapter Chapter
		if err := json.Unmarshal(data, &chapter); err != nil {
			return err
		}

		if chapter.MangaSlug == mangaSlug {
			err := delete("mangas", chapter.Slug)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func SearchChapters(keyword string, page int, pageSize int, sortBy string, sortOrder string) ([]Chapter, error) {
	dataList, err := getAll("chapters")
	if err != nil {
		return nil, err
	}

	var chapters []Chapter
	for _, data := range dataList {
		var chapter Chapter
		if err := json.Unmarshal(data, &chapter); err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}

	if keyword != "" {
		var chapterNames []string
		for _, ch := range chapters {
			chapterNames = append(chapterNames, ch.Name)
		}
		matchingNames := utils.BigramSearch(keyword, chapterNames)
		var filteredChapters []Chapter
		for _, ch := range chapters {
			for _, name := range matchingNames {
				if ch.Name == name {
					filteredChapters = append(filteredChapters, ch)
					break
				}
			}
		}
		chapters = filteredChapters
	}

	// Sort chapters based on sortBy and sortOrder
	// Implement sorting logic here

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start < len(chapters) {
		if end > len(chapters) {
			end = len(chapters)
		}
		return chapters[start:end], nil
	}
	return []Chapter{}, nil
}

func ChapterExists(chapterSlug, mangaSlug string) (bool, error) {
	var chapter Chapter
	err := get("chapters", fmt.Sprintf("%s:%s", mangaSlug, chapterSlug), &chapter)
	if err == bbolt.ErrBucketNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetAdjacentChapters(chapterSlug string, mangaSlug string) (prevSlug, nextSlug string, err error) {
	chapters, err := GetChapters(mangaSlug)
	if err != nil {
		return "", "", err
	}

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

	if currentIndex > 0 {
		prevSlug = chapters[currentIndex-1].Slug
	}
	if currentIndex < len(chapters)-1 {
		nextSlug = chapters[currentIndex+1].Slug
	}

	return prevSlug, nextSlug, nil
}
