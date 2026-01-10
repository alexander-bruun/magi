package models

import (
	"time"
)

// Highlight represents a highlighted series for the home page banner
type Highlight struct {
	ID                 int       `json:"id"`
	MediaSlug          string    `json:"media_slug"`
	BackgroundImageURL string    `json:"background_image_url"`
	Description        string    `json:"description"`
	DisplayOrder       int       `json:"display_order"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// HighlightWithMedia combines highlight data with media information
type HighlightWithMedia struct {
	Highlight Highlight `json:"highlight"`
	Media     Media     `json:"media"`
}

// GetHighlights retrieves all highlights ordered by display_order
func GetHighlights() ([]HighlightWithMedia, error) {
	query := `
		SELECT h.id, h.media_slug, h.background_image_url, h.description, h.display_order, h.created_at, h.updated_at,
		       m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count, m.created_at, m.updated_at
		FROM highlights h
		JOIN media m ON h.media_slug = m.slug
		ORDER BY h.display_order ASC, h.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var highlights []HighlightWithMedia
	for rows.Next() {
		var h HighlightWithMedia
		var mediaCreatedAt, mediaUpdatedAt int64
		err := rows.Scan(
			&h.Highlight.ID, &h.Highlight.MediaSlug, &h.Highlight.BackgroundImageURL, &h.Highlight.Description,
			&h.Highlight.DisplayOrder, &h.Highlight.CreatedAt, &h.Highlight.UpdatedAt,
			&h.Media.Slug, &h.Media.Name, &h.Media.Author, &h.Media.Description, &h.Media.Year,
			&h.Media.OriginalLanguage, &h.Media.Type, &h.Media.Status, &h.Media.ContentRating,
			&h.Media.CoverArtURL, &h.Media.FileCount,
			&mediaCreatedAt, &mediaUpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		h.Media.CreatedAt = time.Unix(mediaCreatedAt, 0)
		h.Media.UpdatedAt = time.Unix(mediaUpdatedAt, 0)
		highlights = append(highlights, h)
	}

	return highlights, rows.Err()
}

// CreateHighlight creates a new highlight
func CreateHighlight(mediaSlug, backgroundImageURL, description string, displayOrder int) (*Highlight, error) {
	query := `
		INSERT INTO highlights (media_slug, background_image_url, description, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := db.Exec(query, mediaSlug, backgroundImageURL, description, displayOrder)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetHighlightByID(int(id))
}

// GetHighlightByID retrieves a highlight by ID
func GetHighlightByID(id int) (*Highlight, error) {
	query := `
		SELECT id, media_slug, background_image_url, description, display_order, created_at, updated_at
		FROM highlights WHERE id = ?
	`

	var h Highlight
	err := db.QueryRow(query, id).Scan(
		&h.ID, &h.MediaSlug, &h.BackgroundImageURL, &h.Description, &h.DisplayOrder, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &h, nil
}

// UpdateHighlight updates an existing highlight
func UpdateHighlight(id int, mediaSlug, backgroundImageURL, description string, displayOrder int) error {
	query := `
		UPDATE highlights
		SET media_slug = ?, background_image_url = ?, description = ?, display_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.Exec(query, mediaSlug, backgroundImageURL, description, displayOrder, id)
	return err
}

// DeleteHighlight deletes a highlight by ID
func DeleteHighlight(id int) error {
	query := `DELETE FROM highlights WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

// DeleteHighlightByMediaSlug deletes a highlight by media slug
func DeleteHighlightByMediaSlug(mediaSlug string) error {
	query := `DELETE FROM highlights WHERE media_slug = ?`
	_, err := db.Exec(query, mediaSlug)
	return err
}

// IsMediaHighlighted checks if a media is already highlighted
func IsMediaHighlighted(mediaSlug string) (bool, error) {
	query := `SELECT COUNT(*) FROM highlights WHERE media_slug = ?`
	var count int
	err := db.QueryRow(query, mediaSlug).Scan(&count)
	return count > 0, err
}
