package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// ScraperScript represents a scraper script stored in the database
type ScraperScript struct {
	ID              int64
	Name            string
	Script          string
	Language        string // "bash"
	Schedule        string // Cron format (e.g., "0 0 * * *")
	Variables       map[string]string // Key-value pairs for script variables
	LastRun         *int64 // Unix timestamp
	LastRunOutput   *string
	LastRunError    *string
	CreatedAt       int64
	UpdatedAt       int64
	Enabled         bool
}

// CreateScraperScript creates a new scraper script in the database
func CreateScraperScript(name, script, language, schedule string, variables map[string]string) (*ScraperScript, error) {
	now := time.Now().Unix()
	
	// Serialize variables to JSON
	variablesJSON := "{}"
	if len(variables) > 0 {
		data, err := json.Marshal(variables)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize variables: %w", err)
		}
		variablesJSON = string(data)
	}
	
	query := `
		INSERT INTO scraper_scripts (name, script, language, schedule, variables, created_at, updated_at, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
	`
	result, err := db.Exec(query, name, script, language, schedule, variablesJSON, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create scraper script: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &ScraperScript{
		ID:        id,
		Name:      name,
		Script:    script,
		Language:  language,
		Schedule:  schedule,
		Variables: variables,
		CreatedAt: now,
		UpdatedAt: now,
		Enabled:   true,
	}, nil
}

// GetScraperScript retrieves a script by ID
func GetScraperScript(id int64) (*ScraperScript, error) {
	query := `
		SELECT id, name, script, language, schedule, variables, last_run, last_run_output, last_run_error, created_at, updated_at, enabled
		FROM scraper_scripts
		WHERE id = ?
	`
	row := db.QueryRow(query, id)
	return scanScraperScript(row)
}

// GetScraperScriptByName retrieves a script by name
func GetScraperScriptByName(name string) (*ScraperScript, error) {
	query := `
		SELECT id, name, script, language, schedule, variables, last_run, last_run_output, last_run_error, created_at, updated_at, enabled
		FROM scraper_scripts
		WHERE name = ?
	`
	row := db.QueryRow(query, name)
	return scanScraperScript(row)
}

// ListScraperScripts retrieves all scraper scripts, optionally filtered by enabled status
func ListScraperScripts(enabledOnly bool) ([]ScraperScript, error) {
	query := `
		SELECT id, name, script, language, schedule, variables, last_run, last_run_output, last_run_error, created_at, updated_at, enabled
		FROM scraper_scripts
	`
	args := []interface{}{}

	if enabledOnly {
		query += " WHERE enabled = 1"
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scraper scripts: %w", err)
	}
	defer rows.Close()

	var scripts []ScraperScript
	for rows.Next() {
		script, err := scanScraperScript(rows)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, *script)
	}

	return scripts, rows.Err()
}

// UpdateScraperScript updates a scraper script
func UpdateScraperScript(id int64, name, script, language, schedule string, variables map[string]string) (*ScraperScript, error) {
	now := time.Now().Unix()
	
	// Serialize variables to JSON
	variablesJSON := "{}"
	if len(variables) > 0 {
		data, err := json.Marshal(variables)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize variables: %w", err)
		}
		variablesJSON = string(data)
	}
	
	query := `
		UPDATE scraper_scripts
		SET name = ?, script = ?, language = ?, schedule = ?, variables = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, name, script, language, schedule, variablesJSON, now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update scraper script: %w", err)
	}

	return GetScraperScript(id)
}

// UpdateScraperScriptLastRun updates the last run timestamp and output/error
func UpdateScraperScriptLastRun(id int64, output string, errMsg string) error {
	now := time.Now().Unix()
	query := `
		UPDATE scraper_scripts
		SET last_run = ?, last_run_output = ?, last_run_error = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, now, output, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("failed to update script last run: %w", err)
	}
	return nil
}

// DeleteScraperScript deletes a scraper script
func DeleteScraperScript(id int64) error {
	query := `DELETE FROM scraper_scripts WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete scraper script: %w", err)
	}
	return nil
}

// EnableScraperScript enables or disables a script
func EnableScraperScript(id int64, enabled bool) error {
	now := time.Now().Unix()
	query := `
		UPDATE scraper_scripts
		SET enabled = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, enabled, now, id)
	if err != nil {
		return fmt.Errorf("failed to enable/disable scraper script: %w", err)
	}
	return nil
}

// scanScraperScript scans a row into a ScraperScript
func scanScraperScript(row interface{ Scan(...interface{}) error }) (*ScraperScript, error) {
	var (
		id            int64
		name          string
		script        string
		language      string
		schedule      string
		variablesJSON sql.NullString
		lastRun       sql.NullInt64
		lastRunOutput sql.NullString
		lastRunError  sql.NullString
		createdAt     int64
		updatedAt     int64
		enabled       bool
	)

	err := row.Scan(&id, &name, &script, &language, &schedule, &variablesJSON, &lastRun, &lastRunOutput, &lastRunError, &createdAt, &updatedAt, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scraper script not found")
		}
		return nil, fmt.Errorf("failed to scan scraper script: %w", err)
	}

	// Parse variables JSON
	variables := make(map[string]string)
	if variablesJSON.Valid && variablesJSON.String != "" {
		err := json.Unmarshal([]byte(variablesJSON.String), &variables)
		if err != nil {
			// If parsing fails, just use empty map
			variables = make(map[string]string)
		}
	}

	ss := &ScraperScript{
		ID:        id,
		Name:      name,
		Script:    script,
		Language:  language,
		Schedule:  schedule,
		Variables: variables,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Enabled:   enabled,
	}

	if lastRun.Valid {
		ss.LastRun = &lastRun.Int64
	}
	if lastRunOutput.Valid {
		ss.LastRunOutput = &lastRunOutput.String
	}
	if lastRunError.Valid {
		ss.LastRunError = &lastRunError.String
	}

	return ss, nil
}

// FormatLastRun returns a human-readable last run time
func (s *ScraperScript) FormatLastRun() string {
	if s.LastRun == nil {
		return "Never"
	}
	t := time.Unix(*s.LastRun, 0)
	return t.Format("2006-01-02 15:04:05")
}

// StatusBadge returns a status string for the script
func (s *ScraperScript) StatusBadge() string {
	if !s.Enabled {
		return "Disabled"
	}
	if s.LastRun == nil {
		return "Pending"
	}
	if s.LastRunError != nil && *s.LastRunError != "" {
		return "Error"
	}
	return "OK"
}

// ValidateScript checks if the script content is valid
func ValidateScript(script, language string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("script cannot be empty")
	}

	if language != "bash" {
		return fmt.Errorf("invalid language: %s, must be 'bash'", language)
	}

	return nil
}

// ========================================
// Scraper Execution Logs
// ========================================

// ScraperExecutionLog represents a single execution log entry
type ScraperExecutionLog struct {
	ID           int64
	ScriptID     int64
	Status       string // "running", "success", "error"
	Output       *string
	ErrorMessage *string
	StartTime    int64
	EndTime      *int64
	DurationMs   *int64
	CreatedAt    int64
}

// CreateScraperLog creates a new execution log entry
func CreateScraperLog(scriptID int64, status string) (*ScraperExecutionLog, error) {
	now := time.Now().Unix()
	query := `
		INSERT INTO scraper_execution_logs (script_id, status, start_time, created_at)
		VALUES (?, ?, ?, ?)
	`
	result, err := db.Exec(query, scriptID, status, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &ScraperExecutionLog{
		ID:        id,
		ScriptID:  scriptID,
		Status:    status,
		StartTime: now,
		CreatedAt: now,
	}, nil
}

// GetScraperLog retrieves a specific execution log
func GetScraperLog(id int64) (*ScraperExecutionLog, error) {
	query := `
		SELECT id, script_id, status, output, error_message, start_time, end_time, duration_ms, created_at
		FROM scraper_execution_logs
		WHERE id = ?
	`
	row := db.QueryRow(query, id)
	return scanScraperLog(row)
}

// ListScraperLogs retrieves all execution logs for a script, ordered by most recent
func ListScraperLogs(scriptID int64, limit int) ([]ScraperExecutionLog, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}
	query := `
		SELECT id, script_id, status, output, error_message, start_time, end_time, duration_ms, created_at
		FROM scraper_execution_logs
		WHERE script_id = ?
		ORDER BY start_time DESC
		LIMIT ?
	`
	rows, err := db.Query(query, scriptID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query execution logs: %w", err)
	}
	defer rows.Close()

	var logs []ScraperExecutionLog
	for rows.Next() {
		log, err := scanScraperLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}

	return logs, rows.Err()
}

// UpdateScraperLog updates an execution log with final results
func UpdateScraperLog(id int64, status, output, errorMsg string) error {
	now := time.Now().Unix()
	query := `
		UPDATE scraper_execution_logs
		SET status = ?, output = ?, error_message = ?, end_time = ?, duration_ms = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, status, output, errorMsg, now, (now-time.Now().Unix())*1000, id)
	if err != nil {
		return fmt.Errorf("failed to update execution log: %w", err)
	}
	return nil
}

// UpdateScraperLogFinal updates an execution log with final results and duration
func UpdateScraperLogFinal(id int64, status, output, errorMsg string, durationMs int64) error {
	now := time.Now().Unix()
	query := `
		UPDATE scraper_execution_logs
		SET status = ?, output = ?, error_message = ?, end_time = ?, duration_ms = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, status, output, errorMsg, now, durationMs, id)
	if err != nil {
		return fmt.Errorf("failed to update execution log: %w", err)
	}
	return nil
}

// scanScraperLog scans a row into a ScraperExecutionLog
func scanScraperLog(row interface{ Scan(...interface{}) error }) (*ScraperExecutionLog, error) {
	var (
		id           int64
		scriptID     int64
		status       string
		output       sql.NullString
		errorMessage sql.NullString
		startTime    int64
		endTime      sql.NullInt64
		durationMs   sql.NullInt64
		createdAt    int64
	)

	err := row.Scan(&id, &scriptID, &status, &output, &errorMessage, &startTime, &endTime, &durationMs, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("execution log not found")
		}
		return nil, fmt.Errorf("failed to scan execution log: %w", err)
	}

	log := &ScraperExecutionLog{
		ID:        id,
		ScriptID:  scriptID,
		Status:    status,
		StartTime: startTime,
		CreatedAt: createdAt,
	}

	if output.Valid {
		log.Output = &output.String
	}
	if errorMessage.Valid {
		log.ErrorMessage = &errorMessage.String
	}
	if endTime.Valid {
		log.EndTime = &endTime.Int64
	}
	if durationMs.Valid {
		log.DurationMs = &durationMs.Int64
	}

	return log, nil
}

// FormatStartTime returns a human-readable start time
func (l *ScraperExecutionLog) FormatStartTime() string {
	t := time.Unix(l.StartTime, 0)
	return t.Format("2006-01-02 15:04:05")
}

// FormatDuration returns a human-readable duration
func (l *ScraperExecutionLog) FormatDuration() string {
	if l.DurationMs == nil {
		return "In progress..."
	}
	ms := *l.DurationMs
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000
	return fmt.Sprintf("%.2fs", seconds)
}

// AbortOrphanedRunningLogs marks all "running" logs as "aborted" that were left
// in that state due to application shutdown or crash. This should be called on startup.
func AbortOrphanedRunningLogs() error {
	query := `
		UPDATE scraper_execution_logs
		SET status = 'aborted', 
		    error_message = 'Execution interrupted by application shutdown',
		    end_time = ?
		WHERE status = 'running'
	`
	now := time.Now().Unix()
	result, err := db.Exec(query, now)
	if err != nil {
		return fmt.Errorf("failed to abort orphaned running logs: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected > 0 {
		log.Infof("Marked %d orphaned running log(s) as aborted", rowsAffected)
	}
	
	return nil
}
