package models

import (
	"errors"
	"time"
)

// Comment represents a comment on media or chapter
type Comment struct {
	ID          int       `json:"id"`
	UserUsername string   `json:"user_username"`
	TargetType  string   `json:"target_type"` // "media" or "chapter"
	TargetSlug  string   `json:"target_slug"`
	MediaSlug   string   `json:"media_slug"`
	Content     string   `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateComment adds a new comment
func CreateComment(comment Comment) error {
	query := `
	INSERT INTO comments (user_username, target_type, target_slug, media_slug, content, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	timestamps := NewTimestamps()
	comment.CreatedAt = timestamps.CreatedAt
	comment.UpdatedAt = timestamps.UpdatedAt

	_, err := db.Exec(query, comment.UserUsername, comment.TargetType, comment.TargetSlug, comment.MediaSlug, comment.Content, timestamps.CreatedAt.Unix(), timestamps.UpdatedAt.Unix())
	return err
}

// GetCommentsByTarget retrieves comments for a specific target
func GetCommentsByTarget(targetType, targetSlug string) ([]Comment, error) {
	query := `
	SELECT id, user_username, target_type, target_slug, media_slug, content, created_at, updated_at
	FROM comments
	WHERE target_type = ? AND target_slug = ?
	ORDER BY created_at DESC
	`

	rows, err := db.Query(query, targetType, targetSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var comment Comment
		var createdAt, updatedAt int64
		err := rows.Scan(&comment.ID, &comment.UserUsername, &comment.TargetType, &comment.TargetSlug, &comment.MediaSlug, &comment.Content, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		comment.CreatedAt = time.Unix(createdAt, 0)
		comment.UpdatedAt = time.Unix(updatedAt, 0)
		comments = append(comments, comment)
	}

	return comments, nil
}

// GetCommentsByTargetAndMedia retrieves comments for a specific target and media
func GetCommentsByTargetAndMedia(targetType, targetSlug, mediaSlug string) ([]Comment, error) {
	query := `
	SELECT id, user_username, target_type, target_slug, media_slug, content, created_at, updated_at
	FROM comments
	WHERE target_type = ? AND target_slug = ? AND media_slug = ?
	ORDER BY created_at DESC
	`

	rows, err := db.Query(query, targetType, targetSlug, mediaSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var comment Comment
		var createdAt, updatedAt int64
		err := rows.Scan(&comment.ID, &comment.UserUsername, &comment.TargetType, &comment.TargetSlug, &comment.MediaSlug, &comment.Content, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		comment.CreatedAt = time.Unix(createdAt, 0)
		comment.UpdatedAt = time.Unix(updatedAt, 0)
		comments = append(comments, comment)
	}

	return comments, nil
}

// DeleteComment removes a comment by ID (only by the author or moderator)
func DeleteComment(id int, userUsername string) error {
	query := `
	DELETE FROM comments
	WHERE id = ? AND user_username = ?
	`

	result, err := db.Exec(query, id, userUsername)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("comment not found or not authorized")
	}

	return nil
}