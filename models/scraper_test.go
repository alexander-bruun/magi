package models

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestScraperScript_FormatLastRun(t *testing.T) {
	tests := []struct {
		name     string
		lastRun  *int64
		expected string
	}{
		{
			name:     "nil last run returns Never",
			lastRun:  nil,
			expected: "Never",
		},
		{
			name: "valid timestamp formats correctly",
			lastRun: func() *int64 {
				// Use a fixed time in UTC to avoid timezone issues
				ts := int64(1640995200) // 2022-01-01 00:00:00 UTC
				return &ts
			}(),
			expected: "2022-01-01 00:00:00", // This will be adjusted by timezone, so we'll check format instead
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ScraperScript{LastRun: tt.lastRun}
			result := s.FormatLastRun()
			if tt.lastRun == nil {
				assert.Equal(t, tt.expected, result)
			} else {
				// For timestamps, just verify it's in the expected format YYYY-MM-DD HH:MM:SS
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
			}
		})
	}
}

func TestScraperScript_StatusBadge(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		lastRun  *int64
		lastErr  *string
		expected string
	}{
		{
			name:     "disabled script returns Disabled",
			enabled:  false,
			lastRun:  nil,
			lastErr:  nil,
			expected: "Disabled",
		},
		{
			name:     "enabled script with no last run returns Pending",
			enabled:  true,
			lastRun:  nil,
			lastErr:  nil,
			expected: "Pending",
		},
		{
			name:     "enabled script with error returns Error",
			enabled:  true,
			lastRun:  func() *int64 { ts := int64(1640995200); return &ts }(),
			lastErr:  func() *string { s := "some error"; return &s }(),
			expected: "Error",
		},
		{
			name:     "enabled script with successful run returns OK",
			enabled:  true,
			lastRun:  func() *int64 { ts := int64(1640995200); return &ts }(),
			lastErr:  nil,
			expected: "OK",
		},
		{
			name:     "enabled script with empty error string returns OK",
			enabled:  true,
			lastRun:  func() *int64 { ts := int64(1640995200); return &ts }(),
			lastErr:  func() *string { s := ""; return &s }(),
			expected: "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ScraperScript{
				Enabled:     tt.enabled,
				LastRun:     tt.lastRun,
				LastRunError: tt.lastErr,
			}
			result := s.StatusBadge()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScraperExecutionLog_FormatStartTime(t *testing.T) {
	tests := []struct {
		name      string
		startTime int64
	}{
		{
			name:      "formats start time correctly",
			startTime: int64(1640995200), // 2022-01-01 00:00:00 UTC
		},
		{
			name:      "formats different timestamp correctly",
			startTime: int64(1609459200), // 2021-01-01 00:00:00 UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ScraperExecutionLog{StartTime: tt.startTime}
			result := l.FormatStartTime()
			// Verify it's in the expected format YYYY-MM-DD HH:MM:SS
			assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
		})
	}
}

func TestScraperExecutionLog_FormatDuration(t *testing.T) {
	tests := []struct {
		name       string
		durationMs *int64
		expected   string
	}{
		{
			name:       "nil duration returns In progress",
			durationMs: nil,
			expected:   "In progress...",
		},
		{
			name:       "duration less than 1000ms formats as ms",
			durationMs: func() *int64 { d := int64(500); return &d }(),
			expected:   "500ms",
		},
		{
			name:       "duration 1000ms formats as seconds",
			durationMs: func() *int64 { d := int64(1000); return &d }(),
			expected:   "1.00s",
		},
		{
			name:       "duration more than 1000ms formats as seconds",
			durationMs: func() *int64 { d := int64(2500); return &d }(),
			expected:   "2.50s",
		},
		{
			name:       "duration with decimal precision",
			durationMs: func() *int64 { d := int64(1234); return &d }(),
			expected:   "1.23s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ScraperExecutionLog{DurationMs: tt.durationMs}
			result := l.FormatDuration()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateScraperScript(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	name := "test-scraper"
	script := "echo 'hello world'"
	language := "bash"
	schedule := "0 0 * * *"
	variables := map[string]string{"key1": "value1", "key2": "value2"}
	packages := []string{"requests", "beautifulsoup4"}
	sharedScript := "source /shared/setup.sh"

	mock.ExpectExec(`INSERT INTO scraper_scripts`).
		WithArgs(name, script, language, schedule, `{"key1":"value1","key2":"value2"}`, `["requests","beautifulsoup4"]`, sharedScript, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := CreateScraperScript(name, script, language, schedule, variables, packages, &sharedScript)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, name, result.Name)
	assert.Equal(t, script, result.Script)
	assert.Equal(t, language, result.Language)
	assert.Equal(t, schedule, result.Schedule)
	assert.Equal(t, variables, result.Variables)
	assert.Equal(t, packages, result.Packages)
	assert.Equal(t, &sharedScript, result.SharedScript)
	assert.True(t, result.Enabled)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateScraperScript_EmptyVariablesAndPackages(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	name := "simple-scraper"
	script := "echo 'simple'"
	language := "bash"
	schedule := "0 0 * * *"
	variables := map[string]string{}
	packages := []string{}

	mock.ExpectExec(`INSERT INTO scraper_scripts`).
		WithArgs(name, script, language, schedule, "{}", "[]", nil, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))

	result, err := CreateScraperScript(name, script, language, schedule, variables, packages, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(2), result.ID)
	assert.Equal(t, name, result.Name)
	assert.Equal(t, variables, result.Variables)
	assert.Equal(t, packages, result.Packages)
	assert.Nil(t, result.SharedScript)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetScraperScript(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	expectedScript := &ScraperScript{
		ID:        1,
		Name:      "test-scraper",
		Script:    "echo 'hello'",
		Language:  "bash",
		Schedule:  "0 0 * * *",
		Variables: map[string]string{"key": "value"},
		Packages:  []string{"requests"},
		CreatedAt: 1640995200,
		UpdatedAt: 1640995200,
		Enabled:   true,
	}

	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "script", "language", "schedule", "last_run", "last_run_output", "last_run_error", "created_at", "updated_at", "enabled", "variables", "packages", "shared_script",
		}).AddRow(
			1, "test-scraper", "echo 'hello'", "bash", "0 0 * * *", nil, nil, nil, 1640995200, 1640995200, true, `{"key":"value"}`, `["requests"]`, nil,
		))

	result, err := GetScraperScript(1)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedScript.ID, result.ID)
	assert.Equal(t, expectedScript.Name, result.Name)
	assert.Equal(t, expectedScript.Script, result.Script)
	assert.Equal(t, expectedScript.Language, result.Language)
	assert.Equal(t, expectedScript.Schedule, result.Schedule)
	assert.Equal(t, expectedScript.Variables, result.Variables)
	assert.Equal(t, expectedScript.Packages, result.Packages)
	assert.Equal(t, expectedScript.CreatedAt, result.CreatedAt)
	assert.Equal(t, expectedScript.UpdatedAt, result.UpdatedAt)
	assert.Equal(t, expectedScript.Enabled, result.Enabled)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetScraperScript_NotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts WHERE id = \?`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	result, err := GetScraperScript(999)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "scraper script not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetScraperScriptByName(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts WHERE name = \?`).
		WithArgs("test-scraper").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "script", "language", "schedule", "last_run", "last_run_output", "last_run_error", "created_at", "updated_at", "enabled", "variables", "packages", "shared_script",
		}).AddRow(
			1, "test-scraper", "echo 'hello'", "bash", "0 0 * * *", nil, nil, nil, 1640995200, 1640995200, true, "{}", "[]", nil,
		))

	result, err := GetScraperScriptByName("test-scraper")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-scraper", result.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListScraperScripts(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts ORDER BY created_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "script", "language", "schedule", "last_run", "last_run_output", "last_run_error", "created_at", "updated_at", "enabled", "variables", "packages", "shared_script",
		}).AddRow(
			1, "scraper-1", "script1", "bash", "0 0 * * *", nil, nil, nil, 1640995200, 1640995200, true, "{}", "[]", nil,
		).AddRow(
			2, "scraper-2", "script2", "python", "0 0 * * *", nil, nil, nil, 1640995200, 1640995200, false, "{}", "[]", nil,
		))

	result, err := ListScraperScripts(false)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "scraper-1", result[0].Name)
	assert.Equal(t, "scraper-2", result[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListScraperScripts_EnabledOnly(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts WHERE enabled = 1 ORDER BY created_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "script", "language", "schedule", "last_run", "last_run_output", "last_run_error", "created_at", "updated_at", "enabled", "variables", "packages", "shared_script",
		}).AddRow(
			1, "scraper-1", "script1", "bash", "0 0 * * *", nil, nil, nil, 1640995200, 1640995200, true, "{}", "[]", nil,
		))

	result, err := ListScraperScripts(true)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "scraper-1", result[0].Name)
	assert.True(t, result[0].Enabled)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateScraperScript(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)
	name := "updated-scraper"
	script := "echo 'updated'"
	language := "python"
	schedule := "0 */2 * * *"
	variables := map[string]string{"new_key": "new_value"}
	packages := []string{"pandas", "numpy"}
	sharedScript := "source /updated/setup.sh"

	mock.ExpectExec(`UPDATE scraper_scripts SET name = \?, script = \?, language = \?, schedule = \?, variables = \?, packages = \?, shared_script = \?, updated_at = \? WHERE id = \?`).
		WithArgs(name, script, language, schedule, `{"new_key":"new_value"}`, `["pandas","numpy"]`, sharedScript, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the SELECT query for the return
	mock.ExpectQuery(`SELECT id, name, script, language, schedule, last_run, last_run_output, last_run_error, created_at, updated_at, enabled, variables, packages, shared_script FROM scraper_scripts WHERE id = \?`).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "script", "language", "schedule", "last_run", "last_run_output", "last_run_error", "created_at", "updated_at", "enabled", "variables", "packages", "shared_script",
		}).AddRow(
			id, name, script, language, schedule, nil, nil, nil, 1640995200, 1640995300, true, `{"new_key":"new_value"}`, `["pandas","numpy"]`, sharedScript,
		))

	result, err := UpdateScraperScript(id, name, script, language, schedule, variables, packages, &sharedScript)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, name, result.Name)
	assert.Equal(t, script, result.Script)
	assert.Equal(t, language, result.Language)
	assert.Equal(t, schedule, result.Schedule)
	assert.Equal(t, variables, result.Variables)
	assert.Equal(t, packages, result.Packages)
	assert.Equal(t, &sharedScript, result.SharedScript)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateScraperScriptLastRun(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)
	output := "Script executed successfully"
	errMsg := ""

	mock.ExpectExec(`UPDATE scraper_scripts SET last_run = \?, last_run_output = \?, last_run_error = \?, updated_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), output, errMsg, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateScraperScriptLastRun(id, output, errMsg)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteScraperScript(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)

	mock.ExpectExec(`DELETE FROM scraper_scripts WHERE id = \?`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteScraperScript(id)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableScraperScript(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)
	enabled := false

	mock.ExpectExec(`UPDATE scraper_scripts SET enabled = \?, updated_at = \? WHERE id = \?`).
		WithArgs(enabled, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = EnableScraperScript(id, enabled)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateScraperLog(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	scriptID := int64(1)
	status := "running"

	mock.ExpectExec(`INSERT INTO scraper_execution_logs \(script_id, status, start_time, created_at\)`).
		WithArgs(scriptID, status, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock CountScraperLogs call from CleanupOldScraperLogs (called in CreateScraperLog)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM scraper_execution_logs WHERE script_id = \?`).
		WithArgs(scriptID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5)) // Less than 15, no cleanup

	result, err := CreateScraperLog(scriptID, status)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, scriptID, result.ScriptID)
	assert.Equal(t, status, result.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetScraperLog(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, script_id, status, output, error_message, start_time, end_time, duration_ms, created_at FROM scraper_execution_logs WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "script_id", "status", "output", "error_message", "start_time", "end_time", "duration_ms", "created_at",
		}).AddRow(
			1, 1, "completed", "output", nil, 1640995200, 1640995300, 10000, 1640995200,
		))

	result, err := GetScraperLog(1)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, int64(1), result.ScriptID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "output", *result.Output)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListScraperLogs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	scriptID := int64(1)
	limit := 10
	offset := 0

	mock.ExpectQuery(`SELECT id, script_id, status, output, error_message, start_time, end_time, duration_ms, created_at FROM scraper_execution_logs WHERE script_id = \? ORDER BY start_time DESC LIMIT \? OFFSET \?`).
		WithArgs(scriptID, limit, offset).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "script_id", "status", "output", "error_message", "start_time", "end_time", "duration_ms", "created_at",
		}).AddRow(
			1, 1, "completed", "output1", nil, 1640995200, 1640995300, 10000, 1640995200,
		).AddRow(
			2, 1, "failed", "output2", "error", 1640995100, 1640995150, 5000, 1640995100,
		))

	result, err := ListScraperLogs(scriptID, limit, offset)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, "completed", result[0].Status)
	assert.Equal(t, int64(2), result[1].ID)
	assert.Equal(t, "failed", result[1].Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteScraperLog(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)

	mock.ExpectExec(`DELETE FROM scraper_execution_logs WHERE id = \?`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteScraperLog(id)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCountScraperLogs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	scriptID := int64(1)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM scraper_execution_logs WHERE script_id = \?`).
		WithArgs(scriptID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	result, err := CountScraperLogs(scriptID)
	assert.NoError(t, err)
	assert.Equal(t, 5, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateScraperLog(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)
	status := "completed"
	output := "success output"
	errorMsg := ""

	mock.ExpectExec(`UPDATE scraper_execution_logs SET status = \?, output = \?, error_message = \?, end_time = \?, duration_ms = \? WHERE id = \?`).
		WithArgs(status, output, errorMsg, sqlmock.AnyArg(), sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateScraperLog(id, status, output, errorMsg)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateScraperLogFinal(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	id := int64(1)
	status := "failed"
	output := "error output"
	errorMsg := "script failed"
	durationMs := int64(15000)

	mock.ExpectExec(`UPDATE scraper_execution_logs SET status = \?, output = \?, error_message = \?, end_time = \?, duration_ms = \? WHERE id = \?`).
		WithArgs(status, output, errorMsg, sqlmock.AnyArg(), durationMs, id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateScraperLogFinal(id, status, output, errorMsg, durationMs)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupOldScraperLogs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	scriptID := int64(1)

	// Mock the CountScraperLogs call first
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM scraper_execution_logs WHERE script_id = \?`).
		WithArgs(scriptID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20)) // More than 15 to trigger cleanup

	mock.ExpectExec(`DELETE FROM scraper_execution_logs WHERE script_id = \? AND id NOT IN`).
		WithArgs(scriptID, scriptID).
		WillReturnResult(sqlmock.NewResult(0, 5))

	err = CleanupOldScraperLogs(scriptID)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateScript(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		language string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid bash script",
			script:   "echo hello",
			language: "bash",
			wantErr:  false,
		},
		{
			name:     "valid python script",
			script:   "print('hello')",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "empty script",
			script:   "",
			language: "bash",
			wantErr:  true,
			errMsg:   "script cannot be empty",
		},
		{
			name:     "whitespace only script",
			script:   "   \n\t   ",
			language: "bash",
			wantErr:  true,
			errMsg:   "script cannot be empty",
		},
		{
			name:     "invalid language",
			script:   "echo hello",
			language: "javascript",
			wantErr:  true,
			errMsg:   "invalid language: javascript, must be 'bash' or 'python'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScript(tt.script, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}