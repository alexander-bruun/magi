package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	
	"github.com/gofiber/fiber/v2/log"
)

// SessionToken represents a user session
type SessionToken struct {
	Token      string    `json:"token"`
	Username   string    `json:"username"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

const SessionTokenDuration = 30 * 24 * time.Hour // 1 month

// GenerateRandomKey creates a new random key of the specified length
func GenerateRandomKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// CreateSessionToken generates a new session token for a user
func CreateSessionToken(username string) (string, error) {
	// Generate a random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	now := time.Now()
	expiresAt := now.Add(SessionTokenDuration)

	query := `
	INSERT INTO session_tokens (token, username, created_at, expires_at, last_used_at)
	VALUES (?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, token, username, now, expiresAt, now)
	if err != nil {
		return "", fmt.Errorf("failed to store session token: %w", err)
	}

	return token, nil
}

// ValidateSessionToken validates a session token and updates last_used_at
func ValidateSessionToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}

	query := `
	SELECT username, expires_at
	FROM session_tokens
	WHERE token = ?
	`
	row := db.QueryRow(query, token)

	var username string
	var expiresAt time.Time
	err := row.Scan(&username, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("invalid session token")
		}
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	// Check if token is expired
	if time.Now().After(expiresAt) {
		// Clean up expired token
		DeleteSessionToken(token)
		return "", errors.New("session token expired")
	}

	// Update last_used_at timestamp
	updateQuery := `
	UPDATE session_tokens
	SET last_used_at = ?
	WHERE token = ?
	`
	_, err = db.Exec(updateQuery, time.Now(), token)
	if err != nil {
		// Log error but don't fail validation
		log.Errorf("Warning: failed to update last_used_at: %v\n", err)
	}

	return username, nil
}

// DeleteSessionToken removes a session token from the database
func DeleteSessionToken(token string) error {
	query := `
	DELETE FROM session_tokens
	WHERE token = ?
	`
	_, err := db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("failed to delete session token: %w", err)
	}
	return nil
}

// DeleteAllUserSessions removes all session tokens for a specific user
func DeleteAllUserSessions(username string) error {
	query := `
	DELETE FROM session_tokens
	WHERE username = ?
	`
	_, err := db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes all expired session tokens
func CleanupExpiredSessions() error {
	query := `
	DELETE FROM session_tokens
	WHERE expires_at < ?
	`
	_, err := db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}

// GetUserSessions retrieves all active sessions for a user
func GetUserSessions(username string) ([]SessionToken, error) {
	query := `
	SELECT token, username, created_at, expires_at, last_used_at
	FROM session_tokens
	WHERE username = ? AND expires_at > ?
	ORDER BY last_used_at DESC
	`
	rows, err := db.Query(query, username, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionToken
	for rows.Next() {
		var session SessionToken
		err := rows.Scan(&session.Token, &session.Username, &session.CreatedAt, &session.ExpiresAt, &session.LastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}
