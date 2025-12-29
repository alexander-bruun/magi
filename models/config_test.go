package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestContentRatingToInt(t *testing.T) {
	tests := []struct {
		rating   string
		expected int
	}{
		{"safe", 0},
		{"suggestive", 1},
		{"erotica", 2},
		{"pornographic", 3},
		{"unknown", 3}, // defaults to highest
		{"", 3},
	}

	for _, tt := range tests {
		t.Run(tt.rating, func(t *testing.T) {
			result := ContentRatingToInt(tt.rating)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsContentRatingAllowed(t *testing.T) {
	tests := []struct {
		rating   string
		limit    int
		expected bool
	}{
		{"safe", 0, true},
		{"safe", 1, true},
		{"safe", 2, true},
		{"safe", 3, true},
		{"suggestive", 0, false},
		{"suggestive", 1, true},
		{"suggestive", 2, true},
		{"erotica", 0, false},
		{"erotica", 1, false},
		{"erotica", 2, true},
		{"pornographic", 0, false},
		{"pornographic", 1, false},
		{"pornographic", 2, false},
		{"pornographic", 3, true},
		{"unknown", 0, false},
		{"unknown", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.rating, func(t *testing.T) {
			result := IsContentRatingAllowed(tt.rating, tt.limit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppConfig_GetMetadataProvider(t *testing.T) {
	config := AppConfig{MetadataProvider: "mangadex"}
	assert.Equal(t, "mangadex", config.GetMetadataProvider())
}

func TestAppConfig_GetMALApiToken(t *testing.T) {
	config := AppConfig{MALApiToken: "test-token"}
	assert.Equal(t, "test-token", config.GetMALApiToken())
}

func TestAppConfig_GetAniListApiToken(t *testing.T) {
	config := AppConfig{AniListApiToken: "anilist-token"}
	assert.Equal(t, "anilist-token", config.GetAniListApiToken())
}

func TestAppConfig_GetContentRatingLimit(t *testing.T) {
	config := AppConfig{ContentRatingLimit: 2}
	assert.Equal(t, 2, config.GetContentRatingLimit())
}

func TestLoadConfigFromDB(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock successful query with all fields
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 100, 2, "mangadex", "mal-token", "anilist-token", "secret-key",
			1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := loadConfigFromDB()
	assert.NoError(t, err)
	assert.Equal(t, true, config.AllowRegistration)
	assert.Equal(t, int64(100), config.MaxUsers)
	assert.Equal(t, 2, config.ContentRatingLimit)
	assert.Equal(t, "mangadex", config.MetadataProvider)
	assert.Equal(t, "mal-token", config.MALApiToken)
	assert.Equal(t, "anilist-token", config.AniListApiToken)
	assert.Equal(t, true, config.RateLimitEnabled)
	assert.Equal(t, 100, config.RateLimitRequests)
	assert.Equal(t, 60, config.RateLimitWindow)
	assert.Equal(t, true, config.BotDetectionEnabled)
	assert.Equal(t, 5, config.BotSeriesThreshold)
	assert.Equal(t, 10, config.BotChapterThreshold)
	assert.Equal(t, 60, config.BotDetectionWindow)
	assert.Equal(t, 70, config.ReaderCompressionQuality)
	assert.Equal(t, 85, config.ModeratorCompressionQuality)
	assert.Equal(t, 100, config.AdminCompressionQuality)
	assert.Equal(t, 90, config.PremiumCompressionQuality)
	assert.Equal(t, 70, config.AnonymousCompressionQuality)
	assert.Equal(t, 85, config.ProcessedImageQuality)
	assert.Equal(t, 5, config.ImageTokenValidityMinutes)
	assert.Equal(t, 3600, config.PremiumEarlyAccessDuration)
	assert.Equal(t, 3, config.MaxPremiumChapters)
	assert.Equal(t, false, config.PremiumCooldownScalingEnabled)
	assert.Equal(t, false, config.MaintenanceEnabled)
	assert.Equal(t, `We are currently performing maintenance. Please check back later.`, config.MaintenanceMessage)
	assert.Equal(t, "secret-key", config.ImageAccessSecret)
	assert.Equal(t, true, config.StripeEnabled)
	assert.Equal(t, "pk_test", config.StripePublishableKey)
	assert.Equal(t, "sk_test", config.StripeSecretKey)
	assert.Equal(t, "whsec_test", config.StripeWebhookSecret)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadConfigFromDB_NoRows(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock no rows returned
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrNoRows)

	config, err := loadConfigFromDB()
	assert.NoError(t, err)
	// Should return default config
	assert.Equal(t, true, config.AllowRegistration)
	assert.Equal(t, int64(0), config.MaxUsers)
	assert.Equal(t, 3, config.ContentRatingLimit)
	assert.Equal(t, "mangadex", config.MetadataProvider)
	assert.Equal(t, "", config.MALApiToken)
	assert.Equal(t, "", config.AniListApiToken)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadConfigFromDB_QueryError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query error
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrConnDone)

	_, err = loadConfigFromDB()
	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAppConfig_CacheMiss(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock successful query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			0, 50, 1, "mal", "mal-token", "anilist-token", "secret-key",
			1, "pk_test", "sk_test", "whsec_test",
			0, 200, 120, 0, 10,
			20, 120, 80, 90,
			100, 95, 75, 90,
			10, 7200, 5, 1,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := GetAppConfig()
	assert.NoError(t, err)
	assert.Equal(t, false, config.AllowRegistration)
	assert.Equal(t, int64(50), config.MaxUsers)
	assert.Equal(t, 1, config.ContentRatingLimit)
	assert.Equal(t, "mal", config.MetadataProvider)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAppConfig_CacheHit(t *testing.T) {
	// Set up cache with known values
	configMu.Lock()
	cachedConfig = AppConfig{
		AllowRegistration:  true,
		MaxUsers:           25,
		ContentRatingLimit: 0,
		MetadataProvider:   "jikan",
	}
	configCacheTime = time.Now()
	configMu.Unlock()

	// This should return cached config without hitting DB
	config, err := GetAppConfig()
	assert.NoError(t, err)
	assert.Equal(t, true, config.AllowRegistration)
	assert.Equal(t, int64(25), config.MaxUsers)
	assert.Equal(t, 0, config.ContentRatingLimit)
	assert.Equal(t, "jikan", config.MetadataProvider)
}

func TestGetAppConfig_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock query error
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrConnDone)

	_, err = GetAppConfig()
	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshAppConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock successful query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 75, 2, "anilist", "mal-token-2", "anilist-token-2", "secret-key",
			1, "pk_test", "sk_test", "whsec_test",
			1, 150, 90, 1, 8,
			15, 90, 75, 88,
			100, 92, 72, 88,
			8, 5400, 4, 1,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := RefreshAppConfig()
	assert.NoError(t, err)
	assert.Equal(t, true, config.AllowRegistration)
	assert.Equal(t, int64(75), config.MaxUsers)
	assert.Equal(t, 2, config.ContentRatingLimit)
	assert.Equal(t, "anilist", config.MetadataProvider)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAppConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET allow_registration = \?, max_users = \?, content_rating_limit = \? WHERE id = 1`).
		WithArgs(1, int64(200), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 200, 1, "mangadex", "", "", "secret-key",
			1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateAppConfig(true, 200, 1)
	assert.NoError(t, err)
	assert.Equal(t, true, config.AllowRegistration)
	assert.Equal(t, int64(200), config.MaxUsers)
	assert.Equal(t, 1, config.ContentRatingLimit)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAppConfig_ContentRatingBounds(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test lower bound (should be clamped to 0)
	mock.ExpectExec(`UPDATE app_config SET allow_registration = \?, max_users = \?, content_rating_limit = \? WHERE id = 1`).
		WithArgs(0, int64(0), 0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			0, 0, 0, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateAppConfig(false, 0, -1) // Should be clamped to 0
	assert.NoError(t, err)

	// Test upper bound (should be clamped to 3)
	mock.ExpectExec(`UPDATE app_config SET allow_registration = \?, max_users = \?, content_rating_limit = \? WHERE id = 1`).
		WithArgs(0, int64(0), 3).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			0, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateAppConfig(false, 0, 5) // Should be clamped to 3
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateRateLimitConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET rate_limit_enabled = \?, rate_limit_requests = \?, rate_limit_window = \? WHERE id = 1`).
		WithArgs(0, 250, 180).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			0, 250, 180, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateRateLimitConfig(false, 250, 180)
	assert.NoError(t, err)
	assert.Equal(t, false, config.RateLimitEnabled)
	assert.Equal(t, 250, config.RateLimitRequests)
	assert.Equal(t, 180, config.RateLimitWindow)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCompressionConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET reader_compression_quality = \?, moderator_compression_quality = \?, admin_compression_quality = \?, premium_compression_quality = \?, anonymous_compression_quality = \?, processed_image_quality = \? WHERE id = 1`).
		WithArgs(75, 88, 100, 95, 72, 90).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 75, 88,
			100, 95, 72, 90,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateCompressionConfig(75, 88, 100, 95, 72, 90)
	assert.NoError(t, err)
	assert.Equal(t, 75, config.ReaderCompressionQuality)
	assert.Equal(t, 88, config.ModeratorCompressionQuality)
	assert.Equal(t, 100, config.AdminCompressionQuality)
	assert.Equal(t, 95, config.PremiumCompressionQuality)
	assert.Equal(t, 72, config.AnonymousCompressionQuality)
	assert.Equal(t, 90, config.ProcessedImageQuality)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCompressionConfig_Bounds(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test bounds clamping (negative values should become 0, values > 100 should become 100)
	mock.ExpectExec(`UPDATE app_config SET reader_compression_quality = \?, moderator_compression_quality = \?, admin_compression_quality = \?, premium_compression_quality = \?, anonymous_compression_quality = \?, processed_image_quality = \? WHERE id = 1`).
		WithArgs(0, 0, 100, 100, 0, 100).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 0, 0,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateCompressionConfig(-10, -5, 150, 120, -1, 110) // Should be clamped to valid ranges
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePremiumEarlyAccessConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET premium_early_access_duration = \? WHERE id = 1`).
		WithArgs(7200).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 7200, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdatePremiumEarlyAccessConfig(7200)
	assert.NoError(t, err)
	assert.Equal(t, 7200, config.PremiumEarlyAccessDuration)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePremiumEarlyAccessConfig_Negative(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test negative value clamping to 0
	mock.ExpectExec(`UPDATE app_config SET premium_early_access_duration = \? WHERE id = 1`).
		WithArgs(0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 0, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdatePremiumEarlyAccessConfig(-100) // Should be clamped to 0
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMaxPremiumChaptersConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET max_premium_chapters = \? WHERE id = 1`).
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 10, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))
	config, err := UpdateMaxPremiumChaptersConfig(10)
	assert.NoError(t, err)
	assert.Equal(t, 10, config.MaxPremiumChapters)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMaxPremiumChaptersConfig_Negative(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test negative value clamping to 0
	mock.ExpectExec(`UPDATE app_config SET max_premium_chapters = \? WHERE id = 1`).
		WithArgs(0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 0, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))
	_, err = UpdateMaxPremiumChaptersConfig(-5) // Should be clamped to 0
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePremiumCooldownScalingConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET premium_cooldown_scaling_enabled = \? WHERE id = 1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 1,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdatePremiumCooldownScalingConfig(true)
	assert.NoError(t, err)
	assert.Equal(t, true, config.PremiumCooldownScalingEnabled)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMetadataConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET metadata_provider = \?, mal_api_token = \?, anilist_api_token = \? WHERE id = 1`).
		WithArgs("mal", "new-mal-token", "new-anilist-token").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mal", "new-mal-token", "new-anilist-token", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateMetadataConfig("mal", "new-mal-token", "new-anilist-token")
	assert.NoError(t, err)
	assert.Equal(t, "mal", config.MetadataProvider)
	assert.Equal(t, "new-mal-token", config.MALApiToken)
	assert.Equal(t, "new-anilist-token", config.AniListApiToken)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMetadataConfig_InvalidProvider(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test invalid provider defaults to mangadex
	mock.ExpectExec(`UPDATE app_config SET metadata_provider = \?, mal_api_token = \?, anilist_api_token = \? WHERE id = 1`).
		WithArgs("mangadex", "token", "token2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "token", "token2", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateMetadataConfig("invalid-provider", "token", "token2") // Should default to mangadex
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateBotDetectionConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET bot_detection_enabled = \?, bot_series_threshold = \?, bot_chapter_threshold = \?, bot_detection_window = \? WHERE id = 1`).
		WithArgs(0, 15, 25, 120).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 0, 15,
			25, 120, 70, 85,
			100, 90, 70, 85,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateBotDetectionConfig(false, 15, 25, 120)
	assert.NoError(t, err)
	assert.Equal(t, false, config.BotDetectionEnabled)
	assert.Equal(t, 15, config.BotSeriesThreshold)
	assert.Equal(t, 25, config.BotChapterThreshold)
	assert.Equal(t, 120, config.BotDetectionWindow)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateImageTokenConfig(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE app_config SET image_token_validity_minutes = \? WHERE id = 1`).
		WithArgs(30).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock the refresh query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			30, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	config, err := UpdateImageTokenConfig(30)
	assert.NoError(t, err)
	assert.Equal(t, 30, config.ImageTokenValidityMinutes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateImageTokenConfig_Bounds(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test lower bound (should be clamped to 1)
	mock.ExpectExec(`UPDATE app_config SET image_token_validity_minutes = \? WHERE id = 1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			1, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateImageTokenConfig(0) // Should be clamped to 1
	assert.NoError(t, err)

	// Test upper bound (should be clamped to 60)
	mock.ExpectExec(`UPDATE app_config SET image_token_validity_minutes = \? WHERE id = 1`).
		WithArgs(60).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			60, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	_, err = UpdateImageTokenConfig(70) // Should be clamped to 60
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCompressionQualityForRole(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock successful query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 75, 88,
			95, 92, 78, 87,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	tests := []struct {
		role     string
		expected int
	}{
		{"admin", 95},
		{"moderator", 88},
		{"premium", 92},
		{"anonymous", 78},
		{"reader", 75},
		{"unknown", 75}, // defaults to reader
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := GetCompressionQualityForRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCompressionQualityForRole_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock query error
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrConnDone)

	// Should return default values when DB fails
	tests := []struct {
		role     string
		expected int
	}{
		{"admin", 100},
		{"moderator", 85},
		{"premium", 90},
		{"anonymous", 70},
		{"reader", 70},
		{"unknown", 70},
	}

	for _, tt := range tests {
		t.Run(tt.role+"_db_error", func(t *testing.T) {
			result := GetCompressionQualityForRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetImageTokenValidityMinutes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock successful query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 85,
			15, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	result := GetImageTokenValidityMinutes()
	assert.Equal(t, 15, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetImageTokenValidityMinutes_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock query error
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrConnDone)

	// Should return default value when DB fails
	result := GetImageTokenValidityMinutes()
	assert.Equal(t, 60, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProcessedImageQuality(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock successful query
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "image_access_secret", "stripe_enabled", "stripe_publishable_key", "stripe_secret_key", "stripe_webhook_secret",
			"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
			"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
			"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
			"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
			"maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 0, 3, "mangadex", "", "", "secret-key", 1, "pk_test", "sk_test", "whsec_test",
			1, 100, 60, 1, 5,
			10, 60, 70, 85,
			100, 90, 70, 88,
			5, 3600, 3, 0,
			0, `We are currently performing maintenance. Please check back later.`, 48))

	result := GetProcessedImageQuality()
	assert.Equal(t, 88, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProcessedImageQuality_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear cache
	configCacheTime = time.Time{}

	// Mock query error
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit,.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrConnDone)

	// Should return default value when DB fails
	result := GetProcessedImageQuality()
	assert.Equal(t, 85, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}
