package models

import (
    "fmt"
)

// GetTagsForManga returns a slice of tag names associated with the manga slug
func GetTagsForManga(mangaSlug string) ([]string, error) {
    query := `SELECT tag FROM manga_tags WHERE manga_slug = ? ORDER BY tag`
    rows, err := db.Query(query, mangaSlug)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tags []string
    for rows.Next() {
        var tag string
        if err := rows.Scan(&tag); err != nil {
            return nil, err
        }
        tags = append(tags, tag)
    }
    return tags, nil
}

// SetTagsForManga replaces tags for given manga slug (delete existing then insert)
func SetTagsForManga(mangaSlug string, tags []string) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        } else {
            _ = tx.Commit()
        }
    }()

    if _, err = tx.Exec(`DELETE FROM manga_tags WHERE manga_slug = ?`, mangaSlug); err != nil {
        return err
    }

    stmt, err := tx.Prepare(`INSERT INTO manga_tags (manga_slug, tag) VALUES (?, ?)`)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, t := range tags {
        if _, err := stmt.Exec(mangaSlug, t); err != nil {
            return err
        }
    }
    return nil
}

// DeleteTagsByMangaSlug removes tags associated with a manga
func DeleteTagsByMangaSlug(mangaSlug string) error {
    query := `DELETE FROM manga_tags WHERE manga_slug = ?`
    if _, err := db.Exec(query, mangaSlug); err != nil {
        return fmt.Errorf("failed to delete tags for manga '%s': %w", mangaSlug, err)
    }
    return nil
}
