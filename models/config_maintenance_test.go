package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateMaintenanceConfig tests the UpdateMaintenanceConfig function
func TestUpdateMaintenanceConfig(t *testing.T) {
	tests := []struct {
		name              string
		enabled           bool
		message           string
		expectedEnabled   bool
		expectedMessage   string
		shouldError       bool
	}{
		{
			name:            "Enable maintenance with custom message",
			enabled:         true,
			message:         "System upgrade in progress. Please try again in 30 minutes.",
			expectedEnabled: true,
			expectedMessage: "System upgrade in progress. Please try again in 30 minutes.",
			shouldError:     false,
		},
		{
			name:            "Enable maintenance with empty message",
			enabled:         true,
			message:         "",
			expectedEnabled: true,
			expectedMessage: "",
			shouldError:     false,
		},
		{
			name:            "Enable maintenance with HTML message",
			enabled:         true,
			message:         "<h2>Maintenance</h2><p>We are updating the system.</p>",
			expectedEnabled: true,
			expectedMessage: "<h2>Maintenance</h2><p>We are updating the system.</p>",
			shouldError:     false,
		},
		{
			name:            "Enable maintenance with Markdown message",
			enabled:         true,
			message:         "## Maintenance\n\nWe are updating the system. **Back in 1 hour.**",
			expectedEnabled: true,
			expectedMessage: "## Maintenance\n\nWe are updating the system. **Back in 1 hour.**",
			shouldError:     false,
		},
		{
			name:            "Disable maintenance",
			enabled:         false,
			message:         "Some message",
			expectedEnabled: false,
			expectedMessage: "Some message",
			shouldError:     false,
		},
		{
			name:            "Message with special characters",
			enabled:         true,
			message:         "Maintenance: 10:00 AM - 2:00 PM (Est.) Contact: support@example.com",
			expectedEnabled: true,
			expectedMessage: "Maintenance: 10:00 AM - 2:00 PM (Est.) Contact: support@example.com",
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			mockDB, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer mockDB.Close()

			// Replace global db
			originalDB := db
			db = mockDB
			defer func() { db = originalDB }()

			// Mock the UPDATE query
			enabledInt := 0
			if tt.enabled {
				enabledInt = 1
			}
			mock.ExpectExec(`UPDATE app_config SET maintenance_enabled = \?, maintenance_message = \? WHERE id = 1`).
				WithArgs(enabledInt, tt.message).
				WillReturnResult(sqlmock.NewResult(1, 1))

			// Mock the SELECT query for GetAppConfig
			mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,`).
				WillReturnRows(sqlmock.NewRows([]string{
					"allow_registration", "max_users", "content_rating_limit", "metadata_provider",
					"mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests",
					"rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold",
					"bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
					"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality",
					"processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration",
					"max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
				}).AddRow(
					1, int64(0), 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60,
					70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, enabledInt, tt.message,
				))

			// Update maintenance config
			cfg, err := UpdateMaintenanceConfig(tt.enabled, tt.message)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedEnabled, cfg.MaintenanceEnabled)
			assert.Equal(t, tt.expectedMessage, cfg.MaintenanceMessage)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetMaintenanceStatus tests the GetMaintenanceStatus function
func TestGetMaintenanceStatus(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock the SELECT query for GetAppConfig when enabled=true
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider",
			"mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests",
			"rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold",
			"bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality",
			"processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration",
			"max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
		}).AddRow(
			1, int64(0), 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60,
			70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 1, "Test maintenance message",
		))

	// Get status
	enabled, message, err := GetMaintenanceStatus()
	require.NoError(t, err)
	assert.True(t, enabled)
	assert.Equal(t, "Test maintenance message", message)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetMaintenanceStatusDisabled tests getting status when maintenance is disabled
func TestGetMaintenanceStatusDisabled(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock the SELECT query for GetAppConfig when enabled=false
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider",
			"mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests",
			"rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold",
			"bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality",
			"processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration",
			"max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
		}).AddRow(
			1, int64(0), 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60,
			70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 0, "Default message",
		))

	// Get status
	enabled, message, err := GetMaintenanceStatus()
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.Equal(t, "Default message", message)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMaintenanceMessageWithSpecialChars tests message containing quotes and special chars
func TestMaintenanceMessageWithSpecialChars(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	testMessage := `We're performing "critical" updates. Don't worry, we'll be back soon. 100% uptime!`

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET maintenance_enabled = \?, maintenance_message = \? WHERE id = 1`).
		WithArgs(1, testMessage).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock the SELECT query for GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider",
			"mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests",
			"rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold",
			"bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality",
			"processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration",
			"max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
		}).AddRow(
			1, int64(0), 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60,
			70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 1, testMessage,
		))

	cfg, err := UpdateMaintenanceConfig(true, testMessage)
	require.NoError(t, err)
	assert.Equal(t, testMessage, cfg.MaintenanceMessage)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMaintenanceMessageEmpty tests handling of empty messages
func TestMaintenanceMessageEmpty(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET maintenance_enabled = \?, maintenance_message = \? WHERE id = 1`).
		WithArgs(1, "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock the SELECT query for GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider",
			"mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests",
			"rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold",
			"bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality",
			"processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration",
			"max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
		}).AddRow(
			1, int64(0), 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60,
			70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 1, "",
		))

	// Set empty message
	cfg, err := UpdateMaintenanceConfig(true, "")
	require.NoError(t, err)
	assert.Equal(t, "", cfg.MaintenanceMessage)
	assert.NoError(t, mock.ExpectationsWereMet())
}

