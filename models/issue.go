package models

import (
	"time"
)

// Issue represents a user-reported issue
type Issue struct {
	ID           int        `json:"id"`
	UserUsername *string    `json:"user_username,omitempty"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Priority     string     `json:"priority"`
	Category     string     `json:"category"`
	UserAgent    string     `json:"user_agent,omitempty"`
	URL          string     `json:"url,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	Resolution   string     `json:"resolution,omitempty"`
}

// CreateIssue adds a new issue report
func CreateIssue(issue Issue) error {
	query := `
	INSERT INTO issues (user_username, title, description, status, priority, category, user_agent, url, created_at, updated_at, resolution)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	timestamps := NewTimestamps()

	_, err := db.Exec(query, issue.UserUsername, issue.Title, issue.Description, issue.Status, issue.Priority, issue.Category, issue.UserAgent, issue.URL, timestamps.CreatedAt.Unix(), timestamps.UpdatedAt.Unix(), issue.Resolution)
	return err
}

// GetIssues retrieves issues with optional filtering
func GetIssues(status, category string, limit, offset int) ([]Issue, error) {
	query := `
	SELECT id, user_username, title, description, status, priority, category, user_agent, url, created_at, updated_at, resolved_at, resolution
	FROM issues
	WHERE 1=1
	`
	args := []any{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var issue Issue
		var userUsername *string
		var resolvedAt *int64
		var createdAt int64
		var updatedAt int64

		err := rows.Scan(&issue.ID, &userUsername, &issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.Category, &issue.UserAgent, &issue.URL, &createdAt, &updatedAt, &resolvedAt, &issue.Resolution)
		if err != nil {
			return nil, err
		}

		issue.UserUsername = userUsername
		issue.CreatedAt = time.Unix(createdAt, 0)
		issue.UpdatedAt = time.Unix(updatedAt, 0)
		if resolvedAt != nil {
			t := time.Unix(*resolvedAt, 0)
			issue.ResolvedAt = &t
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// GetIssueByID retrieves a single issue by ID
func GetIssueByID(id int) (*Issue, error) {
	query := `
	SELECT id, user_username, title, description, status, priority, category, user_agent, url, created_at, updated_at, resolved_at, resolution
	FROM issues
	WHERE id = ?
	`

	var issue Issue
	var userUsername *string
	var resolvedAt *int64
	var createdAt int64
	var updatedAt int64

	err := db.QueryRow(query, id).Scan(&issue.ID, &userUsername, &issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.Category, &issue.UserAgent, &issue.URL, &createdAt, &updatedAt, &resolvedAt, &issue.Resolution)
	if err != nil {
		return nil, err
	}

	issue.UserUsername = userUsername
	issue.CreatedAt = time.Unix(createdAt, 0)
	issue.UpdatedAt = time.Unix(updatedAt, 0)
	if resolvedAt != nil {
		t := time.Unix(*resolvedAt, 0)
		issue.ResolvedAt = &t
	}

	return &issue, nil
}

// UpdateIssueStatus updates the status of an issue
func UpdateIssueStatus(id int, status string) error {
	return UpdateIssueResolution(id, status, "")
}

// UpdateIssueResolution updates the status and resolution of an issue
func UpdateIssueResolution(id int, status string, resolution string) error {
	query := `
	UPDATE issues
	SET status = ?, updated_at = ?
	WHERE id = ?
	`

	if status == "closed" {
		query = `
		UPDATE issues
		SET status = ?, resolved_at = ?, resolution = ?, updated_at = ?
		WHERE id = ?
		`
		timestamps := NewTimestamps()
		_, err := db.Exec(query, status, timestamps.CreatedAt.Unix(), resolution, timestamps.CreatedAt.Unix(), id)
		return err
	}

	timestamps := NewTimestamps()
	_, err := db.Exec(query, status, timestamps.CreatedAt.Unix(), id)
	return err
}

// GetIssueStats returns statistics about issues
func GetIssueStats() (map[string]int, error) {
	query := `
	SELECT status, COUNT(*) as count
	FROM issues
	GROUP BY status
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		err := rows.Scan(&status, &count)
		if err != nil {
			return nil, err
		}
		stats[status] = count
	}

	return stats, nil
}
