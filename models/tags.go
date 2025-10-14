package models

import (
    "fmt"
    "sort"
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

// GetAllTags returns all distinct tags across mangas, sorted ascending
func GetAllTags() ([]string, error) {
    rows, err := db.Query(`SELECT DISTINCT tag FROM manga_tags`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tags []string
    for rows.Next() {
        var t string
        if err := rows.Scan(&t); err != nil {
            return nil, err
        }
        if t != "" {
            tags = append(tags, t)
        }
    }
    sort.Strings(tags)
    return tags, nil
}

// GetAllMangaTagsMap returns a mapping from manga_slug to its tags
func GetAllMangaTagsMap() (map[string][]string, error) {
    rows, err := db.Query(`SELECT manga_slug, tag FROM manga_tags`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    m := make(map[string][]string)
    for rows.Next() {
        var slug, tag string
        if err := rows.Scan(&slug, &tag); err != nil {
            return nil, err
        }
        if tag == "" || slug == "" {
            continue
        }
        m[slug] = append(m[slug], tag)
    }
    return m, nil
}

// GetTagsForUserFavorites returns all distinct tags for mangas favorited by the user
func GetTagsForUserFavorites(username string) ([]string, error) {
    query := `
        SELECT DISTINCT mt.tag 
        FROM manga_tags mt
        INNER JOIN favorites f ON mt.manga_slug = f.manga_slug
        WHERE f.user_username = ?
        ORDER BY mt.tag
    `
    rows, err := db.Query(query, username)
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
        if tag != "" {
            tags = append(tags, tag)
        }
    }
    return tags, nil
}

// GetTagsForUserReading returns all distinct tags for mangas the user is reading
func GetTagsForUserReading(username string) ([]string, error) {
    query := `
        SELECT DISTINCT mt.tag 
        FROM manga_tags mt
        INNER JOIN reading_states rs ON mt.manga_slug = rs.manga_slug
        WHERE rs.user_name = ?
        ORDER BY mt.tag
    `
    rows, err := db.Query(query, username)
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
        if tag != "" {
            tags = append(tags, tag)
        }
    }
    return tags, nil
}

// GetTagsForUserUpvoted returns all distinct tags for mangas the user has upvoted
func GetTagsForUserUpvoted(username string) ([]string, error) {
    query := `
        SELECT DISTINCT mt.tag 
        FROM manga_tags mt
        INNER JOIN votes v ON mt.manga_slug = v.manga_slug
        WHERE v.user_username = ? AND v.value = 1
        ORDER BY mt.tag
    `
    rows, err := db.Query(query, username)
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
        if tag != "" {
            tags = append(tags, tag)
        }
    }
    return tags, nil
}

// GetTagsForUserDownvoted returns all distinct tags for mangas the user has downvoted
func GetTagsForUserDownvoted(username string) ([]string, error) {
    query := `
        SELECT DISTINCT mt.tag 
        FROM manga_tags mt
        INNER JOIN votes v ON mt.manga_slug = v.manga_slug
        WHERE v.user_username = ? AND v.value = -1
        ORDER BY mt.tag
    `
    rows, err := db.Query(query, username)
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
        if tag != "" {
            tags = append(tags, tag)
        }
    }
    return tags, nil
}
