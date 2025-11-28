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
)

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
	query := `
	UPDATE chapters
	SET name = ?, type = ?, file = ?, chapter_cover_url = ?
	WHERE media_slug = ? AND slug = ?
	`

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

// DeleteChaptersByMediaSlug removes all chapters for a specific manga
func DeleteChaptersByMediaSlug(mangaSlug string) error {
	return DeleteRecord(`DELETE FROM chapters WHERE media_slug = ?`, mangaSlug)
}

// ChapterExists checks if a chapter already exists
func ChapterExists(chapterSlug, mangaSlug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM chapters WHERE media_slug = ? AND slug = ?`, mangaSlug, chapterSlug)
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
	chapters, err := GetChaptersByMediaSlug(mediaSlug, maxPremiumChapters, maxPremiumChapters, premiumDuration, scalingEnabled)
	if err != nil {
		return false, "", err
	}

	for _, chapter := range chapters {
		if chapter.IsPremium {
			// Find the position of this chapter among premium chapters
			premiumPosition := 0
			for j := 0; j < len(chapters); j++ {
				if chapters[j].IsPremium {
					if chapters[j].Slug == chapter.Slug {
						break
					}
					premiumPosition++
				}
			}
			
			// Calculate scaled duration for this chapter's position
			multiplier := premiumPosition + 1
			if !scalingEnabled {
				multiplier = 1
			}
			scaledDuration := premiumDuration * multiplier
			
			// Calculate countdown for this chapter
			releaseTime := chapter.CreatedAt.Add(time.Duration(scaledDuration) * time.Second)
			duration := time.Until(releaseTime)
			var countdown string
			if duration.Hours() >= 24 {
				days := int(duration.Hours() / 24)
				countdown = fmt.Sprintf("%dd", days)
			} else if duration.Hours() >= 1 {
				hours := int(duration.Hours())
				minutes := int(duration.Minutes()) % 60
				countdown = fmt.Sprintf("%dh %dm", hours, minutes)
			} else {
				minutes := int(duration.Minutes())
				seconds := int(duration.Seconds()) % 60
				countdown = fmt.Sprintf("%dm %ds", minutes, seconds)
			}
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
		if chapters[chapterIndex].IsPremium {
			duration := time.Until(releaseTime)
			if duration.Hours() >= 24 {
				days := int(duration.Hours() / 24)
				chapters[chapterIndex].PremiumCountdown = fmt.Sprintf("%dd", days)
			} else if duration.Hours() >= 1 {
				hours := int(duration.Hours())
				minutes := int(duration.Minutes()) % 60
				chapters[chapterIndex].PremiumCountdown = fmt.Sprintf("%dh %dm", hours, minutes)
			} else {
				minutes := int(duration.Minutes())
				seconds := int(duration.Seconds()) % 60
				chapters[chapterIndex].PremiumCountdown = fmt.Sprintf("%dm %ds", minutes, seconds)
			}
		}
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
		SELECT DISTINCT m.slug, m.name, m.author, m.description, m.type, m.status, m.cover_art_url, m.created_at, m.updated_at
		FROM media m
		WHERE EXISTS (
			SELECT 1 FROM chapters c 
			WHERE c.media_slug = m.slug
		)
		ORDER BY m.updated_at DESC
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
