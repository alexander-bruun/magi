package models

import (
	"database/sql"
	"errors"
	"sort"

	"github.com/alexander-bruun/magi/utils"
)

// Chapter represents the chapter table schema
type Chapter struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	File            string `json:"file"`
	ChapterCoverURL string `json:"chapter_cover_url"`
	MangaSlug       string `json:"manga_slug"`
	Read            bool   `json:"read"`
}

// CreateChapter adds a new chapter if it does not already exist
func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug, chapter.MangaSlug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("chapter already exists")
	}

	query := `
	INSERT INTO chapters (slug, name, type, file, chapter_cover_url, manga_slug)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, chapter.Slug, chapter.Name, chapter.Type, chapter.File, chapter.ChapterCoverURL, chapter.MangaSlug)
	if err != nil {
		return err
	}

	return nil
}

// GetChapters retrieves all chapters for a specific manga, sorted by name
func GetChapters(mangaSlug string) ([]Chapter, error) {
	query := `
	SELECT slug, name, type, file, chapter_cover_url, manga_slug
	FROM chapters
	WHERE manga_slug = ?
	`

	rows, err := db.Query(query, mangaSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []Chapter
	for rows.Next() {
		var chapter Chapter
		if err := rows.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MangaSlug); err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortChaptersByNumber(chapters)
	return chapters, nil
}

// GetChapter retrieves a specific chapter by its slug
func GetChapter(mangaSlug, chapterSlug string) (*Chapter, error) {
	query := `
	SELECT slug, name, type, file, chapter_cover_url, manga_slug
	FROM chapters
	WHERE manga_slug = ? AND slug = ?
	`

	row := db.QueryRow(query, mangaSlug, chapterSlug)

	var chapter Chapter
	err := row.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MangaSlug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No chapter found
		}
		return nil, err
	}

	return &chapter, nil
}

// UpdateChapter modifies an existing chapter
func UpdateChapter(chapter *Chapter) error {
	query := `
	UPDATE chapters
	SET name = ?, type = ?, file = ?, chapter_cover_url = ?
	WHERE manga_slug = ? AND slug = ?
	`

	_, err := db.Exec(query, chapter.Name, chapter.Type, chapter.File, chapter.ChapterCoverURL, chapter.MangaSlug, chapter.Slug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteChapter removes a specific chapter
func DeleteChapter(mangaSlug, chapterSlug string) error {
	return DeleteRecord(`DELETE FROM chapters WHERE manga_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
}

// DeleteChaptersByMangaSlug removes all chapters for a specific manga
func DeleteChaptersByMangaSlug(mangaSlug string) error {
	return DeleteRecord(`DELETE FROM chapters WHERE manga_slug = ?`, mangaSlug)
}

// ChapterExists checks if a chapter already exists
func ChapterExists(chapterSlug, mangaSlug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM chapters WHERE manga_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
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
