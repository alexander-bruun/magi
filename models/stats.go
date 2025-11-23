package models

import (
	"fmt"
	"time"
)

// Simple DB-backed counters for homepage statistics
func GetTotalMangas() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM mangas`)
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

// GetChaptersReadCount returns the number of reading_state records for a given manga
func GetChaptersReadCount(mangaSlug string) (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states WHERE manga_slug = ?`, mangaSlug)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalLightNovels() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM light_novels`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalLightNovelChapters() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE manga_slug IN (SELECT slug FROM light_novels)`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalLightNovelChaptersRead() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states WHERE manga_slug IN (SELECT slug FROM light_novels)`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func RecordDailyStatistics() error {
    today := time.Now().Format("2006-01-02")
    
    totalMangas, err := GetTotalMangas()
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
    totalLightNovels, err := GetTotalLightNovels()
    if err != nil {
        return err
    }
    totalLightNovelChapters, err := GetTotalLightNovelChapters()
    if err != nil {
        return err
    }
    totalLightNovelChaptersRead, err := GetTotalLightNovelChaptersRead()
    if err != nil {
        return err
    }

    query := `
        INSERT OR REPLACE INTO daily_statistics 
        (date, total_mangas, total_chapters, total_chapters_read, total_light_novels, total_light_novel_chapters, total_light_novel_chapters_read)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `
    _, err = db.Exec(query, today, totalMangas, totalChapters, totalChaptersRead, totalLightNovels, totalLightNovelChapters, totalLightNovelChaptersRead)
    return err
}

func GetDailyChange(statType string) (int, error) {
    today := time.Now().Format("2006-01-02")
    yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
    
    // First, ensure today's statistics are recorded
    if err := ensureTodaysStatsRecorded(today); err != nil {
        return 0, err
    }
    
    var column string
    switch statType {
    case "mangas":
        column = "total_mangas"
    case "chapters":
        column = "total_chapters"
    case "chapters_read":
        column = "total_chapters_read"
    case "light_novels":
        column = "total_light_novels"
    case "light_novel_chapters":
        column = "total_light_novel_chapters"
    case "light_novel_chapters_read":
        column = "total_light_novel_chapters_read"
    default:
        return 0, fmt.Errorf("unknown stat type: %s", statType)
    }

    query := fmt.Sprintf(`
        SELECT COALESCE((SELECT %s FROM daily_statistics WHERE date = ?), 0) - 
               COALESCE((SELECT %s FROM daily_statistics WHERE date = ?), 0)
    `, column, column)
    
    row := db.QueryRow(query, today, yesterday)
    var change int
    err := row.Scan(&change)
    return change, err
}

func ensureTodaysStatsRecorded(today string) error {
    // Always record today's statistics to keep them up to date
    return RecordDailyStatistics()
}

// GetTopReadMangas returns the top mangas by reading activity for the given period
func GetTopReadMangas(period string, limit int) ([]Manga, error) {
    var dateFilter string
    switch period {
    case "today":
        dateFilter = "AND rs.created_at >= strftime('%s', 'now', 'start of day')"
    case "week":
        dateFilter = "AND rs.created_at >= strftime('%s', 'now', '-7 days', 'start of day')"
    case "month":
        dateFilter = "AND rs.created_at >= strftime('%s', 'now', '-1 month', 'start of day')"
    case "year":
        dateFilter = "AND rs.created_at >= strftime('%s', 'now', '-1 year', 'start of day')"
    case "all":
        dateFilter = ""
    default:
        return nil, fmt.Errorf("invalid period: %s", period)
    }

    query := fmt.Sprintf(`
        SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.manga_type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count, top_reads.read_count, m.created_at, m.updated_at
        FROM mangas m
        INNER JOIN (
            SELECT manga_slug, COUNT(*) as read_count
            FROM reading_states rs
            WHERE 1=1 %s
            GROUP BY manga_slug
            ORDER BY read_count DESC
            LIMIT ?
        ) top_reads ON m.slug = top_reads.manga_slug
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

    var mangas []Manga
    for rows.Next() {
        var m Manga
        var createdAt, updatedAt int64
        err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL, &m.Path, &m.FileCount, &m.ReadCount, &createdAt, &updatedAt)
        if err != nil {
            return nil, err
        }
        m.CreatedAt = time.Unix(createdAt, 0)
        m.UpdatedAt = time.Unix(updatedAt, 0)

        if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
            mangas = append(mangas, m)
        }
    }

    return mangas, rows.Err()
}
