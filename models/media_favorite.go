package models

import (
	"database/sql"
	"time"
)

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
