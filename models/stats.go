package models

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// SeriesData represents a series with its count/value
type SeriesData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SystemStats holds system resource statistics
type SystemStats struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	CPUFrequencyGHz    float64 `json:"cpu_frequency_ghz"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	MemoryUsedMB       float64 `json:"memory_used_mb"`
	MemoryTotalMB      float64 `json:"memory_total_mb"`
}

// DiskStats holds disk usage statistics for a single disk
type DiskStats struct {
	Path         string  `json:"path"`
	UsedGB       float64 `json:"used_gb"`
	TotalGB      float64 `json:"total_gb"`
	UsagePercent float64 `json:"usage_percent"`
	AvailableGB  float64 `json:"available_gb"`
}

// MonitoringData holds all monitoring dashboard data
type MonitoringData struct {
	UserData             string
	TagData              string
	RoleData             string
	ReadingData          string
	PopularReads         string
	PopularFavorites     string
	PopularVotes         string
	CommentsActivity     string
	ReviewsActivity      string
	TopCommented         string
	TopReviewed          string
	VoteDistribution     string
	ControversialSeries  string
	ChaptersDistribution string
	MostActiveReaders    string
	ActivityByMediaType  string
	NewMediaOverTime     string
	NewChaptersOverTime  string
	MediaGrowthByType    string
	SystemStats          string
	DiskStats            string
}

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
	cfg, err := GetAppConfig()
	if err != nil {
		// If we can't get config, default to showing all content
		cfg.ContentRatingLimit = 3
	}

	var allowedRatings []string
	switch cfg.ContentRatingLimit {
	case 0:
		allowedRatings = []string{"safe"}
	case 1:
		allowedRatings = []string{"safe", "suggestive"}
	case 2:
		allowedRatings = []string{"safe", "suggestive", "erotica"}
	default:
		allowedRatings = []string{"safe", "suggestive", "erotica", "pornographic"}
	}

	placeholders := strings.Repeat("?,", len(allowedRatings))
	placeholders = placeholders[:len(placeholders)-1] // remove trailing comma

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
        WHERE m.content_rating IN (%s)
        ORDER BY top_reads.read_count DESC
    `, dateFilter, placeholders)

	args := make([]interface{}, len(allowedRatings)+1)
	args[0] = limit
	for i, rating := range allowedRatings {
		args[i+1] = rating
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

		media = append(media, m)
	}

	return media, rows.Err()
}

// User-specific statistics functions

// GetUserTotalChaptersRead returns the total number of chapters read by a user
func GetUserTotalChaptersRead(userName string) (int, error) {
	var count int
	row := db.QueryRow(`SELECT COUNT(*) FROM reading_states WHERE user_name = ?`, userName)
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetUserTotalMediaRead returns the total number of distinct media read by a user
func GetUserTotalMediaRead(userName string) (int, error) {
	var count int
	row := db.QueryRow(`SELECT COUNT(DISTINCT media_slug) FROM reading_states WHERE user_name = ?`, userName)
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetUserReadingStreak returns the current reading streak in days
func GetUserReadingStreak(userName string) (int, error) {
	// Get the most recent reading date
	var latestDate time.Time
	row := db.QueryRow(`
		SELECT DATE(MAX(created_at))
		FROM reading_states
		WHERE user_name = ?
	`, userName)

	if err := row.Scan(&latestDate); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	// If the latest reading is not today or yesterday, streak is broken
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	if latestDate.Before(yesterday) {
		return 0, nil
	}

	// Count consecutive days with readings
	streak := 0
	currentDate := today

	for {
		var count int
		row := db.QueryRow(`
			SELECT COUNT(*)
			FROM reading_states
			WHERE user_name = ? AND DATE(created_at) = DATE(?)
		`, userName, currentDate)

		if err := row.Scan(&count); err != nil {
			return 0, err
		}

		if count == 0 {
			break
		}

		streak++
		currentDate = currentDate.AddDate(0, 0, -1)
	}

	return streak, nil
}

// GetUserFavoriteGenres returns the top 5 genres based on user's reading history
func GetUserFavoriteGenres(userName string) ([]string, error) {
	rows, err := db.Query(`
		SELECT m.genres, COUNT(*) as read_count
		FROM reading_states rs
		JOIN media m ON rs.media_slug = m.slug
		WHERE rs.user_name = ? AND m.genres != ''
		GROUP BY m.genres
		ORDER BY read_count DESC
		LIMIT 5
	`, userName)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []string
	for rows.Next() {
		var genreList string
		var count int
		if err := rows.Scan(&genreList, &count); err != nil {
			return nil, err
		}
		// Split comma-separated genres and take the first one as representative
		if strings.Contains(genreList, ",") {
			genres = append(genres, strings.TrimSpace(strings.Split(genreList, ",")[0]))
		} else {
			genres = append(genres, strings.TrimSpace(genreList))
		}
	}

	return genres, nil
}

// GetReadingActivityOverTime returns daily reading activity for the last N days
func GetReadingActivityOverTime(days int) (map[string]int, error) {
	query := `
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM reading_states
		WHERE created_at >= datetime('now', '-' || ? || ' days')
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at)
	`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activity := make(map[string]int)
	for rows.Next() {
		var date sql.NullString
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		if date.Valid {
			activity[date.String] = count
		}
	}
	return activity, nil
}

// GetTopPopularSeriesByReads returns the top N series by total chapters read
func GetTopPopularSeriesByReads(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name, COUNT(rs.id) as read_count
		FROM media m
		INNER JOIN reading_states rs ON m.slug = rs.media_slug
		GROUP BY m.slug, m.name
		ORDER BY read_count DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		series = append(series, data)
	}
	return series, nil
}

// GetTopPopularSeriesByFavorites returns the top N series by favorite count
func GetTopPopularSeriesByFavorites(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name, COUNT(f.id) as favorite_count
		FROM media m
		INNER JOIN favorites f ON m.slug = f.media_slug
		GROUP BY m.slug, m.name
		ORDER BY favorite_count DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		series = append(series, data)
	}
	return series, nil
}

// GetTopPopularSeriesByVotes returns the top N series by vote score
func GetTopPopularSeriesByVotes(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name,
			   COALESCE(SUM(CASE WHEN v.value = 1 THEN 1 ELSE 0 END), 0) - 
			   COALESCE(SUM(CASE WHEN v.value = -1 THEN 1 ELSE 0 END), 0) as vote_score
		FROM media m
		LEFT JOIN votes v ON m.slug = v.media_slug
		GROUP BY m.slug, m.name
		HAVING vote_score > 0
		ORDER BY vote_score DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		series = append(series, data)
	}
	return series, nil
}

// GetCommentsActivityOverTime returns daily comment activity for the last N days
// GetCommentsActivityOverTime returns daily comment activity for the last N days
func GetCommentsActivityOverTime(days int) (map[string]int, error) {
	query := `
		SELECT DATE(datetime(created_at, 'unixepoch')) as date, COUNT(*) as count
		FROM comments
		WHERE created_at >= strftime('%s', 'now', '-' || ? || ' days')
		GROUP BY DATE(datetime(created_at, 'unixepoch'))
		ORDER BY DATE(datetime(created_at, 'unixepoch'))
	`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activity := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		activity[date] = count
	}
	return activity, nil
}

// GetReviewsActivityOverTime returns daily review activity for the last N days
// GetReviewsActivityOverTime returns daily review activity for the last N days
func GetReviewsActivityOverTime(days int) (map[string]int, error) {
	query := `
		SELECT DATE(datetime(created_at, 'unixepoch')) as date, COUNT(*) as count
		FROM reviews
		WHERE created_at >= strftime('%s', 'now', '-' || ? || ' days')
		GROUP BY DATE(datetime(created_at, 'unixepoch'))
		ORDER BY DATE(datetime(created_at, 'unixepoch'))
	`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activity := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		activity[date] = count
	}
	return activity, nil
}

// GetTopSeriesByComments returns the top N series by comment count
func GetTopSeriesByComments(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name, COUNT(c.id) as comment_count
		FROM media m
		INNER JOIN comments c ON m.slug = c.media_slug
		GROUP BY m.slug, m.name
		ORDER BY comment_count DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		series = append(series, data)
	}
	return series, nil
}

// GetTopSeriesByReviews returns the top N series by review count
func GetTopSeriesByReviews(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name, COUNT(r.id) as review_count
		FROM media m
		INNER JOIN reviews r ON m.slug = r.media_slug
		GROUP BY m.slug, m.name
		ORDER BY review_count DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		series = append(series, data)
	}
	return series, nil
}

// GetVoteDistribution returns total upvotes and downvotes across all content
func GetVoteDistribution() (upvotes int, downvotes int, err error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END), 0) as upvotes,
			COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END), 0) as downvotes
		FROM votes
	`
	row := db.QueryRow(query)
	if err := row.Scan(&upvotes, &downvotes); err != nil {
		return 0, 0, err
	}
	return upvotes, downvotes, nil
}

// GetMostControversialSeries returns series with high vote variance (close to 50/50 split)
// Only includes series with at least 5 total votes
func GetMostControversialSeries(limit int) ([]SeriesData, error) {
	query := `
		SELECT m.name,
			   (COALESCE(SUM(CASE WHEN v.value = 1 THEN 1 ELSE 0 END), 0) + 
			    COALESCE(SUM(CASE WHEN v.value = -1 THEN 1 ELSE 0 END), 0)) as total_votes,
			   ABS(COALESCE(SUM(CASE WHEN v.value = 1 THEN 1 ELSE 0 END), 0) - 
			   	   COALESCE(SUM(CASE WHEN v.value = -1 THEN 1 ELSE 0 END), 0)) as vote_diff
		FROM media m
		LEFT JOIN votes v ON m.slug = v.media_slug
		GROUP BY m.slug, m.name
		HAVING total_votes >= 5
		ORDER BY (vote_diff * 1.0 / total_votes) ASC, total_votes DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var data SeriesData
		var totalVotes, voteDiff int
		if err := rows.Scan(&data.Name, &totalVotes, &voteDiff); err != nil {
			return nil, err
		}
		// Use the controversy score (lower diff/total means more controversial)
		data.Count = int((float64(voteDiff) / float64(totalVotes)) * 100) // percentage difference
		series = append(series, data)
	}
	return series, nil
}

// GetChaptersReadPerUserDistribution returns distribution of chapters read per user
// Returns data suitable for a histogram showing how many users read X chapters
func GetChaptersReadPerUserDistribution() (map[string]int, error) {
	query := `
		SELECT 
			CASE
				WHEN chapter_count = 0 THEN '0'
				WHEN chapter_count BETWEEN 1 AND 5 THEN '1-5'
				WHEN chapter_count BETWEEN 6 AND 10 THEN '6-10'
				WHEN chapter_count BETWEEN 11 AND 20 THEN '11-20'
				WHEN chapter_count BETWEEN 21 AND 50 THEN '21-50'
				WHEN chapter_count BETWEEN 51 AND 100 THEN '51-100'
				ELSE '100+'
			END as range_bucket,
			COUNT(*) as user_count
		FROM (
			SELECT user_name, COUNT(*) as chapter_count
			FROM reading_states
			GROUP BY user_name
		)
		GROUP BY range_bucket
		ORDER BY CASE 
			WHEN range_bucket = '0' THEN 0
			WHEN range_bucket = '1-5' THEN 1
			WHEN range_bucket = '6-10' THEN 2
			WHEN range_bucket = '11-20' THEN 3
			WHEN range_bucket = '21-50' THEN 4
			WHEN range_bucket = '51-100' THEN 5
			ELSE 6
		END
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[string]int)
	for rows.Next() {
		var rangeBucket string
		var count int
		if err := rows.Scan(&rangeBucket, &count); err != nil {
			return nil, err
		}
		distribution[rangeBucket] = count
	}
	return distribution, nil
}

// GetMostActiveReaders returns the top N most active readers
func GetMostActiveReaders(limit int) ([]SeriesData, error) {
	query := `
		SELECT user_name, COUNT(*) as chapter_count
		FROM reading_states
		GROUP BY user_name
		ORDER BY chapter_count DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readers []SeriesData
	for rows.Next() {
		var data SeriesData
		if err := rows.Scan(&data.Name, &data.Count); err != nil {
			return nil, err
		}
		readers = append(readers, data)
	}
	return readers, nil
}

// GetAverageChaptersReadPerUser returns the average chapters read per user
func GetAverageChaptersReadPerUser() (float64, error) {
	query := `
		SELECT AVG(chapter_count)
		FROM (
			SELECT user_name, COUNT(*) as chapter_count
			FROM reading_states
			GROUP BY user_name
		)
	`

	row := db.QueryRow(query)
	var avg sql.NullFloat64
	if err := row.Scan(&avg); err != nil {
		return 0, err
	}
	if avg.Valid {
		return avg.Float64, nil
	}
	return 0, nil
}

// GetUserActivityByMediaType returns chapter reads grouped by media type
func GetUserActivityByMediaType() (map[string]int, error) {
	query := `
		SELECT m.type, COUNT(*) as read_count
		FROM reading_states rs
		INNER JOIN media m ON rs.media_slug = m.slug
		WHERE m.type IS NOT NULL AND TRIM(m.type) != ''
		GROUP BY m.type
		ORDER BY read_count DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activityByType := make(map[string]int)
	for rows.Next() {
		var mediaType string
		var count int
		if err := rows.Scan(&mediaType, &count); err != nil {
			return nil, err
		}
		activityByType[mediaType] = count
	}
	return activityByType, nil
}

// GetNewMediaOverTime returns daily new media added for the last N days
func GetNewMediaOverTime(days int) (map[string]int, error) {
	query := `
		SELECT DATE(datetime(created_at, 'unixepoch')) as date, COUNT(*) as count
		FROM media
		WHERE created_at >= strftime('%s', 'now', '-' || ? || ' days')
		GROUP BY DATE(datetime(created_at, 'unixepoch'))
		ORDER BY DATE(datetime(created_at, 'unixepoch'))
	`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	newMedia := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		newMedia[date] = count
	}
	return newMedia, nil
}

// GetNewChaptersOverTime returns daily new chapters added for the last N days
func GetNewChaptersOverTime(days int) (map[string]int, error) {
	query := `
		SELECT DATE(datetime(created_at, 'unixepoch')) as date, COUNT(*) as count
		FROM chapters
		WHERE created_at >= strftime('%s', 'now', '-' || ? || ' days')
		GROUP BY DATE(datetime(created_at, 'unixepoch'))
		ORDER BY DATE(datetime(created_at, 'unixepoch'))
	`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	newChapters := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		newChapters[date] = count
	}
	return newChapters, nil
}

// GetMediaGrowthByType returns the count of media grouped by type
func GetMediaGrowthByType() ([]SeriesData, error) {
	query := `
		SELECT type, COUNT(*) as count
		FROM media
		WHERE type IS NOT NULL AND TRIM(type) != ''
		GROUP BY type
		ORDER BY count DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SeriesData
	for rows.Next() {
		var mediaType string
		var count int
		if err := rows.Scan(&mediaType, &count); err != nil {
			return nil, err
		}
		result = append(result, SeriesData{Name: mediaType, Count: count})
	}
	return result, nil
}

// GetSystemStats retrieves CPU and memory usage statistics
func GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Convert bytes to MB
	stats.MemoryUsedMB = float64(m.Alloc) / 1024 / 1024
	stats.MemoryTotalMB = float64(m.TotalAlloc) / 1024 / 1024

	// Try to get system memory info from /proc/meminfo on Linux
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			var memTotal, memAvailable uint64
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					fmt.Sscanf(line, "MemTotal: %d", &memTotal)
				}
				if strings.HasPrefix(line, "MemAvailable:") {
					fmt.Sscanf(line, "MemAvailable: %d", &memAvailable)
				}
			}
			if memTotal > 0 {
				memTotalMB := float64(memTotal) / 1024
				memAvailableMB := float64(memAvailable) / 1024
				memUsedMB := memTotalMB - memAvailableMB

				stats.MemoryUsedMB = memUsedMB
				stats.MemoryTotalMB = memTotalMB
				stats.MemoryUsagePercent = (memUsedMB / memTotalMB) * 100
			}
		}

		// Try to get CPU usage from /proc/stat
		if cpuPercent, err := getCPUUsage(); err == nil {
			stats.CPUUsagePercent = cpuPercent
		}

		// Try to get CPU frequency from /proc/cpuinfo
		if cpuFreqGHz, err := getCPUFrequency(); err == nil {
			stats.CPUFrequencyGHz = cpuFreqGHz
		}
	} else {
		// Fallback: use Go runtime stats
		stats.MemoryUsagePercent = float64(m.Alloc) / float64(m.Sys) * 100
		if stats.MemoryUsagePercent > 100 {
			stats.MemoryUsagePercent = 100
		}
	}

	return stats, nil
}

// getCPUUsage reads CPU usage from /proc/stat
func getCPUUsage() (float64, error) {
	// Read first snapshot
	stat1, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	// Read second snapshot
	stat2, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	// Calculate CPU usage
	totalDiff := stat2.Total - stat1.Total
	idleDiff := stat2.Idle - stat1.Idle

	if totalDiff == 0 {
		return 0, nil
	}

	cpuUsage := float64(totalDiff-idleDiff) / float64(totalDiff) * 100
	if cpuUsage < 0 {
		cpuUsage = 0
	}
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	return cpuUsage, nil
}

type cpuStats struct {
	Total uint64
	Idle  uint64
}

func readCPUStats() (*cpuStats, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no cpu info found")
	}

	var user, nice, system, idle, iowait, irq, softirq uint64
	cpuLine := lines[0]

	_, err = fmt.Sscanf(cpuLine, "cpu %d %d %d %d %d %d %d",
		&user, &nice, &system, &idle, &iowait, &irq, &softirq)

	if err != nil {
		return nil, err
	}

	total := user + nice + system + idle + iowait + irq + softirq

	return &cpuStats{
		Total: total,
		Idle:  idle,
	}, nil
}

// getCPUFrequency reads CPU frequency in GHz from /proc/cpuinfo
func getCPUFrequency() (float64, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "cpu MHz") {
			// Parse the MHz value
			var cpuMHz float64
			_, err := fmt.Sscanf(line, "cpu MHz : %f", &cpuMHz)
			if err == nil && cpuMHz > 0 {
				// Convert MHz to GHz
				return cpuMHz / 1000.0, nil
			}
		}
	}

	return 0, fmt.Errorf("CPU frequency not found")
}

// GetDiskStats retrieves disk usage statistics for main logical disks (not mount points)
func GetDiskStats() ([]DiskStats, error) {
	var disks []DiskStats

	if runtime.GOOS == "windows" {
		// Windows disk stats not implemented yet
		return disks, nil
	} else if runtime.GOOS == "linux" {
		return getDiskUsageLinux()
	} else {
		return disks, nil
	}
}
