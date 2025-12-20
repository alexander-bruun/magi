package models

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

// CalculateCountdownText calculates the countdown text for a premium chapter
func CalculateCountdownText(releaseTime time.Time) string {
	if time.Now().After(releaseTime) || time.Now().Equal(releaseTime) {
		return "Available now!"
	}
	
	duration := time.Until(releaseTime)
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	} else if duration.Hours() >= 1 {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		result := fmt.Sprintf("%dh", hours)
		if minutes > 0 {
			result += fmt.Sprintf(" %dm", minutes)
		}
		return result
	} else {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if minutes > 0 {
			result := fmt.Sprintf("%dm", minutes)
			if seconds > 0 {
				result += fmt.Sprintf(" %ds", seconds)
			}
			return result
		} else {
			return fmt.Sprintf("%ds", seconds)
		}
	}
}

// Chapter represents the chapter table schema
type Chapter struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	File            string `json:"file"`
	ChapterCoverURL string `json:"chapter_cover_url"`
	MediaSlug       string `json:"media_slug"`
	Read            bool   `json:"read"`
	ReadCount       int    `json:"read_count"`
	CreatedAt       time.Time `json:"created_at"`
	ReleasedAt      *time.Time `json:"released_at,omitempty"`
	IsPremium       bool   `json:"is_premium"`
	PremiumCountdown string `json:"premium_countdown,omitempty"`
}

// CreateChapter adds a new chapter if it does not already exist
func CreateChapter(chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("chapter already exists")
	}

	query := `
	INSERT INTO chapters (slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`

	timestamps := NewTimestamps()
	chapter.CreatedAt = timestamps.CreatedAt
	createdAt := timestamps.CreatedAt.Unix()

	_, err = db.Exec(query, chapter.Slug, chapter.Name, chapter.Type, chapter.File, chapter.ChapterCoverURL, chapter.MediaSlug, createdAt)
	if err != nil {
		return err
	}

	return nil
}

// CreateChapterTx adds a new chapter within a transaction
func CreateChapterTx(tx *sql.Tx, chapter Chapter) error {
	chapter.Slug = utils.Sluggify(chapter.Name)
	exists, err := ChapterExists(chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("chapter already exists")
	}

	query := `
	INSERT INTO chapters (slug, name, type, file, chapter_cover_url, media_slug, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	timestamps := NewTimestamps()
	chapter.CreatedAt = timestamps.CreatedAt
	createdAt := timestamps.CreatedAt.Unix()

	_, err = tx.Exec(query, chapter.Slug, chapter.Name, chapter.Type, chapter.File, chapter.ChapterCoverURL, chapter.MediaSlug, createdAt)
	if err != nil {
		return err
	}

	return nil
}

// UpdateChapterNameTx updates the name of a chapter in a transaction
func UpdateChapterNameTx(tx *sql.Tx, mediaSlug, chapterSlug, newName string) error {
	query := `
	UPDATE chapters
	SET name = ?
	WHERE media_slug = ? AND slug = ?
	`
	_, err := tx.Exec(query, newName, mediaSlug, chapterSlug)
	return err
}

// GetChapters retrieves all chapters for a specific manga, sorted by name
func GetChapters(mangaSlug string) ([]Chapter, error) {
	query := `
	SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at
	FROM chapters
	WHERE media_slug = ?
	`

	rows, err := db.Query(query, mangaSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []Chapter
	for rows.Next() {
		var chapter Chapter
		var createdAt int64
		var releasedAt sql.NullInt64
		if err := rows.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MediaSlug, &createdAt, &releasedAt); err != nil {
			return nil, err
		}
		chapter.CreatedAt = time.Unix(createdAt, 0)
		if releasedAt.Valid {
			t := time.Unix(releasedAt.Int64, 0)
			chapter.ReleasedAt = &t
		}
		chapters = append(chapters, chapter)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortChaptersByNumber(chapters)

	// Set IsPremium
	cfg, err := GetAppConfig()
	if err != nil {
		// If config error, don't set premium
		return chapters, nil
	}
	now := time.Now()
	// Since sorted ascending, the highest numbers are at the end
	for i := len(chapters) - 1; i >= 0 && i >= len(chapters)-cfg.MaxPremiumChapters; i-- {
		if chapters[i].ReleasedAt != nil {
			chapters[i].IsPremium = false
		} else {
			releaseTime := chapters[i].CreatedAt.Add(time.Duration(cfg.PremiumEarlyAccessDuration) * time.Second)
			chapters[i].IsPremium = now.Before(releaseTime)
		}
	}

	return chapters, nil
}

// GetChapter retrieves a specific chapter by its slug
func GetChapter(mangaSlug, chapterSlug string) (*Chapter, error) {
	query := `
	SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at
	FROM chapters
	WHERE media_slug = ? AND slug = ?
	`

	row := db.QueryRow(query, mangaSlug, chapterSlug)

	var chapter Chapter
	var createdAt int64
	var releasedAt sql.NullInt64
	err := row.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MediaSlug, &createdAt, &releasedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No chapter found
		}
		return nil, err
	}

	chapter.CreatedAt = time.Unix(createdAt, 0)
	if releasedAt.Valid {
		t := time.Unix(releasedAt.Int64, 0)
		chapter.ReleasedAt = &t
	}

	return &chapter, nil
}

// UpdateChapter modifies an existing chapter
func UpdateChapter(chapter *Chapter) error {
	query := `UPDATE chapters SET name = ?, type = ?, file = ?, chapter_cover_url = ? WHERE media_slug = ? AND slug = ?`

	_, err := db.Exec(query, chapter.Name, chapter.Type, chapter.File, chapter.ChapterCoverURL, chapter.MediaSlug, chapter.Slug)
	if err != nil {
		return err
	}

	return nil
}

// UpdateChapterCreatedAt updates the created_at timestamp for a chapter
func UpdateChapterCreatedAt(mediaSlug, chapterSlug string, createdAt time.Time) error {
	query := `
	UPDATE chapters
	SET created_at = ?
	WHERE media_slug = ? AND slug = ?
	`

	_, err := db.Exec(query, createdAt.Unix(), mediaSlug, chapterSlug)
	if err != nil {
		return err
	}

	return nil
}

func UpdateChapterReleasedAt(mediaSlug, chapterSlug string, releasedAt time.Time) error {
	query := `
	UPDATE chapters
	SET released_at = ?
	WHERE media_slug = ? AND slug = ?
	`

	_, err := db.Exec(query, releasedAt.Unix(), mediaSlug, chapterSlug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteChapterTx removes a specific chapter within a transaction
func DeleteChapterTx(tx *sql.Tx, mangaSlug, chapterSlug string) error {
	return DeleteRecordTx(tx, `DELETE FROM chapters WHERE media_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
}

// DeleteChapter removes a specific chapter
func DeleteChapter(mangaSlug, chapterSlug string) error {
	return DeleteRecord(`DELETE FROM chapters WHERE media_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
}

// DeleteChaptersByMediaSlug removes all chapters for a specific manga
func DeleteChaptersByMediaSlug(mangaSlug string) error {
	return DeleteRecord(`DELETE FROM chapters WHERE media_slug = ?`, mangaSlug)
}

// ChapterExists checks if a chapter already exists
func ChapterExists(chapterSlug, mangaSlug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM chapters WHERE media_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
}

// isChapterAccessibleForUser checks if a chapter is accessible to the user
func isChapterAccessibleForUser(chapter *Chapter, userName string) bool {
	// If released_at is set, it's released
	if chapter.ReleasedAt != nil {
		return true
	}

	if userName == "" {
		// Anonymous user
		if !chapter.IsPremium {
			// Non-premium chapters are accessible to everyone
			return true
		}

		// For premium chapters, check if anonymous role has premium chapter access
		hasAccess, err := RoleHasAccess("anonymous")
		if err != nil {
			log.Errorf("Failed to check premium chapter access for anonymous role: %v", err)
			return false
		}
		return hasAccess
	}

	// For logged-in users
	if !chapter.IsPremium {
		// Non-premium chapters are accessible to everyone
		return true
	}

	// For premium chapters, check if user has premium chapter access via permissions
	hasAccess, err := UserHasPremiumChapterAccess(userName)
	if err != nil {
		log.Errorf("Failed to check premium chapter access for user %s: %v", userName, err)
		return false
	}
	return hasAccess
}

// GetAdjacentChapters finds the previous and next chapters based on the current chapter slug
func GetAdjacentChapters(chapters []Chapter, chapterSlug, userName string) (prevSlug, nextSlug string, err error) {
	// Filter chapters to only include accessible ones
	var accessibleChapters []Chapter
	for _, chapter := range chapters {
		if isChapterAccessibleForUser(&chapter, userName) {
			accessibleChapters = append(accessibleChapters, chapter)
		}
	}

	currentIndex := indexOfChapter(accessibleChapters, chapterSlug)
	if currentIndex == -1 {
		return "", "", errors.New("chapter not found")
	}

	if currentIndex > 0 {
		prevSlug = accessibleChapters[currentIndex-1].Slug
	}
	if currentIndex < len(accessibleChapters)-1 {
		nextSlug = accessibleChapters[currentIndex+1].Slug
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

// ChapterWithMedia represents a chapter with its associated media information
type ChapterWithMedia struct {
	Chapter
	MediaSlug   string `json:"media_slug"`
	MediaName   string `json:"media_name"`
	MediaType   string `json:"media_type"`
	CoverArtURL string `json:"cover_art_url"`
}

// GetRecentChapters returns the most recently created chapters with their media info
func GetRecentChapters(limit int) ([]ChapterWithMedia, error) {
	query := `
		SELECT c.slug, c.name, c.type, c.file, c.chapter_cover_url, c.media_slug, c.created_at,
		       m.name, m.type, m.cover_art_url
		FROM chapters c
		JOIN media m ON c.media_slug = m.slug
		ORDER BY c.created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []ChapterWithMedia
	for rows.Next() {
		var chapter ChapterWithMedia
		var createdAt int64
		err := rows.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MediaSlug, &createdAt,
			&chapter.MediaName, &chapter.MediaType, &chapter.CoverArtURL)
		if err != nil {
			return nil, err
		}
		chapter.CreatedAt = time.Unix(createdAt, 0)
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

// HasPremiumChapters checks if a media has any chapters that are still in the premium early access period
func HasPremiumChapters(mediaSlug string, maxPremiumChapters int, premiumDuration int, scalingEnabled bool) (bool, string, error) {
	chapters, err := GetChaptersByMediaSlug(mediaSlug, 1000, maxPremiumChapters, premiumDuration, scalingEnabled)
	if err != nil {
		return false, "", err
	}

	// Count total premium chapters
	premiumCount := 0
	for _, ch := range chapters {
		if ch.IsPremium {
			premiumCount++
		}
	}

	for _, chapter := range chapters {
		if chapter.IsPremium {
			// Use the same multiplier as GetChaptersByMediaSlug for the first premium chapter
			multiplier := premiumCount
			if !scalingEnabled {
				multiplier = 1
			}
			scaledDuration := premiumDuration * multiplier
			
			// Calculate countdown for this chapter
			releaseTime := chapter.CreatedAt.Add(time.Duration(scaledDuration) * time.Second)
			countdown := CalculateCountdownText(releaseTime)
			return true, countdown, nil
		}
	}

	return false, "", nil
}

// GetLatestChapter returns the slug and name of the chapter with the highest number for a media
func GetLatestChapter(mediaSlug string) (string, string, error) {
	chapters, err := GetChapters(mediaSlug)
	if err != nil {
		return "", "", err
	}
	if len(chapters) == 0 {
		return "", "", nil
	}

	maxNum := -1
	var latestSlug, latestName string

	for _, ch := range chapters {
		num := extractChapterNumber(ch.Name)
		if num > maxNum {
			maxNum = num
			latestSlug = ch.Slug
			latestName = ch.Name
		}
	}

	return latestSlug, latestName, nil
}

// GetChaptersByMediaSlug returns the highest numbered chapters for a media (limited by count)
func GetChaptersByMediaSlug(mediaSlug string, limit int, maxPremiumChapters int, premiumDuration int, scalingEnabled bool) ([]Chapter, error) {
	query := `
		SELECT c.slug, c.name, c.type, c.file, c.chapter_cover_url, c.media_slug, c.created_at, c.released_at,
		       COALESCE(rs.read_count, 0) as read_count
		FROM chapters c
		LEFT JOIN (
			SELECT chapter_slug, COUNT(*) as read_count
			FROM reading_states
			WHERE media_slug = ?
			GROUP BY chapter_slug
		) rs ON c.slug = rs.chapter_slug
		WHERE c.media_slug = ?
	`

	rows, err := db.Query(query, mediaSlug, mediaSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []Chapter
	for rows.Next() {
		var chapter Chapter
		var createdAt int64
		var releasedAt sql.NullInt64
		if err := rows.Scan(&chapter.Slug, &chapter.Name, &chapter.Type, &chapter.File, &chapter.ChapterCoverURL, &chapter.MediaSlug, &createdAt, &releasedAt, &chapter.ReadCount); err != nil {
			return nil, err
		}
		chapter.CreatedAt = time.Unix(createdAt, 0)
		if releasedAt.Valid {
			t := time.Unix(releasedAt.Int64, 0)
			chapter.ReleasedAt = &t
		}
		chapters = append(chapters, chapter)
	}

	// Sort chapters by extracted chapter number descending
	sort.Slice(chapters, func(i, j int) bool {
		numI := extractChapterNumber(chapters[i].Name)
		numJ := extractChapterNumber(chapters[j].Name)
		return numI > numJ
	})

	// Set IsPremium for chapters within maxPremiumChapters and within time
	now := time.Now()
	premiumChapters := make([]int, 0) // Store indices of premium chapters
	for i := range chapters {
		if i < maxPremiumChapters {
			if chapters[i].ReleasedAt != nil {
				chapters[i].IsPremium = false
			} else {
				// First pass: determine which chapters are premium using base duration
				releaseTime := chapters[i].CreatedAt.Add(time.Duration(premiumDuration) * time.Second)
				chapters[i].IsPremium = now.Before(releaseTime)
				if chapters[i].IsPremium {
					premiumChapters = append(premiumChapters, i)
				}
			}
		} else {
			chapters[i].IsPremium = false
		}
	}
	
	// Second pass: calculate scaled durations for premium chapters
	// Newest premium chapter gets highest multiplier, oldest gets 1x
	for position, chapterIndex := range premiumChapters {
		multiplier := len(premiumChapters) - position
		if !scalingEnabled {
			multiplier = 1
		}
		scaledDuration := premiumDuration * multiplier
		releaseTime := chapters[chapterIndex].CreatedAt.Add(time.Duration(scaledDuration) * time.Second)
		
		// Recalculate IsPremium with scaled duration (in case it changed)
		chapters[chapterIndex].IsPremium = now.Before(releaseTime)
		
		// Calculate countdown for premium chapters
		chapters[chapterIndex].PremiumCountdown = CalculateCountdownText(releaseTime)
	}

	// Return only the top 'limit' chapters
	if len(chapters) > limit {
		chapters = chapters[:limit]
	}

	return chapters, nil
}

// MediaWithRecentChapters represents a media with its 3 most recent chapters
type MediaWithRecentChapters struct {
	Media    Media   `json:"media"`
	Chapters []Chapter `json:"chapters"`
}

// GetRecentSeriesWithChapters returns the most recently updated series with their 3 highest numbered chapters
func GetRecentSeriesWithChapters(limit int, maxPremiumChapters int, premiumDuration int, scalingEnabled bool) ([]MediaWithRecentChapters, error) {
	query := `
		SELECT m.slug, m.name, m.author, m.description, m.type, m.status, m.cover_art_url, m.created_at, m.updated_at
		FROM media m
		INNER JOIN chapters c ON c.media_slug = m.slug
		GROUP BY m.slug, m.name, m.author, m.description, m.type, m.status, m.cover_art_url, m.created_at, m.updated_at
		ORDER BY MAX(c.created_at) DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaSlugs []string
	mediaMap := make(map[string]Media)

	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Type, &m.Status, &m.CoverArtURL, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.Tags = []string{} // Initialize empty tags since we're not fetching them
		
		if _, exists := mediaMap[m.Slug]; !exists {
			mediaMap[m.Slug] = m
			mediaSlugs = append(mediaSlugs, m.Slug)
		}
	}

	// For each media, get the 3 most recent chapters
	var result []MediaWithRecentChapters
	for _, slug := range mediaSlugs {
		chapters, err := GetChaptersByMediaSlug(slug, 3, maxPremiumChapters, premiumDuration, scalingEnabled) // Get 3 highest numbered chapters
		if err != nil {
			return nil, err
		}
		
		result = append(result, MediaWithRecentChapters{
			Media:    mediaMap[slug],
			Chapters: chapters,
		})
	}

	return result, nil
}

// extractChapterNumber extracts the chapter number from a chapter name
func extractChapterNumber(name string) int {
	// Look for patterns like "Chapter 123", "Vol 1 Ch 123", "Volume 1", etc.
	re := regexp.MustCompile(`(?i)(?:chapter|ch\.?|episode|ep\.?|volume|vol\.?)\s*(\d+)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}
	// If no pattern, try to parse the whole name as number
	if num, err := strconv.Atoi(strings.TrimSpace(name)); err == nil {
		return num
	}
	return -1
}

// MediaEnrichmentData contains preloaded data for a media item
type MediaEnrichmentData struct {
	MediaSlug           string
	HasPremium          bool
	PremiumCountdown    string
	LatestChapterSlug   string
	LatestChapterName   string
	AverageRating       float64
	ReviewCount         int
}

// BatchEnrichMediaData fetches enrichment data for multiple media items in bulk
// This reduces N+1 queries by batch loading all chapters and ratings at once
func BatchEnrichMediaData(mediaSlugs []string, maxPremiumChapters int, premiumDuration int, scalingEnabled bool) (map[string]MediaEnrichmentData, error) {
	if len(mediaSlugs) == 0 {
		return make(map[string]MediaEnrichmentData), nil
	}

	result := make(map[string]MediaEnrichmentData)
	
	// Initialize result map for all slugs
	for _, slug := range mediaSlugs {
		result[slug] = MediaEnrichmentData{MediaSlug: slug}
	}

	// Batch fetch all chapters in one query
	placeholders := strings.Repeat("?,", len(mediaSlugs)-1) + "?"
	query := fmt.Sprintf(`
		SELECT c.media_slug, c.slug, c.name, c.created_at, c.type,
		       COALESCE(read_counts.read_count, 0) as read_count
		FROM chapters c
		LEFT JOIN (
			SELECT chapter_slug, COUNT(*) as read_count
			FROM reading_states
			GROUP BY chapter_slug
		) read_counts ON c.slug = read_counts.chapter_slug
		WHERE c.media_slug IN (%s)
		ORDER BY c.media_slug, c.created_at DESC
	`, placeholders)

	args := make([]interface{}, len(mediaSlugs))
	for i, slug := range mediaSlugs {
		args[i] = slug
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	// Group chapters by media slug
	chaptersByMedia := make(map[string][]Chapter)
	for rows.Next() {
		var mediaSlug, slug, name, chType string
		var createdAt int64
		var readCount int
		if err := rows.Scan(&mediaSlug, &slug, &name, &createdAt, &chType, &readCount); err != nil {
			return result, err
		}
		ch := Chapter{
			Slug:      slug,
			Name:      name,
			Type:      chType,
			MediaSlug: mediaSlug,
			ReadCount: readCount,
			CreatedAt: time.Unix(createdAt, 0),
			IsPremium: chType == "premium",
		}
		chaptersByMedia[mediaSlug] = append(chaptersByMedia[mediaSlug], ch)
	}

	// Batch fetch all ratings in one query
	ratingQuery := fmt.Sprintf(`
		SELECT media_slug, COALESCE(AVG(rating), 0), COUNT(*)
		FROM reviews
		WHERE media_slug IN (%s)
		GROUP BY media_slug
	`, placeholders)

	ratingsRows, err := db.Query(ratingQuery, args...)
	if err != nil {
		return result, err
	}
	defer ratingsRows.Close()

	ratingsByMedia := make(map[string][2]interface{})
	for ratingsRows.Next() {
		var mediaSlug string
		var avg float64
		var count int
		if err := ratingsRows.Scan(&mediaSlug, &avg, &count); err != nil {
			return result, err
		}
		ratingsByMedia[mediaSlug] = [2]interface{}{avg, count}
	}

	// Process each media slug
	for _, mediaSlug := range mediaSlugs {
		enrichData := result[mediaSlug]
		enrichData.MediaSlug = mediaSlug

		// Calculate premium status and countdown
		chapters := chaptersByMedia[mediaSlug]
		if len(chapters) > 0 {
			premiumCount := 0
			for _, ch := range chapters {
				if ch.IsPremium {
					premiumCount++
				}
			}

			for _, chapter := range chapters {
				if chapter.IsPremium {
					enrichData.HasPremium = true
					multiplier := premiumCount
					if !scalingEnabled {
						multiplier = 1
					}
					scaledDuration := premiumDuration * multiplier
					releaseTime := chapter.CreatedAt.Add(time.Duration(scaledDuration) * time.Second)
					enrichData.PremiumCountdown = CalculateCountdownText(releaseTime)
					break
				}
			}

			// Get latest chapter
			maxNum := -1
			for _, ch := range chapters {
				num := extractChapterNumber(ch.Name)
				if num > maxNum {
					maxNum = num
					enrichData.LatestChapterSlug = ch.Slug
					enrichData.LatestChapterName = ch.Name
				}
			}
		}

		// Add rating data
		if ratingData, exists := ratingsByMedia[mediaSlug]; exists {
			enrichData.AverageRating = ratingData[0].(float64)
			enrichData.ReviewCount = ratingData[1].(int)
		}

		result[mediaSlug] = enrichData
	}

	return result, nil
}
