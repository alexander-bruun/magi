package models

import (
    "time"
)

// ReadingState represents a record that a user has read a chapter of a manga
type ReadingState struct {
    ID        int64     `json:"id"`
    UserName  string    `json:"user_name"`
    MangaSlug string    `json:"manga_slug"`
    Chapter   string    `json:"chapter_slug"`
    CreatedAt time.Time `json:"created_at"`
}

// MarkChapterRead inserts a reading state if not exists
func MarkChapterRead(userName, mangaSlug, chapterSlug string) error {
    query := `
    INSERT INTO reading_states (user_name, manga_slug, chapter_slug, created_at)
    VALUES (?, ?, ?, CURRENT_TIMESTAMP)
    ON CONFLICT(user_name, manga_slug, chapter_slug) DO UPDATE SET created_at = CURRENT_TIMESTAMP
    `

    _, err := db.Exec(query, userName, mangaSlug, chapterSlug)
    return err
}

// UnmarkChapterRead deletes a reading state for a given user/manga/chapter
func UnmarkChapterRead(userName, mangaSlug, chapterSlug string) error {
    query := `
    DELETE FROM reading_states
    WHERE user_name = ? AND manga_slug = ? AND chapter_slug = ?
    `

    _, err := db.Exec(query, userName, mangaSlug, chapterSlug)
    return err
}

// GetReadChaptersForUser returns a map of chapterSlug->true for chapters the user has read for a given manga
func GetReadChaptersForUser(userName, mangaSlug string) (map[string]bool, error) {
    query := `
    SELECT chapter_slug
    FROM reading_states
    WHERE user_name = ? AND manga_slug = ?
    `

    rows, err := db.Query(query, userName, mangaSlug)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    m := make(map[string]bool)
    for rows.Next() {
        var chapter string
        if err := rows.Scan(&chapter); err != nil {
            return nil, err
        }
        m[chapter] = true
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return m, nil
}

// DeleteReadingStatesByUser optionally used for cleanup
func DeleteReadingStatesByUser(userName string) error {
    _, err := db.Exec(`DELETE FROM reading_states WHERE user_name = ?`, userName)
    return err
}
