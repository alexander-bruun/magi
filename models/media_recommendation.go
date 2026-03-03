package models

import (
	"database/sql"
	"time"
)

type MediaRecommendation struct {
	MediaSlug       string
	RecommendedSlug string
	Score           int
	CreatedAt       time.Time
}

// SaveMediaRecommendations stores recommendations for a media (overwrites existing)
func SaveMediaRecommendations(mediaSlug string, recs []MediaRecommendation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()
	_, err = tx.Exec("DELETE FROM media_recommendations WHERE media_slug = ?", mediaSlug)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO media_recommendations (media_slug, recommended_slug, score) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, rec := range recs {
		_, err = stmt.Exec(mediaSlug, rec.RecommendedSlug, rec.Score)
		if err != nil {
			return err
		}
		// Ensure reciprocal recommendation exists (recommended -> media)
		var tmp int
		err = tx.QueryRow("SELECT 1 FROM media_recommendations WHERE media_slug = ? AND recommended_slug = ? LIMIT 1", rec.RecommendedSlug, mediaSlug).Scan(&tmp)
		if err == sql.ErrNoRows {
			// Insert reciprocal entry with same score
			_, err = stmt.Exec(rec.RecommendedSlug, mediaSlug, rec.Score)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// GetMediaRecommendations fetches recommended media slugs for a given media_slug, ordered by score desc
func GetMediaRecommendations(mediaSlug string, limit int) ([]string, error) {
	query := "SELECT recommended_slug FROM media_recommendations WHERE media_slug = ? ORDER BY score DESC LIMIT ?"
	rows, err := db.Query(query, mediaSlug, limit)
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
	return slugs, nil
}
