package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Vote represents a user's upvote or downvote on a media.
type Vote struct {
	ID           int64
	UserUsername string
	MediaSlug    string
	Value        int // 1 for upvote, -1 for downvote
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SQL fragments for vote score calculations, shared across multiple queries.
const (
	// voteScoreExpr calculates the vote score as a 0-10 integer (upvote percentage × 10).
	voteScoreExpr = `CASE WHEN COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0) > 0
		THEN ROUND((COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) * 1.0 / (COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0) + COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0))) * 10)
		ELSE 0 END`

	// voteUpvotesExpr counts upvotes.
	voteUpvotesExpr = `COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END),0)`

	// voteDownvotesExpr counts downvotes.
	voteDownvotesExpr = `COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END),0)`

	// VoteScoreSubquery is a complete subquery that calculates per-media vote scores.
	// Use with fmt.Sprintf to inject an optional WHERE clause: fmt.Sprintf(VoteScoreSubquery, "AND created_at > ?")
	VoteScoreSubquery = `SELECT media_slug, ` + voteScoreExpr + ` as score FROM votes WHERE 1=1 %s GROUP BY media_slug`
)

// GetMediaVotes returns the aggregated score and counts for a media
func GetMediaVotes(mangaSlug string) (score int, upvotes int, downvotes int, err error) {
	query := `SELECT ` + voteScoreExpr + ` as score, ` + voteUpvotesExpr + ` as upvotes, ` + voteDownvotesExpr + ` as downvotes FROM votes WHERE media_slug = ?`
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
	query := fmt.Sprintf(`SELECT media_slug, `+voteScoreExpr+` as score, `+voteUpvotesExpr+` as upvotes, `+voteDownvotesExpr+` as downvotes FROM votes WHERE media_slug IN (%s) GROUP BY media_slug`, placeholders)

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

// getVotedMediaSlugsForUser returns media slugs the user has voted on with the given value, ordered by most recent vote.
func getVotedMediaSlugsForUser(username string, value int) ([]string, error) {
	rows, err := db.Query(`SELECT media_slug FROM votes WHERE user_username = ? AND value = ? ORDER BY updated_at DESC`, username, value)
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
	return getVotedMediaSlugsForUser(username, 1)
}

// GetDownvotedMediasForUser returns media slugs the user has downvoted (value = -1), ordered by most recent vote
func GetDownvotedMediasForUser(username string) ([]string, error) {
	return getVotedMediaSlugsForUser(username, -1)
}
