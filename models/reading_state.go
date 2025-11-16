package models

import (
    "time"
)

// ReadingState represents a record that a user has read a chapter of a manga
type ReadingState struct {
    ID         int64     `json:"id"`
    UserName   string    `json:"user_name"`
    MangaSlug  string    `json:"manga_slug"`
    Chapter    string    `json:"chapter_slug"`
    CreatedAt  time.Time `json:"created_at"`
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

// GetLastReadChapter returns the most recently read chapter slug for a user on a specific manga
func GetLastReadChapter(userName, mangaSlug string) (string, error) {
    query := `
    SELECT chapter_slug
    FROM reading_states
    WHERE user_name = ? AND manga_slug = ?
    ORDER BY created_at DESC
    LIMIT 1
    `

    var chapterSlug string
    err := db.QueryRow(query, userName, mangaSlug).Scan(&chapterSlug)
    if err != nil {
        // Return empty string if no record found
        if err.Error() == "sql: no rows in result set" {
            return "", nil
        }
        return "", err
    }

    return chapterSlug, nil
}

// GetChapterProgress returns the image index where the user left off in a chapter
func GetChapterProgress(userName, mangaSlug, chapterSlug string) (int, error) {
    query := `
    SELECT image_index
    FROM reading_states
    WHERE user_name = ? AND manga_slug = ? AND chapter_slug = ?
    `

    var imageIndex int
    err := db.QueryRow(query, userName, mangaSlug, chapterSlug).Scan(&imageIndex)
    if err != nil {
        // Return 0 if no record found
        if err.Error() == "sql: no rows in result set" {
            return 0, nil
        }
        return 0, err
    }

    return imageIndex, nil
}

// DeleteReadingStatesByUser optionally used for cleanup
func DeleteReadingStatesByUser(userName string) error {
    _, err := db.Exec(`DELETE FROM reading_states WHERE user_name = ?`, userName)
    return err
}

// ReadingActivityItem represents a recent reading activity with manga details
type ReadingActivityItem struct {
    ReadingState ReadingState
    Manga        *Manga
}

// GetRecentReadingActivity returns the most recent reading activities for a user with manga details
func GetRecentReadingActivity(userName string, limit int) ([]ReadingActivityItem, error) {
    query := `
    SELECT id, user_name, manga_slug, chapter_slug, created_at
    FROM reading_states
    WHERE user_name = ? AND (manga_slug, created_at) IN (
        SELECT manga_slug, MAX(created_at)
        FROM reading_states
        WHERE user_name = ?
        GROUP BY manga_slug
    )
    ORDER BY created_at DESC
    LIMIT ?
    `

    rows, err := db.Query(query, userName, userName, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var activities []ReadingActivityItem
    for rows.Next() {
        var rs ReadingState
        if err := rows.Scan(&rs.ID, &rs.UserName, &rs.MangaSlug, &rs.Chapter, &rs.CreatedAt); err != nil {
            return nil, err
        }

        manga, err := GetManga(rs.MangaSlug)
        if err != nil {
            continue // Skip if manga not found
        }

        activities = append(activities, ReadingActivityItem{
            ReadingState: rs,
            Manga:        manga,
        })
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return activities, nil
}
