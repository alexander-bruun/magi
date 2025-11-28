package models

import (
	"database/sql"
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
func GetCollectionsByUser(username string) ([]Collection, error) {
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
		collections = append(collections, collection)
	}

	return collections, nil
}

// GetAllCollections retrieves all collections with media counts
func GetAllCollections() ([]Collection, error) {
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

// GetCollectionMedia retrieves all media in a collection
func GetCollectionMedia(collectionID int) ([]Media, error) {
	query := `
		SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language, m.type, m.status, m.content_rating, m.library_slug, m.cover_art_url, m.path, m.file_count,
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
			&m.Type, &m.Status, &m.ContentRating, &m.LibrarySlug, &m.CoverArtURL,
			&m.Path, &m.FileCount, &m.ReadCount, &voteScore, &createdAt, &updatedAt)
		if err != nil {
			log.Error("Failed to scan media:", err)
			return nil, err
		}

		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		m.VoteScore = voteScore
		media = append(media, m)
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