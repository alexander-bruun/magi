package models

import (
	"fmt"
	"sort"
)

// GetTagsForMedia returns a slice of tag names associated with the media slug
func GetTagsForMedia(mangaSlug string) ([]string, error) {
	query := `SELECT tag FROM media_tags WHERE media_slug = ? ORDER BY tag`
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

// SetTagsForMedia replaces tags for given media slug (delete existing then insert)
func SetTagsForMedia(mangaSlug string, tags []string) error {
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

	if _, err = tx.Exec(`DELETE FROM media_tags WHERE media_slug = ?`, mangaSlug); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO media_tags (media_slug, tag) VALUES (?, ?)`)
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

// DeleteTagsByMediaSlug removes tags associated with a media
func DeleteTagsByMediaSlug(mangaSlug string) error {
	query := `DELETE FROM media_tags WHERE media_slug = ?`
	if _, err := db.Exec(query, mangaSlug); err != nil {
		return fmt.Errorf("failed to delete tags for media '%s': %w", mangaSlug, err)
	}
	return nil
}

// GetAllTags returns all distinct tags across mangas, sorted ascending
func GetAllTags() ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT tag FROM media_tags`)
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

// GetAllMediaTagsMap returns a mapping from media_slug to its tags
func GetAllMediaTagsMap() (map[string][]string, error) {
	rows, err := db.Query(`SELECT media_slug, tag FROM media_tags`)
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
        FROM media_tags mt
        INNER JOIN favorites f ON mt.media_slug = f.media_slug
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
        FROM media_tags mt
        INNER JOIN reading_states rs ON mt.media_slug = rs.media_slug
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
        FROM media_tags mt
        INNER JOIN votes v ON mt.media_slug = v.media_slug
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
        FROM media_tags mt
        INNER JOIN votes v ON mt.media_slug = v.media_slug
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

// GetTagUsageStats returns a map of tag names to their usage count across all media
func GetTagUsageStats() (map[string]int, error) {
	query := `SELECT tag, COUNT(*) as count FROM media_tags GROUP BY tag ORDER BY count DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		if tag != "" {
			stats[tag] = count
		}
	}
	return stats, nil
}
