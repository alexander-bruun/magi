package models

import (
	"database/sql"
	"errors"
	"time"
)

// Review represents a review with rating on media
type Review struct {
	ID           int       `json:"id"`
	UserUsername string    `json:"user_username"`
	MediaSlug    string    `json:"media_slug"`
	Rating       int       `json:"rating"` // 1-10
	Content      string    `json:"content,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateReview adds or updates a review (unique per user per media)
func CreateReview(review Review) error {
	query := `
	INSERT INTO reviews (user_username, media_slug, rating, content, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_username, media_slug) DO UPDATE SET
		rating = excluded.rating,
		content = excluded.content,
		updated_at = excluded.updated_at
	`

	timestamps := NewTimestamps()

	_, err := db.Exec(query, review.UserUsername, review.MediaSlug, review.Rating, review.Content, timestamps.CreatedAt.Unix(), timestamps.UpdatedAt.Unix())
	return err
}

// GetReviewsByMedia retrieves all reviews for a media
func GetReviewsByMedia(mediaSlug string) ([]Review, error) {
	query := `
	SELECT id, user_username, media_slug, rating, content, created_at, updated_at
	FROM reviews
	WHERE media_slug = ?
	ORDER BY created_at DESC
	`

	rows, err := db.Query(query, mediaSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var review Review
		var createdAt, updatedAt int64
		err := rows.Scan(&review.ID, &review.UserUsername, &review.MediaSlug, &review.Rating, &review.Content, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		review.CreatedAt = time.Unix(createdAt, 0)
		review.UpdatedAt = time.Unix(updatedAt, 0)
		reviews = append(reviews, review)
	}

	return reviews, nil
}

// GetReviewByUserAndMedia gets a specific user's review for a media
func GetReviewByUserAndMedia(userUsername, mediaSlug string) (*Review, error) {
	query := `
	SELECT id, user_username, media_slug, rating, content, created_at, updated_at
	FROM reviews
	WHERE user_username = ? AND media_slug = ?
	`

	var review Review
	var createdAt, updatedAt int64
	err := db.QueryRow(query, userUsername, mediaSlug).Scan(&review.ID, &review.UserUsername, &review.MediaSlug, &review.Rating, &review.Content, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	review.CreatedAt = time.Unix(createdAt, 0)
	review.UpdatedAt = time.Unix(updatedAt, 0)
	return &review, nil
}

// GetReviewByID gets a review by ID
func GetReviewByID(reviewID int) (*Review, error) {
	query := `
	SELECT id, user_username, media_slug, rating, content, created_at, updated_at
	FROM reviews
	WHERE id = ?
	`

	var review Review
	var createdAt, updatedAt int64
	err := db.QueryRow(query, reviewID).Scan(&review.ID, &review.UserUsername, &review.MediaSlug, &review.Rating, &review.Content, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	review.CreatedAt = time.Unix(createdAt, 0)
	review.UpdatedAt = time.Unix(updatedAt, 0)
	return &review, nil
}

// GetAverageRating calculates the average rating for a media
func GetAverageRating(mediaSlug string) (float64, int, error) {
	query := `
	SELECT COALESCE(AVG(rating), 0), COUNT(*)
	FROM reviews
	WHERE media_slug = ?
	`

	var avg float64
	var count int
	err := db.QueryRow(query, mediaSlug).Scan(&avg, &count)
	if err != nil {
		return 0, 0, err
	}

	return avg, count, nil
}

// DeleteReview removes a review by user and media
func DeleteReview(userUsername, mediaSlug string) error {
	query := `
	DELETE FROM reviews
	WHERE user_username = ? AND media_slug = ?
	`

	result, err := db.Exec(query, userUsername, mediaSlug)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("review not found")
	}

	return nil
}

// DeleteReviewByID removes a review by ID
func DeleteReviewByID(reviewID int) error {
	query := `
	DELETE FROM reviews
	WHERE id = ?
	`

	result, err := db.Exec(query, reviewID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("review not found")
	}

	return nil
}

// UpdateReviewByID updates a review by ID
func UpdateReviewByID(reviewID int, rating int, content string) error {
	query := `
	UPDATE reviews
	SET rating = ?, content = ?
	WHERE id = ?
	`

	result, err := db.Exec(query, rating, content, reviewID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("review not found")
	}

	return nil
}
