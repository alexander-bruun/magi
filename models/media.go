package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils/files"
	"github.com/gofiber/fiber/v2/log"
)

// AuthorInfo represents author/artist information
type AuthorInfo struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// AttributionLink represents a link to the source metadata page
type AttributionLink struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

// SortOption describes a single allowed sort field and optional alias list.
type SortOption struct {
	Key     string   // canonical key used internally
	Aliases []string // accepted alternative names
}

// MediaSortConfigType holds configuration for validating and applying sorts.
type MediaSortConfigType struct {
	Allowed      []SortOption
	DefaultKey   string
	DefaultOrder string // "asc" or "desc"
}

// NormalizeSort resolves user supplied sortBy & order into a canonical (key, order).
// Unknown keys fall back to DefaultKey. Unknown order falls back to DefaultOrder.
func (c MediaSortConfigType) NormalizeSort(sortBy, order string) (key string, ord string) {
	sb := strings.ToLower(strings.TrimSpace(sortBy))
	ob := strings.ToLower(strings.TrimSpace(order))

	// Determine default order based on sort key
	defaultOrder := c.DefaultOrder
	if sb == "popularity" || sb == "read_count" {
		defaultOrder = "desc"
	}

	if ob != "asc" && ob != "desc" {
		ob = defaultOrder
	}
	key = c.DefaultKey
	for _, opt := range c.Allowed {
		if sb == opt.Key {
			key = opt.Key
			break
		}
		for _, a := range opt.Aliases {
			if sb == strings.ToLower(a) {
				key = opt.Key
				break
			}
		}
	}
	return key, ob
}

var MediaSortConfig = MediaSortConfigType{
	Allowed: []SortOption{
		{Key: "name", Aliases: []string{"title"}},
		{Key: "type"},
		{Key: "year"},
		{Key: "status"},
		{Key: "content_rating", Aliases: []string{"contentrating"}},
		{Key: "created_at", Aliases: []string{"createdat"}},
		{Key: "updated_at", Aliases: []string{"updatedat"}},
		{Key: "read_count", Aliases: []string{"readcount"}},
		{Key: "popularity"},
	},
	DefaultKey:   "name",
	DefaultOrder: "asc",
}

// GetAllowedMediaSortOptions returns sort options
func GetAllowedMediaSortOptions() []SortOption {
	return MediaSortConfig.Allowed
}

// SortMedias applies the given normalized key & order (use MediaSortConfig.NormalizeSort)
// to the slice in-place.
func SortMedias(media []Media, key, order string) {
	asc := strings.ToLower(order) != "desc"
	switch key {
	case "name":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) < strings.ToLower(media[j].Name) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) > strings.ToLower(media[j].Name) })
		}
	case "type":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Type) < strings.ToLower(media[j].Type) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Type) > strings.ToLower(media[j].Type) })
		}
	case "year":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].Year < media[j].Year })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].Year > media[j].Year })
		}
	case "status":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Status) < strings.ToLower(media[j].Status) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Status) > strings.ToLower(media[j].Status) })
		}
	case "content_rating":
		if asc {
			sort.Slice(media, func(i, j int) bool {
				return strings.ToLower(media[i].ContentRating) < strings.ToLower(media[j].ContentRating)
			})
		} else {
			sort.Slice(media, func(i, j int) bool {
				return strings.ToLower(media[i].ContentRating) > strings.ToLower(media[j].ContentRating)
			})
		}
	case "created_at":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].CreatedAt.Before(media[j].CreatedAt) })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].CreatedAt.After(media[j].CreatedAt) })
		}
	case "updated_at":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].UpdatedAt.Before(media[j].UpdatedAt) })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].UpdatedAt.After(media[j].UpdatedAt) })
		}
	case "read_count":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].ReadCount < media[j].ReadCount })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].ReadCount > media[j].ReadCount })
		}
	case "popularity":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].VoteScore < media[j].VoteScore })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].VoteScore > media[j].VoteScore })
		}
	default:
		// default already handled by NormalizeSort -> name
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) < strings.ToLower(media[j].Name) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) > strings.ToLower(media[j].Name) })
		}
	}
}

// Media represents the media table schema
type Media struct {
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Author           string    `json:"author"`
	Description      string    `json:"description"`
	Year             int       `json:"year"`
	OriginalLanguage string    `json:"original_language"`
	Type             string    `json:"type"`
	Status           string    `json:"status"`
	ContentRating    string    `json:"content_rating"`
	CoverArtURL      string    `json:"cover_art_url"`
	FileCount        int       `json:"file_count"`
	ReadCount        int       `json:"read_count"`
	VoteScore        int       `json:"vote_score"`
	Tags             []string  `json:"tags"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PremiumCountdown string    `json:"premium_countdown,omitempty"`

	// Enhanced metadata fields
	Authors           []AuthorInfo      `json:"authors,omitempty"`
	Artists           []AuthorInfo      `json:"artists,omitempty"`
	StartDate         string            `json:"start_date,omitempty"`
	EndDate           string            `json:"end_date,omitempty"`
	ChapterCount      int               `json:"chapter_count,omitempty"`
	VolumeCount       int               `json:"volume_count,omitempty"`
	AverageScore      float64           `json:"average_score,omitempty"`
	Popularity        int               `json:"popularity,omitempty"`
	Favorites         int               `json:"favorites,omitempty"`
	Demographic       string            `json:"demographic,omitempty"`
	Publisher         string            `json:"publisher,omitempty"`
	Magazine          string            `json:"magazine,omitempty"`
	Serialization     string            `json:"serialization,omitempty"`
	Genres            []string          `json:"genres,omitempty"`
	Characters        []string          `json:"characters,omitempty"`
	AlternativeTitles []string          `json:"alternative_titles,omitempty"`
	AttributionLinks  []AttributionLink `json:"attribution_links,omitempty"`
}

// EnrichedMedia extends Media with premium countdown information
type EnrichedMedia struct {
	Media
	PremiumCountdown  string
	LatestChapterSlug string
	LatestChapterName string
	AverageRating     float64
	ReviewCount       int
	VoteScore         int
	Upvotes           int
	Downvotes         int
}

// CreateMedia adds a new media to the database
func CreateMedia(media Media) error {
	exists, err := MediaExists(media.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("media already exists")
	}

	timestamps := NewTimestamps()

	// Marshal JSON fields
	authorsJSON, _ := json.Marshal(media.Authors)
	artistsJSON, _ := json.Marshal(media.Artists)
	genresJSON, _ := json.Marshal(media.Genres)
	charactersJSON, _ := json.Marshal(media.Characters)
	alternativeTitlesJSON, _ := json.Marshal(media.AlternativeTitles)
	attributionLinksJSON, _ := json.Marshal(media.AttributionLinks)

	query := `
	INSERT INTO media (slug, name, author, description, year, original_language, type, status, content_rating, cover_art_url, file_count, created_at, updated_at,
		start_date, end_date, chapter_count, volume_count, average_score, popularity, favorites, demographic, publisher, magazine, serialization,
		authors, artists, genres, characters, alternative_titles, attribution_links)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	createdAt, updatedAt := timestamps.UnixTimestamps()
	_, err = db.Exec(query,
		media.Slug, media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.CoverArtURL, media.FileCount, createdAt, updatedAt,
		media.StartDate, media.EndDate, media.ChapterCount, media.VolumeCount, media.AverageScore, media.Popularity, media.Favorites, media.Demographic, media.Publisher, media.Magazine, media.Serialization,
		string(authorsJSON), string(artistsJSON), string(genresJSON), string(charactersJSON), string(alternativeTitlesJSON), string(attributionLinksJSON))
	return err
}

// GetMedia retrieves a single media by slug from enabled libraries
func GetMedia(slug string) (*Media, error) {
	cfg, err := GetAppConfig()
	contentRatingLimit := 3 // default to show all
	if err == nil {
		contentRatingLimit = cfg.ContentRatingLimit
	}
	return getMedia(slug, true, contentRatingLimit)
}

// GetMediaUnfiltered retrieves a single media by slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetMediaUnfiltered(slug string) (*Media, error) {
	return getMedia(slug, false, 0)
}

// GetMediaWithContentLimit retrieves a single media by slug with specified content rating limit
func GetMediaWithContentLimit(slug string, contentRatingLimit int) (*Media, error) {
	return getMedia(slug, true, contentRatingLimit)
}

// GetMediaBySlugAndLibrary retrieves a single media by slug and library slug without content rating filtering.
// This should only be used for internal operations like indexing, updates, etc.
func GetMediaBySlugAndLibrary(slug, librarySlug string) (*Media, error) {
	return getMediaBySlugAndLibrary(slug, librarySlug, false, 0)
}

// GetMediaBySlugAndLibraryUnfiltered retrieves a single media by slug and library slug without content filtering.
func GetMediaBySlugAndLibraryUnfiltered(slug, librarySlug string) (*Media, error) {
	return getMediaBySlugAndLibrary(slug, librarySlug, false, 0)
}

// GetMediaBySlugAndLibraryFiltered retrieves a single media by slug and library slug with content rating filtering.
func GetMediaBySlugAndLibraryFiltered(slug, librarySlug string) (*Media, error) {
	cfg, err := GetAppConfig()
	contentRatingLimit := 3 // default to show all
	if err == nil {
		contentRatingLimit = cfg.ContentRatingLimit
	}
	return getMediaBySlugAndLibrary(slug, librarySlug, true, contentRatingLimit)
}

// getMedia is the internal implementation that optionally applies content rating filtering
func getMedia(slug string, applyContentFilter bool, contentRatingLimit int) (*Media, error) {
	query := `SELECT slug, name, author, description, year, original_language, type, status, content_rating, cover_art_url, file_count, created_at, updated_at,
		COALESCE(start_date, '') as start_date, COALESCE(end_date, '') as end_date, COALESCE(chapter_count, 0) as chapter_count, COALESCE(volume_count, 0) as volume_count,
		COALESCE(average_score, 0.0) as average_score, COALESCE(popularity, 0) as popularity, COALESCE(favorites, 0) as favorites,
		COALESCE(demographic, '') as demographic, COALESCE(publisher, '') as publisher, COALESCE(magazine, '') as magazine, COALESCE(serialization, '') as serialization,
		COALESCE(authors, '[]') as authors, COALESCE(artists, '[]') as artists, COALESCE(genres, '[]') as genres, COALESCE(characters, '[]') as characters,
		COALESCE(alternative_titles, '[]') as alternative_titles, COALESCE(attribution_links, '[]') as attribution_links FROM media WHERE slug = ?`

	row := db.QueryRow(query, slug)

	var m Media
	var createdAt, updatedAt int64
	var authorsJSON, artistsJSON, genresJSON, charactersJSON, alternativeTitlesJSON, attributionLinksJSON []byte

	err := row.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &createdAt, &updatedAt,
		&m.StartDate, &m.EndDate, &m.ChapterCount, &m.VolumeCount, &m.AverageScore, &m.Popularity, &m.Favorites, &m.Demographic, &m.Publisher, &m.Magazine, &m.Serialization,
		&authorsJSON, &artistsJSON, &genresJSON, &charactersJSON, &alternativeTitlesJSON, &attributionLinksJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("getMedia: no rows for slug=%s", slug)
			return nil, nil // No media found
		}
		return nil, err
	}

	log.Debugf("getMedia: found media slug=%s, content_rating=%s", m.Slug, m.ContentRating)

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)

	// Unmarshal JSON fields
	json.Unmarshal(authorsJSON, &m.Authors)
	json.Unmarshal(artistsJSON, &m.Artists)
	json.Unmarshal(genresJSON, &m.Genres)
	json.Unmarshal(charactersJSON, &m.Characters)
	json.Unmarshal(alternativeTitlesJSON, &m.AlternativeTitles)
	json.Unmarshal(attributionLinksJSON, &m.AttributionLinks)

	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		allowed := IsContentRatingAllowed(m.ContentRating, contentRatingLimit)
		log.Debugf("getMedia: content filter apply, rating=%s, limit=%d, allowed=%v", m.ContentRating, contentRatingLimit, allowed)
		if !allowed {
			return nil, nil // Return nil to indicate media not found/accessible
		}
	}

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)

	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		allowed := IsContentRatingAllowed(m.ContentRating, contentRatingLimit)
		log.Debugf("getMedia: content filter apply, rating=%s, limit=%d, allowed=%v", m.ContentRating, contentRatingLimit, allowed)
		if !allowed {
			return nil, nil // Return nil to indicate media not found/accessible
		}
	}

	// Load tags for this media if any
	if tags, err := GetTagsForMedia(m.Slug); err == nil {
		m.Tags = tags
		log.Debugf("Loaded %d tags for media '%s': %v", len(tags), m.Slug, tags)
	} else {
		log.Errorf("Failed to load tags for media '%s': %v", m.Slug, err)
	}
	return &m, nil
}

// getMediaBySlugAndLibrary is the internal implementation for getting media by slug and library
func getMediaBySlugAndLibrary(slug, _ string, applyContentFilter bool, contentRatingLimit int) (*Media, error) {
	query := `SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, m.created_at, m.updated_at,
		COALESCE(m.start_date, '') as start_date, COALESCE(m.end_date, '') as end_date, COALESCE(m.chapter_count, 0) as chapter_count, COALESCE(m.volume_count, 0) as volume_count,
		COALESCE(m.average_score, 0.0) as average_score, COALESCE(m.popularity, 0) as popularity, COALESCE(m.favorites, 0) as favorites,
		COALESCE(m.demographic, '') as demographic, COALESCE(m.publisher, '') as publisher, COALESCE(m.magazine, '') as magazine, COALESCE(m.serialization, '') as serialization,
		COALESCE(m.authors, '[]') as authors, COALESCE(m.artists, '[]') as artists, COALESCE(m.genres, '[]') as genres, COALESCE(m.characters, '[]') as characters,
		COALESCE(m.alternative_titles, '[]') as alternative_titles, COALESCE(m.attribution_links, '[]') as attribution_links
	          FROM media m 
	          WHERE m.slug = ?`

	row := db.QueryRow(query, slug)

	var m Media
	var createdAt, updatedAt int64
	var authorsJSON, artistsJSON, genresJSON, charactersJSON, alternativeTitlesJSON, attributionLinksJSON []byte

	err := row.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &createdAt, &updatedAt,
		&m.StartDate, &m.EndDate, &m.ChapterCount, &m.VolumeCount, &m.AverageScore, &m.Popularity, &m.Favorites, &m.Demographic, &m.Publisher, &m.Magazine, &m.Serialization,
		&authorsJSON, &artistsJSON, &genresJSON, &charactersJSON, &alternativeTitlesJSON, &attributionLinksJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No media found
		}
		return nil, err
	}

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)

	// Unmarshal JSON fields
	json.Unmarshal(authorsJSON, &m.Authors)
	json.Unmarshal(artistsJSON, &m.Artists)
	json.Unmarshal(genresJSON, &m.Genres)
	json.Unmarshal(charactersJSON, &m.Characters)
	json.Unmarshal(alternativeTitlesJSON, &m.AlternativeTitles)
	json.Unmarshal(attributionLinksJSON, &m.AttributionLinks)

	// Apply content rating filter only if requested (for user-facing operations)
	if applyContentFilter {
		if !IsContentRatingAllowed(m.ContentRating, contentRatingLimit) {
			return nil, nil // Return nil to indicate media not found/accessible
		}
	}

	// Load tags for this media if any
	if tags, err := GetTagsForMedia(m.Slug); err == nil {
		m.Tags = tags
	}
	return &m, nil
}

// Setter methods to implement metadata.MediaUpdater interface
func (m *Media) SetName(name string)             { m.Name = name }
func (m *Media) SetDescription(desc string)      { m.Description = desc }
func (m *Media) SetYear(year int)                { m.Year = year }
func (m *Media) SetOriginalLanguage(lang string) { m.OriginalLanguage = lang }
func (m *Media) SetStatus(status string)         { m.Status = status }
func (m *Media) SetContentRating(rating string)  { m.ContentRating = rating }
func (m *Media) SetType(mangaType string)        { m.Type = mangaType }
func (m *Media) SetCoverArtURL(url string)       { m.CoverArtURL = url }

// Enhanced metadata setters
func (m *Media) SetAuthors(authors []AuthorInfo)             { m.Authors = authors }
func (m *Media) SetArtists(artists []AuthorInfo)             { m.Artists = artists }
func (m *Media) SetStartDate(date string)                    { m.StartDate = date }
func (m *Media) SetEndDate(date string)                      { m.EndDate = date }
func (m *Media) SetChapterCount(count int)                   { m.ChapterCount = count }
func (m *Media) SetVolumeCount(count int)                    { m.VolumeCount = count }
func (m *Media) SetAverageScore(score float64)               { m.AverageScore = score }
func (m *Media) SetPopularity(pop int)                       { m.Popularity = pop }
func (m *Media) SetFavorites(fav int)                        { m.Favorites = fav }
func (m *Media) SetDemographic(demo string)                  { m.Demographic = demo }
func (m *Media) SetPublisher(pub string)                     { m.Publisher = pub }
func (m *Media) SetMagazine(mag string)                      { m.Magazine = mag }
func (m *Media) SetSerialization(serial string)              { m.Serialization = serial }
func (m *Media) SetGenres(genres []string)                   { m.Genres = genres }
func (m *Media) SetCharacters(chars []string)                { m.Characters = chars }
func (m *Media) SetAlternativeTitles(titles []string)        { m.AlternativeTitles = titles }
func (m *Media) SetAttributionLinks(links []AttributionLink) { m.AttributionLinks = links }

// UpdateMedia modifies an existing media and always updates the updated_at timestamp to the current time
func UpdateMedia(media *Media) error {
	media.UpdatedAt = time.Now()

	query := `
	UPDATE media
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, type = ?, status = ?, content_rating = ?, cover_art_url = ?, file_count = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query, media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.CoverArtURL, media.FileCount, media.UpdatedAt.Unix(), media.Slug)
	if err != nil {
		return err
	}

	return nil
}

// UpdateMediaMetadata updates media metadata fields while preserving the original created_at timestamp.
// This is useful for refresh operations where we want to preserve the original creation date.
// It does not update the updated_at timestamp.
func UpdateMediaMetadata(media *Media) error {
	// Marshal JSON fields
	authorsJSON, _ := json.Marshal(media.Authors)
	artistsJSON, _ := json.Marshal(media.Artists)
	genresJSON, _ := json.Marshal(media.Genres)
	charactersJSON, _ := json.Marshal(media.Characters)
	alternativeTitlesJSON, _ := json.Marshal(media.AlternativeTitles)
	attributionLinksJSON, _ := json.Marshal(media.AttributionLinks)

	query := `
	UPDATE media
	SET name = ?, author = ?, description = ?, year = ?, original_language = ?, type = ?, status = ?, content_rating = ?, cover_art_url = ?,
		start_date = ?, end_date = ?, chapter_count = ?, volume_count = ?, average_score = ?, popularity = ?, favorites = ?,
		demographic = ?, publisher = ?, magazine = ?, serialization = ?, authors = ?, artists = ?, genres = ?,
		characters = ?, alternative_titles = ?, attribution_links = ?
	WHERE slug = ?
	`

	_, err := db.Exec(query,
		media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.CoverArtURL,
		media.StartDate, media.EndDate, media.ChapterCount, media.VolumeCount, media.AverageScore, media.Popularity, media.Favorites,
		media.Demographic, media.Publisher, media.Magazine, media.Serialization, string(authorsJSON), string(artistsJSON), string(genresJSON),
		string(charactersJSON), string(alternativeTitlesJSON), string(attributionLinksJSON), media.Slug)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMedia removes a media and its associated chapters
func DeleteMedia(slug string) error {
	// Delete poster images
	deletePosterImages(slug)

	// Delete associated chapters first
	if err := DeleteChaptersByMediaSlug(slug); err != nil {
		return err
	}

	// Delete associated tags
	if err := DeleteTagsByMediaSlug(slug); err != nil {
		return err
	}

	return DeleteRecord(`DELETE FROM media WHERE slug = ?`, slug)
}

// deletePosterImages deletes the poster image files for a media
func deletePosterImages(slug string) {
	dataDir := files.GetDataDirectory()
	postersDir := filepath.Join(dataDir, "posters")

	// Delete main poster image
	mainPath := filepath.Join(postersDir, fmt.Sprintf("%s.webp", slug))
	if err := os.Remove(mainPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster image %s: %v", mainPath, err)
	}

	// Delete original image
	originalPath := filepath.Join(postersDir, fmt.Sprintf("%s_original.webp", slug))
	if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster original image %s: %v", originalPath, err)
	}

	// Delete thumbnail
	thumbPath := filepath.Join(postersDir, fmt.Sprintf("%s_thumb.webp", slug))
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster thumbnail %s: %v", thumbPath, err)
	}

	// Delete small image
	smallPath := filepath.Join(postersDir, fmt.Sprintf("%s_small.webp", slug))
	if err := os.Remove(smallPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete poster small image %s: %v", smallPath, err)
	}
}

// SearchMedias filters, sorts, and paginates media based on provided criteria
func SearchMedias(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
	})
}

// SearchMediasWithTags extends SearchMedias to filter by selected tags (all must match)
func SearchMediasWithTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
		Tags:        selectedTags,
		TagMode:     "all",
	})
}

// SearchMediasWithAnyTags filters media to those that have at least one of the selected tags
func SearchMediasWithAnyTags(filter string, page, pageSize int, sortBy, sortOrder, filterBy, librarySlug string, selectedTags []string) ([]Media, int64, error) {
	return SearchMediasWithOptions(SearchOptions{
		Filter:      filter,
		Page:        page,
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		FilterBy:    filterBy,
		LibrarySlug: librarySlug,
		Tags:        selectedTags,
		TagMode:     "any",
	})
}

// MediaExists checks if a media exists by slug
func MediaExists(slug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM media WHERE slug = ?`, slug)
}

// MediaCount counts the number of media based on filter criteria
func MediaCount(filterBy, filter string) (int, error) {
	var mediaList []Media
	if err := loadAllMedias(&mediaList); err != nil {
		return 0, err
	}

	count := 0
	for _, media := range mediaList {
		if filterBy != "" && filter != "" {
			var value string
			switch filterBy {
			case "Slug":
				value = media.Slug
			case "Name":
				value = media.Name
			case "Author":
				value = media.Author
			case "Description":
				value = media.Description
			case "Year":
				value = fmt.Sprintf("%d", media.Year)
			case "OriginalLanguage":
				value = media.OriginalLanguage
			case "Type":
				value = media.Type
			case "Status":
				value = media.Status
			case "ContentRating":
				value = media.ContentRating
			case "CoverArtURL":
				value = media.CoverArtURL
			case "FileCount":
				value = fmt.Sprintf("%d", media.FileCount)
			case "Tags":
				value = strings.Join(media.Tags, " ")
			case "CreatedAt":
				value = media.CreatedAt.String()
			case "UpdatedAt":
				value = media.UpdatedAt.String()
			default:
				// If unknown field, skip
				continue
			}
			if strings.Contains(strings.ToLower(value), strings.ToLower(filter)) {
				count++
			}
		} else {
			count++
		}
	}
	return count, nil
}

// DeleteMediasByLibrarySlug removes all media associated with a specific library
func DeleteMediasByLibrarySlug(librarySlug string) error {
	// Get all media slugs that have chapters in this library
	query := `SELECT DISTINCT media_slug FROM chapters WHERE library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query media by librarySlug: %v", err)
		return err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			log.Errorf("Failed to scan row: %v", err)
			return err
		}
		slugs = append(slugs, slug)
	}

	// Delete chapters for this library
	deleteChaptersQuery := `DELETE FROM chapters WHERE library_slug = ?`
	_, err = db.Exec(deleteChaptersQuery, librarySlug)
	if err != nil {
		log.Errorf("Failed to delete chapters for library '%s': %v", librarySlug, err)
		return err
	}

	// For each affected media, check if it has any remaining chapters, if not, delete the media
	for _, mediaSlug := range slugs {
		var count int
		err = db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE media_slug = ?`, mediaSlug).Scan(&count)
		if err != nil {
			log.Errorf("Failed to check remaining chapters for media '%s': %v", mediaSlug, err)
			continue
		}
		if count == 0 {
			// No chapters left, delete the media
			if err := DeleteMedia(mediaSlug); err != nil {
				log.Errorf("Failed to delete orphaned media '%s': %v", mediaSlug, err)
			}
		}
	}

	return nil
}

// GetMediasBySlugs loads multiple media by slugs with their tags in batch to avoid N+1 queries
func GetMediasBySlugs(slugs []string) ([]Media, error) {
	if len(slugs) == 0 {
		return []Media{}, nil
	}

	// Build query with IN clause
	placeholders := strings.Repeat("?,", len(slugs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, cover_art_url, file_count, created_at, updated_at,
		start_date, end_date, chapter_count, volume_count, average_score, popularity, favorites, demographic, publisher, magazine, serialization,
		authors, artists, genres, characters, alternative_titles, attribution_links FROM media WHERE slug IN (%s)`, placeholders)

	args := make([]any, len(slugs))
	for i, slug := range slugs {
		args[i] = slug
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config for content rating check: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	var medias []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var authorsJSON, artistsJSON, genresJSON, charactersJSON, alternativeTitlesJSON, attributionLinksJSON []byte

		err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &createdAt, &updatedAt,
			&m.StartDate, &m.EndDate, &m.ChapterCount, &m.VolumeCount, &m.AverageScore, &m.Popularity, &m.Favorites, &m.Demographic, &m.Publisher, &m.Magazine, &m.Serialization,
			&authorsJSON, &artistsJSON, &genresJSON, &charactersJSON, &alternativeTitlesJSON, &attributionLinksJSON)
		if err != nil {
			return nil, err
		}

		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)

		// Unmarshal JSON fields
		json.Unmarshal(authorsJSON, &m.Authors)
		json.Unmarshal(artistsJSON, &m.Artists)
		json.Unmarshal(genresJSON, &m.Genres)
		json.Unmarshal(charactersJSON, &m.Characters)
		json.Unmarshal(alternativeTitlesJSON, &m.AlternativeTitles)
		json.Unmarshal(attributionLinksJSON, &m.AttributionLinks)

		// Apply content rating filter
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			medias = append(medias, m)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch load tags for all media
	if len(medias) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for media batch: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range medias {
				if tags, ok := tagMap[medias[i].Slug]; ok {
					medias[i].Tags = tags
				}
			}
		}
	}

	return medias, nil
}

// GetMediasByLibrarySlug returns all media that belong to a specific library
func GetMediasByLibrarySlug(librarySlug string) ([]Media, error) {
	var mediaList []Media
	query := `SELECT DISTINCT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, m.created_at, m.updated_at,
		m.start_date, m.end_date, m.chapter_count, m.volume_count, m.average_score, m.popularity, m.favorites, m.demographic, m.publisher, m.magazine, m.serialization,
		m.authors, m.artists, m.genres, m.characters, m.alternative_titles, m.attribution_links FROM media m INNER JOIN chapters c ON m.slug = c.media_slug WHERE c.library_slug = ?`

	rows, err := db.Query(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to query media by librarySlug: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config, defaulting to show all content: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var authorsJSON, artistsJSON, genresJSON, charactersJSON, alternativeTitlesJSON, attributionLinksJSON []byte

		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &createdAt, &updatedAt,
			&m.StartDate, &m.EndDate, &m.ChapterCount, &m.VolumeCount, &m.AverageScore, &m.Popularity, &m.Favorites, &m.Demographic, &m.Publisher, &m.Magazine, &m.Serialization,
			&authorsJSON, &artistsJSON, &genresJSON, &charactersJSON, &alternativeTitlesJSON, &attributionLinksJSON); err != nil {
			log.Errorf("Failed to scan media row: %v", err)
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)

		// Unmarshal JSON fields
		json.Unmarshal(authorsJSON, &m.Authors)
		json.Unmarshal(artistsJSON, &m.Artists)
		json.Unmarshal(genresJSON, &m.Genres)
		json.Unmarshal(charactersJSON, &m.Characters)
		json.Unmarshal(alternativeTitlesJSON, &m.AlternativeTitles)
		json.Unmarshal(attributionLinksJSON, &m.AttributionLinks)

		// Filter based on content rating limit
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			mediaList = append(mediaList, m)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mediaList, nil
}

// GetTopMedias returns the top media ordered by vote score (descending).
// It joins the media table with aggregated votes and respects content rating limits.
func GetTopMedias(limit int, accessibleLibraries []string) ([]Media, error) {
	return GetTopMediasByPeriod("all", limit, accessibleLibraries)
}

func GetTopMediasByPeriod(period string, limit int, accessibleLibraries []string) ([]Media, error) {
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
		dateFilter = "AND votes.created_at >= strftime('%s', datetime('now', 'start of day'))"
	case "week":
		dateFilter = "AND votes.created_at >= strftime('%s', datetime('now', '-7 days', 'start of day'))"
	case "month":
		dateFilter = "AND votes.created_at >= strftime('%s', datetime('now', '-1 month', 'start of day'))"
	case "year":
		dateFilter = "AND votes.created_at >= strftime('%s', datetime('now', '-1 year', 'start of day'))"
	case "all":
		dateFilter = ""
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	var libraryFilter string
	var args []any

	// Add content rating args first
	for _, rating := range allowedRatings {
		args = append(args, rating)
	}

	if len(accessibleLibraries) > 0 {
		libraryPlaceholders := strings.Repeat("?,", len(accessibleLibraries))
		libraryPlaceholders = libraryPlaceholders[:len(libraryPlaceholders)-1] // remove trailing comma
		libraryFilter = fmt.Sprintf("AND EXISTS (SELECT 1 FROM chapters c WHERE c.media_slug = m.slug AND c.library_slug IN (%s))", libraryPlaceholders)
		for _, lib := range accessibleLibraries {
			args = append(args, lib)
		}
	}
	// Note: If no accessible libraries specified, we don't filter by library, allowing all media to be included

	query := fmt.Sprintf(`
	SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, m.created_at, m.updated_at
	FROM media m
	LEFT JOIN (
		SELECT media_slug, 
			CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
			THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
			ELSE 0 END as score
		FROM votes
		WHERE 1=1 %s
		GROUP BY media_slug
	) v ON v.media_slug = m.slug
	WHERE m.content_rating IN (%s) %s
	ORDER BY v.score DESC
	LIMIT ?
	`, dateFilter, placeholders, libraryFilter)

	// Then limit
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)

		mediaList = append(mediaList, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch load tags for all media
	if len(mediaList) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for top media: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range mediaList {
				if tags, ok := tagMap[mediaList[i].Slug]; ok {
					mediaList[i].Tags = tags
				}
			}
		}
	}

	return mediaList, nil
}

// Helper functions

func loadAllMedias(media *[]Media) error {
	return loadAllMediasWithTags(media, false)
}

// loadAllMediasWithTags loads all media with optional tag loading to avoid N+1 queries when tags are needed
func loadAllMediasWithTags(media *[]Media, loadTags bool) error {
	query := `SELECT DISTINCT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, 
		COALESCE(read_counts.read_count, 0) as read_count,
		COALESCE(vote_scores.score, 0) as vote_score,
		m.created_at, m.updated_at 
	FROM media m
	INNER JOIN chapters c ON m.slug = c.media_slug
	INNER JOIN libraries l ON c.library_slug = l.slug
	LEFT JOIN (
		SELECT media_slug, COUNT(*) as read_count
		FROM reading_states
		GROUP BY media_slug
	) read_counts ON m.slug = read_counts.media_slug
	LEFT JOIN (
		SELECT media_slug, 
			CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
			THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
			ELSE 0 END as score
		FROM votes
		GROUP BY media_slug
	) vote_scores ON m.slug = vote_scores.media_slug`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to get all media: %v", err)
		return err
	}
	defer rows.Close()

	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config, defaulting to show all content: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var voteScore int
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt); err != nil {
			return err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore

		// Filter based on content rating limit
		if IsContentRatingAllowed(m.ContentRating, cfg.ContentRatingLimit) {
			*media = append(*media, m)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Batch load tags for all media if requested
	if loadTags && len(*media) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for media batch: %v", err)
			// Continue without tags rather than failing
		} else {
			for i := range *media {
				if tags, ok := tagMap[(*media)[i].Slug]; ok {
					(*media)[i].Tags = tags
				}
			}
		}
	}

	return nil
}

// GetAllMediaTypes returns all distinct type values (lowercased) sorted ascending
func GetAllMediaTypes() ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT LOWER(TRIM(type)) FROM media WHERE type IS NOT NULL AND TRIM(type) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t != "" {
			types = append(types, t)
		}
	}
	sort.Strings(types)
	return types, nil
}

// sortMedias moved to sorting.go (exported as SortMedias) for reuse across account pages.

// Vote represents a user's vote on a media
type Vote struct {
	ID           int64
	UserUsername string
	MediaSlug    string
	Value        int // 1 for upvote, -1 for downvote
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetMediaVotes returns the aggregated score and counts for a media
func GetMediaVotes(mangaSlug string) (score int, upvotes int, downvotes int, err error) {
	// Use COALESCE so aggregates return 0 instead of NULL when there are no rows
	query := `SELECT 
		CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
		THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
		ELSE 0 END as score,
		COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) as upvotes, 
		COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) as downvotes 
		FROM votes WHERE media_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	if err := row.Scan(&score, &upvotes, &downvotes); err != nil {
		return 0, 0, 0, err
	}
	return score, upvotes, downvotes, nil
}

// BatchGetMediaVotes gets vote data for multiple media slugs in one query
func BatchGetMediaVotes(mediaSlugs []string) (map[string][3]int, error) {
	if len(mediaSlugs) == 0 {
		return make(map[string][3]int), nil
	}

	placeholders := strings.Repeat("?,", len(mediaSlugs)-1) + "?"
	query := fmt.Sprintf(`SELECT media_slug,
		CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
		THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
		ELSE 0 END as score,
		COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) as upvotes, 
		COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) as downvotes 
		FROM votes WHERE media_slug IN (%s) GROUP BY media_slug`, placeholders)

	args := make([]any, len(mediaSlugs))
	for i, slug := range mediaSlugs {
		args[i] = slug
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][3]int)
	for rows.Next() {
		var slug string
		var score, upvotes, downvotes int
		if err := rows.Scan(&slug, &score, &upvotes, &downvotes); err != nil {
			return nil, err
		}
		result[slug] = [3]int{score, upvotes, downvotes}
	}

	return result, nil
}

// GetUserVoteForMedia returns the vote value (1, -1) for a user on a media. If none, returns 0.
func GetUserVoteForMedia(username, mangaSlug string) (int, error) {
	query := `SELECT value FROM votes WHERE user_username = ? AND media_slug = ?`
	row := db.QueryRow(query, username, mangaSlug)
	var val int
	err := row.Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// SetVote inserts or updates a user's vote for a media. value must be 1 or -1.
func SetVote(username, mangaSlug string, value int) error {
	if value != 1 && value != -1 {
		return errors.New("invalid vote value")
	}
	now := time.Now().Unix()
	// Try update first
	res, err := db.Exec(`UPDATE votes SET value = ?, updated_at = ? WHERE user_username = ? AND media_slug = ?`, value, now, username, mangaSlug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// Insert
	_, err = db.Exec(`INSERT INTO votes (user_username, media_slug, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, username, mangaSlug, value, now, now)
	return err
}

// RemoveVote deletes a user's vote for a media
func RemoveVote(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM votes WHERE user_username = ? AND media_slug = ?`, username, mangaSlug)
	return err
}

// Favorite represents a user's favorite media
type Favorite struct {
	ID           int64
	UserUsername string
	MediaSlug    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SetFavorite inserts a favorite relationship for a user and media.
func SetFavorite(username, mangaSlug string) error {
	now := time.Now().Unix()
	// Try update first (in case row exists) - this keeps updated_at current
	res, err := db.Exec(`UPDATE favorites SET updated_at = ? WHERE user_username = ? AND media_slug = ?`, now, username, mangaSlug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = db.Exec(`INSERT INTO favorites (user_username, media_slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, mangaSlug, now, now)
	return err
}

// RemoveFavorite deletes a user's favorite for a media
func RemoveFavorite(username, mangaSlug string) error {
	_, err := db.Exec(`DELETE FROM favorites WHERE user_username = ? AND media_slug = ?`, username, mangaSlug)
	return err
}

// ToggleFavorite toggles the favorite status for a user and media
func ToggleFavorite(username, mangaSlug string) error {
	isFavorite, err := IsFavoriteForUser(username, mangaSlug)
	if err != nil {
		return err
	}

	if isFavorite {
		return RemoveFavorite(username, mangaSlug)
	} else {
		return SetFavorite(username, mangaSlug)
	}
}

// IsFavoriteForUser returns true if the user has favorited the media
func IsFavoriteForUser(username, mangaSlug string) (bool, error) {
	query := `SELECT 1 FROM favorites WHERE user_username = ? AND media_slug = ?`
	row := db.QueryRow(query, username, mangaSlug)
	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFavoritesCount returns the number of users who favorited the media
func GetFavoritesCount(mangaSlug string) (int, error) {
	query := `SELECT COUNT(*) FROM favorites WHERE media_slug = ?`
	row := db.QueryRow(query, mangaSlug)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetFavoritesForUser returns media slugs favorited by the user ordered by most recent update
func GetFavoritesForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM favorites WHERE user_username = ? ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetReadingMediasForUser returns distinct media slugs that the user has reading state records for,
// ordered by most recent activity.
func GetReadingMediasForUser(username string) ([]string, error) {
	query := `SELECT DISTINCT media_slug FROM reading_states WHERE user_name = ? ORDER BY created_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetUpvotedMediasForUser returns media slugs the user has upvoted (value = 1), ordered by most recent vote
func GetUpvotedMediasForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM votes WHERE user_username = ? AND value = 1 ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// GetDownvotedMediasForUser returns media slugs the user has downvoted (value = -1), ordered by most recent vote
func GetDownvotedMediasForUser(username string) ([]string, error) {
	query := `SELECT media_slug FROM votes WHERE user_username = ? AND value = -1 ORDER BY updated_at DESC`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// UserMediaListOptions defines parameters for user-specific media list queries (favorites, reading, etc.)
type UserMediaListOptions struct {
	Username            string
	Page                int
	PageSize            int
	SortBy              string
	SortOrder           string
	Tags                []string
	TagMode             string // "all" or "any"
	SearchFilter        string
	AccessibleLibraries []string // filter by accessible libraries for permission system
	Types               []string // filter by media types (any match)
}

// GetUserFavoritesWithOptions fetches, filters, sorts, and paginates a user's favorite media
func GetUserFavoritesWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetFavoritesForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserReadingWithOptions fetches, filters, sorts, and paginates a user's reading list
func GetUserReadingWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetReadingMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserUpvotedWithOptions fetches, filters, sorts, and paginates a user's upvoted media
func GetUserUpvotedWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetUpvotedMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// GetUserDownvotedWithOptions fetches, filters, sorts, and paginates a user's downvoted media
func GetUserDownvotedWithOptions(opts UserMediaListOptions) ([]Media, int, error) {
	slugs, err := GetDownvotedMediasForUser(opts.Username)
	if err != nil {
		return nil, 0, err
	}
	return processUserMediaList(slugs, opts)
}

// processUserMediaList handles the common logic for filtering, sorting, and paginating user media lists
func processUserMediaList(slugs []string, opts UserMediaListOptions) ([]Media, int, error) {
	// Load all media from slugs in batch (with tags) to avoid N+1 queries
	allMedias, err := GetMediasBySlugs(slugs)
	if err != nil {
		return nil, 0, err
	}

	// Filter by tags if specified
	if len(opts.Tags) > 0 {
		allMedias = FilterMediasByTags(allMedias, opts.Tags, opts.TagMode)
	}

	// Filter by search term if specified
	if opts.SearchFilter != "" {
		allMedias = FilterMediasBySearch(allMedias, opts.SearchFilter)
	}

	// Filter by types if specified
	if len(opts.Types) > 0 {
		typeSet := make(map[string]struct{}, len(opts.Types))
		for _, t := range opts.Types {
			typeSet[t] = struct{}{}
		}

		filtered := make([]Media, 0, len(allMedias))
		for _, m := range allMedias {
			if _, ok := typeSet[m.Type]; ok {
				filtered = append(filtered, m)
			}
		}
		allMedias = filtered
	}

	// Sort media
	SortMedias(allMedias, opts.SortBy, opts.SortOrder)

	// Calculate total before pagination
	total := len(allMedias)

	// Paginate
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start > len(allMedias) {
		start = len(allMedias)
	}
	if end > len(allMedias) {
		end = len(allMedias)
	}

	return allMedias[start:end], total, nil
}

// FilterMediasByTags filters a slice of media by selected tags
// tagMode can be "all" (all tags must match) or "any" (at least one tag must match)
// FilterMediasByTags filters a slice of media by selected tags
// This function assumes that media.Tags are already populated to avoid N+1 queries
func FilterMediasByTags(mediaList []Media, selectedTags []string, tagMode string) []Media {
	if len(selectedTags) == 0 {
		return mediaList
	}

	var filtered []Media
	for _, media := range mediaList {
		if tagMode == "any" {
			// At least one selected tag must be in media's tags
			for _, selTag := range selectedTags {
				for _, mTag := range media.Tags {
					if strings.EqualFold(selTag, mTag) {
						filtered = append(filtered, media)
						goto nextMedia
					}
				}
			}
		} else {
			// All selected tags must be in media's tags
			matchCount := 0
			for _, selTag := range selectedTags {
				for _, mTag := range media.Tags {
					if strings.EqualFold(selTag, mTag) {
						matchCount++
						break
					}
				}
			}
			if matchCount == len(selectedTags) {
				filtered = append(filtered, media)
			}
		}
	nextMedia:
	}
	return filtered
}

// FilterMediasBySearch filters a slice of media by search term using very lenient fuzzy matching
func FilterMediasBySearch(mediaList []Media, searchTerm string) []Media {
	if searchTerm == "" {
		return mediaList
	}

	// Aggressive normalization function
	normalize := func(s string) string {
		s = strings.ToLower(s)
		// Remove all non-alphanumeric characters except spaces
		var result strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
				result.WriteRune(r)
			} else if r >= 'A' && r <= 'Z' {
				result.WriteRune(r + 32) // Convert to lowercase
			} else {
				// Replace any other character with space
				result.WriteRune(' ')
			}
		}
		return result.String()
	}

	// Normalize and split search term
	normalizedSearch := normalize(searchTerm)
	searchWords := strings.Fields(normalizedSearch)
	if len(searchWords) == 0 {
		return mediaList
	}

	var filtered []Media
	for _, media := range mediaList {
		// Normalize media name
		normalizedName := normalize(media.Name)

		// Check if all search words match
		matched := true
		for _, searchWord := range searchWords {
			if searchWord == "" {
				continue
			}

			// First check: simple substring match
			if strings.Contains(normalizedName, searchWord) {
				continue
			}

			// Second check: word prefix match
			nameWords := strings.Fields(normalizedName)
			wordMatched := false
			for _, nameWord := range nameWords {
				if strings.HasPrefix(nameWord, searchWord) {
					wordMatched = true
					break
				}
				// Also check if the search word appears within the name word (substring)
				if strings.Contains(nameWord, searchWord) {
					wordMatched = true
					break
				}
			}

			if !wordMatched {
				matched = false
				break
			}
		}

		if matched {
			filtered = append(filtered, media)
		}
	}
	return filtered
}

// GetMediaAndChapters retrieves a media and its chapters in one call
func GetMediaAndChapters(mangaSlug string) (*Media, []Chapter, error) {
	media, err := GetMedia(mangaSlug)
	if err != nil {
		return nil, nil, err
	}
	if media == nil {
		return nil, nil, nil // Media not found or access restricted
	}

	chapters, err := GetChapters(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	return media, chapters, nil
}

// GetChapterImages generates URLs for all images in a chapter
func GetChapterImages(media *Media, chapter *Chapter) ([]string, error) {
	// Get the library to determine the root path
	library, err := GetLibrary(chapter.LibrarySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get library '%s': %w", chapter.LibrarySlug, err)
	}

	// Use the first library folder as the root (since media are path-agnostic now)
	var rootPath string
	if len(library.Folders) > 0 {
		rootPath = library.Folders[0]
	} else {
		return nil, fmt.Errorf("library '%s' has no folders configured", chapter.LibrarySlug)
	}

	// Determine the actual chapter file path using the library root + relative chapter file path
	chapterFilePath := filepath.Join(rootPath, chapter.File)

	// Check if chapter file path exists
	if _, err := os.Stat(chapterFilePath); err != nil {
		return nil, fmt.Errorf("chapter file path '%s' does not exist: %w", chapterFilePath, err)
	}

	pageCount, err := files.CountImageFiles(chapterFilePath)
	if err != nil {
		return nil, err
	}

	if pageCount <= 0 {
		return []string{}, nil
	}

	images := make([]string, pageCount)
	for i := range pageCount {
		// Generate one-time use token
		validityMinutes := GetImageTokenValidityMinutes()
		token := files.GenerateImageAccessTokenWithValidity(media.Slug, chapter.LibrarySlug, chapter.Slug, i+1, validityMinutes)
		images[i] = token
	}

	return images, nil
}

// GetFirstAndLastChapterSlugs returns the first and last chapter slugs for a media
func GetFirstAndLastChapterSlugs(chapters []Chapter) (firstSlug, lastSlug string) {
	if len(chapters) > 0 {
		firstSlug = chapters[len(chapters)-1].Slug
		lastSlug = chapters[0].Slug
	}
	return firstSlug, lastSlug
}

// SearchOptions defines parameters for media searches
type SearchOptions struct {
	Filter              string
	Page                int
	PageSize            int
	SortBy              string
	SortOrder           string
	FilterBy            string
	LibrarySlug         string
	Tags                []string
	TagMode             string   // "all" or "any"
	Types               []string // filter by media types (any match)
	AccessibleLibraries []string // filter by accessible libraries for permission system
	ContentRatingLimit  int      // filter by content rating
	SearchFilter        string   // lenient search filter
}

// SearchMediasWithOptions performs a flexible media search using options with SQL-based filtering and sorting
func SearchMediasWithOptions(opts SearchOptions) ([]Media, int64, error) {
	// Get content rating limit from config
	cfg, err := GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config, defaulting to show all content: %v", err)
		cfg.ContentRatingLimit = 3 // default to show all if config fails
	}

	// Build the base query
	baseQuery := `SELECT DISTINCT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, 
		COALESCE(read_counts.read_count, 0) as read_count,
		COALESCE(vote_scores.score, 0) as vote_score,
		m.created_at, m.updated_at 
	FROM media m
	LEFT JOIN (
		SELECT media_slug, COUNT(*) as read_count
		FROM reading_states
		GROUP BY media_slug
	) read_counts ON m.slug = read_counts.media_slug
	LEFT JOIN (
		SELECT media_slug, 
			CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0 
			THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10) 
			ELSE 0 END as score
		FROM votes
		GROUP BY media_slug
	) vote_scores ON m.slug = vote_scores.media_slug
	WHERE 1=1`

	var whereConditions []string
	var args []any

	// Filter by accessible libraries (permission system)
	if len(opts.AccessibleLibraries) > 0 {
		placeholders := strings.Repeat("?,", len(opts.AccessibleLibraries))
		placeholders = placeholders[:len(placeholders)-1] // remove trailing comma
		whereConditions = append(whereConditions, fmt.Sprintf("EXISTS (SELECT 1 FROM chapters c WHERE c.media_slug = m.slug AND c.library_slug IN (%s))", placeholders))
		for _, lib := range opts.AccessibleLibraries {
			args = append(args, lib)
		}
	}
	// Note: If no accessible libraries specified, we don't filter by library, allowing media without chapters to be included

	// Filter by library
	if opts.LibrarySlug != "" {
		whereConditions = append(whereConditions, "c.library_slug = ?")
		args = append(args, opts.LibrarySlug)
	}

	// Filter by content rating
	contentLimit := cfg.ContentRatingLimit
	if opts.ContentRatingLimit > 0 {
		contentLimit = opts.ContentRatingLimit
	}
	whereConditions = append(whereConditions, `CASE LOWER(TRIM(m.content_rating))
		WHEN 'safe' THEN 0
		WHEN 'suggestive' THEN 1
		WHEN 'erotica' THEN 2
		WHEN 'pornographic' THEN 3
		ELSE 3 END <= ?`)
	args = append(args, contentLimit)

	// Filter by tags
	if len(opts.Tags) > 0 {
		normalizedTags := make([]string, len(opts.Tags))
		for i, tag := range opts.Tags {
			normalizedTags[i] = strings.TrimSpace(strings.ToLower(tag))
		}
		if opts.TagMode == "any" {
			placeholders := strings.Repeat("?,", len(normalizedTags))
			placeholders = placeholders[:len(placeholders)-1]
			whereConditions = append(whereConditions, fmt.Sprintf("EXISTS (SELECT 1 FROM media_tags mt WHERE mt.media_slug = m.slug AND LOWER(TRIM(mt.tag)) IN (%s))", placeholders))
			for _, tag := range normalizedTags {
				args = append(args, tag)
			}
		} else { // "all"
			for _, tag := range normalizedTags {
				whereConditions = append(whereConditions, "EXISTS (SELECT 1 FROM media_tags mt WHERE mt.media_slug = m.slug AND LOWER(TRIM(mt.tag)) = ?)")
				args = append(args, tag)
			}
		}
	}

	// Filter by types
	if len(opts.Types) > 0 {
		normalizedTypes := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			normalizedTypes[i] = strings.TrimSpace(strings.ToLower(t))
		}
		placeholders := strings.Repeat("?,", len(normalizedTypes))
		placeholders = placeholders[:len(placeholders)-1]
		whereConditions = append(whereConditions, fmt.Sprintf("LOWER(TRIM(m.type)) IN (%s)", placeholders))
		for _, t := range normalizedTypes {
			args = append(args, t)
		}
	}

	// Apply text search filter
	if opts.SearchFilter != "" {
		// Split search term into words and create LIKE conditions
		searchWords := strings.Fields(strings.ToLower(strings.TrimSpace(opts.SearchFilter)))
		if len(searchWords) > 0 {
			var likeConditions []string
			for _, word := range searchWords {
				if word != "" {
					likeConditions = append(likeConditions, "LOWER(m.name) LIKE ?")
					args = append(args, "%"+word+"%")
				}
			}
			if len(likeConditions) > 0 {
				whereConditions = append(whereConditions, "("+strings.Join(likeConditions, " AND ")+")")
			}
		}
	} else if opts.Filter != "" {
		// Similar for Filter
		filterWords := strings.Fields(strings.ToLower(strings.TrimSpace(opts.Filter)))
		if len(filterWords) > 0 {
			var likeConditions []string
			for _, word := range filterWords {
				if word != "" {
					likeConditions = append(likeConditions, "LOWER(m.name) LIKE ?")
					args = append(args, "%"+word+"%")
				}
			}
			if len(likeConditions) > 0 {
				whereConditions = append(whereConditions, "("+strings.Join(likeConditions, " AND ")+")")
			}
		}
	}

	// Combine WHERE conditions
	query := baseQuery
	if len(whereConditions) > 0 {
		query += " AND " + strings.Join(whereConditions, " AND ")
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ") as subquery"
	var total int64
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %v", err)
	}

	// Add sorting
	key, ord := MediaSortConfig.NormalizeSort(opts.SortBy, opts.SortOrder)
	var orderBy string
	switch key {
	case "name":
		orderBy = "LOWER(m.name)"
	case "type":
		orderBy = "LOWER(m.type)"
	case "year":
		orderBy = "m.year"
	case "status":
		orderBy = "LOWER(m.status)"
	case "content_rating":
		orderBy = "LOWER(m.content_rating)"
	case "created_at":
		orderBy = "m.created_at"
	case "updated_at":
		orderBy = "m.updated_at"
	case "read_count":
		orderBy = "COALESCE(read_counts.read_count, 0)"
	case "popularity":
		orderBy = "COALESCE(vote_scores.score, 0)"
	default:
		orderBy = "LOWER(m.name)"
	}
	if ord == "desc" {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}
	query += " ORDER BY " + orderBy

	// Add pagination
	if opts.PageSize > 0 {
		offset := max((opts.Page-1)*opts.PageSize, 0)
		query += " LIMIT ? OFFSET ?"
		args = append(args, opts.PageSize, offset)
	}

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search query: %v", err)
	}
	defer rows.Close()

	var medias []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var voteScore int
		if err := rows.Scan(&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage, &m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL, &m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan media: %v", err)
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore
		medias = append(medias, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %v", err)
	}

	// Load tags if needed for display (only if tags were filtered)
	if len(opts.Tags) > 0 && len(medias) > 0 {
		tagMap, err := GetAllMediaTagsMap()
		if err != nil {
			log.Errorf("Failed to load tags for media: %v", err)
		} else {
			for i := range medias {
				if tags, ok := tagMap[medias[i].Slug]; ok {
					medias[i].Tags = tags
				}
			}
		}
	}

	return medias, total, nil
}

func filterByAccessibleLibraries(mangas []Media, accessibleLibraries []string) []Media {
	if len(accessibleLibraries) == 0 {
		return []Media{} // No accessible libraries means no media
	}

	if len(mangas) == 0 {
		return mangas
	}

	// Collect media slugs
	mediaSlugs := make([]string, len(mangas))
	for i, m := range mangas {
		mediaSlugs[i] = m.Slug
	}

	// Create placeholders for IN clause
	placeholders := strings.Repeat("?,", len(mediaSlugs))
	placeholders = placeholders[:len(placeholders)-1]
	libPlaceholders := strings.Repeat("?,", len(accessibleLibraries))
	libPlaceholders = libPlaceholders[:len(libPlaceholders)-1]

	query := fmt.Sprintf("SELECT DISTINCT media_slug FROM chapters WHERE media_slug IN (%s) AND library_slug IN (%s)", placeholders, libPlaceholders)

	args := make([]any, 0, len(mediaSlugs)+len(accessibleLibraries))
	for _, slug := range mediaSlugs {
		args = append(args, slug)
	}
	for _, lib := range accessibleLibraries {
		args = append(args, lib)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Errorf("Failed to query accessible media: %v", err)
		return []Media{} // On error, return empty to be safe
	}
	defer rows.Close()

	accessibleMediaSet := make(map[string]struct{})
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			log.Errorf("Failed to scan media slug: %v", err)
			continue
		}
		accessibleMediaSet[slug] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		log.Errorf("Error iterating rows: %v", err)
		return []Media{}
	}

	// Filter mangas
	filtered := make([]Media, 0, len(mangas))
	for _, m := range mangas {
		if _, ok := accessibleMediaSet[m.Slug]; ok {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// DetermineMangaTypeByLanguage returns a suggested type (manga/manhwa/manhua/etc.)
// based on the original language code.
func DetermineMangaTypeByLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ja", "jp":
		return "manga"
	case "ko":
		return "manhwa"
	case "zh", "cn", "zh-cn", "zh-hk", "zh-tw":
		return "manhua"
	case "fr":
		return "manfra"
	case "en":
		return "oel"
	default:
		return "manga"
	}
}
