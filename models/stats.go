package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Simple DB-backed counters for homepage statistics
func GetTotalMedias() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM media`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChapters() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM chapters`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChaptersRead() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalMediasByType(mediaType string) (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM media WHERE LOWER(TRIM(type)) = LOWER(TRIM(?))`, mediaType)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChaptersByType(mediaType string) (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM chapters c INNER JOIN media m ON c.media_slug = m.slug WHERE LOWER(TRIM(m.type)) = LOWER(TRIM(?))`, mediaType)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChaptersReadByType(mediaType string) (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states rs INNER JOIN media m ON rs.media_slug = m.slug WHERE LOWER(TRIM(m.type)) = LOWER(TRIM(?))`, mediaType)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

// GetChaptersReadCount returns the number of reading_state records for a given manga
func GetChaptersReadCount(mangaSlug string) (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states WHERE media_slug = ?`, mangaSlug)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func RecordDailyStatistics() error {
    today := time.Now().Format("2006-01-02")
    
    totalMedias, err := GetTotalMedias()
    if err != nil {
        return err
    }
    totalChapters, err := GetTotalChapters()
    if err != nil {
        return err
    }
    totalChaptersRead, err := GetTotalChaptersRead()
    if err != nil {
        return err
    }

    query := `
        INSERT OR REPLACE INTO daily_statistics 
        (date, total_media, total_chapters, total_chapters_read)
        VALUES (?, ?, ?, ?)
    `
    _, err = db.Exec(query, today, totalMedias, totalChapters, totalChaptersRead)
    return err
}

func GetDailyChange(statType string) (int, error) {
    today := time.Now().Format("2006-01-02")
    
    var query string
    switch statType {
    case "media":
        // For media, count distinct media that have chapters read today
        query = `SELECT COUNT(DISTINCT media_slug) FROM reading_states WHERE DATE(created_at) = ?`
    case "chapters":
        // For chapters, count total chapters read today
        query = `SELECT COUNT(*) FROM reading_states WHERE DATE(created_at) = ?`
    case "chapters_read":
        // This is the same as chapters for reading_states
        query = `SELECT COUNT(*) FROM reading_states WHERE DATE(created_at) = ?`
    default:
        return 0, fmt.Errorf("unknown stat type: %s", statType)
    }
    
    row := db.QueryRow(query, today)
    var change int
    err := row.Scan(&change)
    return change, err
}

func GetDailyChangeByType(statType string, mediaType string) (int, error) {
    today := time.Now().Format("2006-01-02")
    yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
    
    var query string
    switch statType {
    case "media":
        query = `
            SELECT 
                (SELECT COUNT(*) FROM media WHERE DATE(created_at) = ? AND LOWER(TRIM(type)) = LOWER(TRIM(?))) - 
                (SELECT COUNT(*) FROM media WHERE DATE(created_at) = ? AND LOWER(TRIM(type)) = LOWER(TRIM(?)))
        `
    case "chapters":
        query = `
            SELECT 
                (SELECT COUNT(*) FROM chapters c INNER JOIN media m ON c.media_slug = m.slug WHERE DATE(c.created_at) = ? AND LOWER(TRIM(m.type)) = LOWER(TRIM(?))) - 
                (SELECT COUNT(*) FROM chapters c INNER JOIN media m ON c.media_slug = m.slug WHERE DATE(c.created_at) = ? AND LOWER(TRIM(m.type)) = LOWER(TRIM(?)))
        `
    case "chapters_read":
        query = `
            SELECT 
                (SELECT COUNT(*) FROM reading_states rs INNER JOIN media m ON rs.media_slug = m.slug WHERE DATE(rs.created_at) = ? AND LOWER(TRIM(m.type)) = LOWER(TRIM(?))) - 
                (SELECT COUNT(*) FROM reading_states rs INNER JOIN media m ON rs.media_slug = m.slug WHERE DATE(rs.created_at) = ? AND LOWER(TRIM(m.type)) = LOWER(TRIM(?)))
        `
    default:
        return 0, fmt.Errorf("unknown stat type: %s", statType)
    }
    
    row := db.QueryRow(query, today, mediaType, yesterday, mediaType)
    var change int
    err := row.Scan(&change)
    return change, err
}

func ensureTodaysStatsRecorded(today string) error {
    // Always record today's statistics to keep them up to date
    return RecordDailyStatistics()
}

// GetTopReadMedias returns the top media by reading activity for the given period
func GetTopReadMedias(period string, limit int) ([]Media, error) {
    var dateFilter string
    switch period {
    case "today":
        dateFilter = "AND rs.created_at >= datetime('now', 'start of day')"
    case "week":
        dateFilter = "AND rs.created_at >= datetime('now', '-7 days', 'start of day')"
    case "month":
        dateFilter = "AND rs.created_at >= datetime('now', '-1 month', 'start of day')"
    case "year":
        dateFilter = "AND rs.created_at >= datetime('now', '-1 year', 'start of day')"
    case "all":
        dateFilter = ""
    default:
        return nil, fmt.Errorf("invalid period: %s", period)
    }

    query := fmt.Sprintf(`
        SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count, top_reads.read_count, m.created_at, m.updated_at
        FROM media m
        INNER JOIN (
            SELECT media_slug, COUNT(*) as read_count
            FROM reading_states rs
            WHERE 1=1 %s
            GROUP BY media_slug
            ORDER BY read_count DESC
            LIMIT ?
        ) top_reads ON m.slug = top_reads.media_slug
        ORDER BY top_reads.read_count DESC
    `, dateFilter)

    rows, err := db.Query(query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    cfg, err := GetAppConfig()
    if err != nil {
        // If we can't get config, default to showing all content
        cfg.ContentRatingLimit = 3
    }

    var media []Media
    for rows.Next() {
        var m Media
        var createdAt, updatedAt int64
        var year sql.NullInt64
        err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &m.ReadCount, &createdAt, &updatedAt)
        if err != nil {
            return nil, err
        }
        if year.Valid {
            m.Year = int(year.Int64)
        } else {
            m.Year = 0
        }
        m.CreatedAt = time.Unix(createdAt, 0)
        m.UpdatedAt = time.Unix(updatedAt, 0)

        if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
            media = append(media, m)
        }
    }

    return media, rows.Err()
}
