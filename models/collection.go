package models

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// Collection represents a user-created collection of media
type Collection struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MediaCount  int       `json:"media_count,omitempty"`
	TopMedia    []Media   `json:"top_media,omitempty"`
}

// CollectionWithMedia represents a collection with its associated media
type CollectionWithMedia struct {
	Collection
	Media []Media `json:"media"`
}

// CreateCollection creates a new collection
func CreateCollection(name, description, createdBy string) (*Collection, error) {
	now := time.Now().Unix()

	result, err := db.Exec(`
		INSERT INTO collections (name, description, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		name, description, createdBy, now, now)
	if err != nil {
		log.Error("Failed to create collection:", err)
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Error("Failed to get collection ID:", err)
		return nil, err
	}

	return &Collection{
		ID:          int(id),
		Name:        name,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Unix(now, 0),
		UpdatedAt:   time.Unix(now, 0),
	}, nil
}

// GetCollectionByID retrieves a collection by ID with media count
func GetCollectionByID(id int) (*Collection, error) {
	var collection Collection
	var createdAt, updatedAt int64

	err := db.QueryRow(`
		SELECT c.id, c.name, c.description, c.created_by, c.created_at, c.updated_at,
		       COUNT(cm.media_slug) as media_count
		FROM collections c
		LEFT JOIN collection_media cm ON c.id = cm.collection_id
		WHERE c.id = ?
		GROUP BY c.id`, id).Scan(
		&collection.ID,
		&collection.Name,
		&collection.Description,
		&collection.CreatedBy,
		&createdAt,
		&updatedAt,
		&collection.MediaCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		log.Error("Failed to get collection:", err)
		return nil, err
	}

	collection.CreatedAt = time.Unix(createdAt, 0)
	collection.UpdatedAt = time.Unix(updatedAt, 0)

	return &collection, nil
}

// GetCollectionsByUser retrieves all collections created by a user
func GetCollectionsByUser(username string, accessibleLibraries []string) ([]Collection, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.description, c.created_by, c.created_at, c.updated_at,
		       COUNT(cm.media_slug) as media_count
		FROM collections c
		LEFT JOIN collection_media cm ON c.id = cm.collection_id
		WHERE c.created_by = ?
		GROUP BY c.id
		ORDER BY c.created_at DESC`, username)
	if err != nil {
		log.Error("Failed to get user collections:", err)
		return nil, err
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var collection Collection
		var createdAt, updatedAt int64

		err := rows.Scan(
			&collection.ID,
			&collection.Name,
			&collection.Description,
			&collection.CreatedBy,
			&createdAt,
			&updatedAt,
			&collection.MediaCount)
		if err != nil {
			log.Error("Failed to scan collection:", err)
			return nil, err
		}

		collection.CreatedAt = time.Unix(createdAt, 0)
		collection.UpdatedAt = time.Unix(updatedAt, 0)

		// Get top 4 media for poster display
		topMedia, err := GetTopMediaInCollection(collection.ID, accessibleLibraries)
		if err != nil {
			log.Debugf("Failed to get top media for collection %d: %v", collection.ID, err)
		}
		collection.TopMedia = topMedia

		collections = append(collections, collection)
	}

	return collections, nil
}

// GetAllCollections retrieves all collections with media counts
func GetAllCollections(accessibleLibraries []string) ([]Collection, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.description, c.created_by, c.created_at, c.updated_at,
		       COUNT(cm.media_slug) as media_count
		FROM collections c
		LEFT JOIN collection_media cm ON c.id = cm.collection_id
		GROUP BY c.id
		ORDER BY c.created_at DESC`)
	if err != nil {
		log.Error("Failed to get all collections:", err)
		return nil, err
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var collection Collection
		var createdAt, updatedAt int64

		err := rows.Scan(
			&collection.ID,
			&collection.Name,
			&collection.Description,
			&collection.CreatedBy,
			&createdAt,
			&updatedAt,
			&collection.MediaCount)
		if err != nil {
			log.Error("Failed to scan collection:", err)
			return nil, err
		}

		collection.CreatedAt = time.Unix(createdAt, 0)
		collection.UpdatedAt = time.Unix(updatedAt, 0)

		// Get top 4 media for poster display
		topMedia, err := GetTopMediaInCollection(collection.ID, accessibleLibraries)
		if err != nil {
			log.Debugf("Failed to get top media for collection %d: %v", collection.ID, err)
		}
		collection.TopMedia = topMedia

		collections = append(collections, collection)
	}

	return collections, nil
}

// UpdateCollection updates a collection
func UpdateCollection(id int, name, description string) error {
	now := time.Now().Unix()

	_, err := db.Exec(`
		UPDATE collections
		SET name = ?, description = ?, updated_at = ?
		WHERE id = ?`,
		name, description, now, id)
	if err != nil {
		log.Error("Failed to update collection:", err)
		return err
	}

	return nil
}

// DeleteCollection deletes a collection
func DeleteCollection(id int) error {
	_, err := db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		log.Error("Failed to delete collection:", err)
		return err
	}

	return nil
}

// AddMediaToCollection adds media to a collection
func AddMediaToCollection(collectionID int, mediaSlug string) error {
	now := time.Now().Unix()

	_, err := db.Exec(`
		INSERT OR IGNORE INTO collection_media (collection_id, media_slug, added_at)
		VALUES (?, ?, ?)`,
		collectionID, mediaSlug, now)
	if err != nil {
		log.Error("Failed to add media to collection:", err)
		return err
	}

	return nil
}

// RemoveMediaFromCollection removes media from a collection
func RemoveMediaFromCollection(collectionID int, mediaSlug string) error {
	_, err := db.Exec(`
		DELETE FROM collection_media
		WHERE collection_id = ? AND media_slug = ?`,
		collectionID, mediaSlug)
	if err != nil {
		log.Error("Failed to remove media from collection:", err)
		return err
	}

	return nil
}

// GetCollectionMedia retrieves all media in a collection, filtered by accessible libraries
func GetCollectionMedia(collectionID int, accessibleLibraries []string) ([]Media, error) {
	query := `
		SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count,
			COALESCE(read_counts.read_count, 0) as read_count,
			COALESCE(vote_scores.score, 0) as vote_score,
			m.created_at, m.updated_at
		FROM media m
		INNER JOIN collection_media cm ON m.slug = cm.media_slug
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
		WHERE cm.collection_id = ?
		ORDER BY cm.added_at DESC`

	rows, err := db.Query(query, collectionID)
	if err != nil {
		log.Error("Failed to get collection media:", err)
		return nil, err
	}
	defer rows.Close()

	var media []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var voteScore int
		err := rows.Scan(
			&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage,
			&m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL,
			&m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt)
		if err != nil {
			log.Error("Failed to scan media:", err)
			return nil, err
		}

		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore
		media = append(media, m)
	}

	// Filter by accessible libraries
	if len(accessibleLibraries) > 0 {
		media = filterByAccessibleLibraries(media, accessibleLibraries)
	}

	return media, nil
}

// GetTopMediaInCollection retrieves the top 4 most popular media in a collection (by vote score), filtered by accessible libraries
func GetTopMediaInCollection(collectionID int, accessibleLibraries []string) ([]Media, error) {
	query := `
		SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.cover_art_url, m.file_count,
			COALESCE(read_counts.read_count, 0) as read_count,
			COALESCE(vote_scores.score, 0) as vote_score,
			m.created_at, m.updated_at
		FROM media m
		INNER JOIN collection_media cm ON m.slug = cm.media_slug
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
		WHERE cm.collection_id = ?
		ORDER BY vote_scores.score DESC, cm.added_at DESC
		LIMIT 4`

	rows, err := db.Query(query, collectionID)
	if err != nil {
		log.Error("Failed to get top media in collection:", err)
		return nil, err
	}
	defer rows.Close()

	var media []Media
	for rows.Next() {
		var m Media
		var createdAt, updatedAt int64
		var voteScore int
		err := rows.Scan(
			&m.Slug, &m.Name, &m.Author, &m.Description, &m.Year, &m.OriginalLanguage,
			&m.Type, &m.Status, &m.ContentRating, &m.CoverArtURL,
			&m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt)
		if err != nil {
			log.Error("Failed to scan media:", err)
			return nil, err
		}

		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore
		media = append(media, m)
	}

	// Filter by accessible libraries
	if len(accessibleLibraries) > 0 {
		media = filterByAccessibleLibraries(media, accessibleLibraries)
	}

	return media, nil
}

// IsMediaInCollection checks if media is in a collection
func IsMediaInCollection(collectionID int, mediaSlug string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM collection_media
		WHERE collection_id = ? AND media_slug = ?`,
		collectionID, mediaSlug).Scan(&count)
	if err != nil {
		log.Error("Failed to check if media is in collection:", err)
		return false, err
	}

	return count > 0, nil
}

// BatchCheckMediaInCollections checks if a media is in multiple collections (batch operation)
// Returns a map of collection IDs that contain the media
func BatchCheckMediaInCollections(collectionIDs []int, mediaSlug string) (map[int]bool, error) {
	result := make(map[int]bool)

	if len(collectionIDs) == 0 {
		return result, nil
	}

	// Build placeholders for IN clause
	var placeholders strings.Builder
	for i := range collectionIDs {
		if i > 0 {
			placeholders.WriteString(",")
		}
		placeholders.WriteString("?")
	}

	// Convert collection IDs to interface slice for query
	args := make([]any, len(collectionIDs)+1)
	for i, id := range collectionIDs {
		args[i] = id
	}
	args[len(collectionIDs)] = mediaSlug

	// Initialize all collection IDs as false
	for _, id := range collectionIDs {
		result[id] = false
	}

	// Query all matching collection_media in one go
	query := `
		SELECT DISTINCT collection_id FROM collection_media
		WHERE collection_id IN (` + placeholders.String() + `) AND media_slug = ?`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Error("Failed to batch check media in collections:", err)
		return result, err
	}
	defer rows.Close()

	// Mark collections that contain this media
	for rows.Next() {
		var collectionID int
		if err := rows.Scan(&collectionID); err != nil {
			log.Error("Failed to scan collection ID:", err)
			continue
		}
		result[collectionID] = true
	}

	return result, nil
}
