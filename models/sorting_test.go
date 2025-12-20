package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGenericSortConfigNormalizeSort(t *testing.T) {
	config := GenericSortConfig{
		Allowed: []SortOption{
			{Key: "name", Aliases: []string{"title"}},
			{Key: "created", Aliases: []string{"date"}},
			{Key: "popularity", Aliases: []string{}},
		},
		DefaultKey:   "name",
		DefaultOrder: "asc",
	}

	tests := []struct {
		name        string
		sortBy      string
		order       string
		expectedKey string
		expectedOrd string
	}{
		{
			name:        "valid key and order",
			sortBy:      "name",
			order:       "desc",
			expectedKey: "name",
			expectedOrd: "desc",
		},
		{
			name:        "alias",
			sortBy:      "title",
			order:       "asc",
			expectedKey: "name",
			expectedOrd: "asc",
		},
		{
			name:        "case insensitive",
			sortBy:      "TITLE",
			order:       "DESC",
			expectedKey: "name",
			expectedOrd: "desc",
		},
		{
			name:        "invalid key falls back to default",
			sortBy:      "invalid",
			order:       "asc",
			expectedKey: "name",
			expectedOrd: "asc",
		},
		{
			name:        "invalid order falls back to default",
			sortBy:      "name",
			order:       "invalid",
			expectedKey: "name",
			expectedOrd: "asc",
		},
		{
			name:        "popularity gets desc default",
			sortBy:      "popularity",
			order:       "",
			expectedKey: "popularity",
			expectedOrd: "desc",
		},
		{
			name:        "empty inputs",
			sortBy:      "",
			order:       "",
			expectedKey: "name",
			expectedOrd: "asc",
		},
		{
			name:        "whitespace trimmed",
			sortBy:      "  name  ",
			order:       "  desc  ",
			expectedKey: "name",
			expectedOrd: "desc",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, ord := config.NormalizeSort(test.sortBy, test.order)
			assert.Equal(t, test.expectedKey, key)
			assert.Equal(t, test.expectedOrd, ord)
		})
	}
}

func TestSortMedias(t *testing.T) {
	// Create test media
	media := []Media{
		{Name: "Z Media", Type: "manga", Year: 2020, Status: "completed", ContentRating: "safe", ReadCount: 100, VoteScore: 85},
		{Name: "A Media", Type: "manhwa", Year: 2019, Status: "ongoing", ContentRating: "suggestive", ReadCount: 50, VoteScore: 90},
		{Name: "M Media", Type: "manhua", Year: 2021, Status: "hiatus", ContentRating: "erotica", ReadCount: 200, VoteScore: 70},
	}

	tests := []struct {
		name     string
		key      string
		order    string
		expected []string // expected order of names
	}{
		{
			name:     "sort by name ascending",
			key:      "name",
			order:    "asc",
			expected: []string{"A Media", "M Media", "Z Media"},
		},
		{
			name:     "sort by name descending",
			key:      "name",
			order:    "desc",
			expected: []string{"Z Media", "M Media", "A Media"},
		},
		{
			name:     "sort by type ascending",
			key:      "type",
			order:    "asc",
			expected: []string{"Z Media", "M Media", "A Media"}, // manga, manhua, manhwa
		},
		{
			name:     "sort by year descending",
			key:      "year",
			order:    "desc",
			expected: []string{"M Media", "Z Media", "A Media"}, // 2021, 2020, 2019
		},
		{
			name:     "sort by read_count descending",
			key:      "read_count",
			order:    "desc",
			expected: []string{"M Media", "Z Media", "A Media"}, // 200, 100, 50
		},
		{
			name:     "sort by popularity descending",
			key:      "popularity",
			order:    "desc",
			expected: []string{"A Media", "Z Media", "M Media"}, // 9.0, 8.5, 7.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of media for each test
			mediaCopy := make([]Media, len(media))
			copy(mediaCopy, media)

			// Sort
			SortMedias(mediaCopy, tt.key, tt.order)

			// Check order
			actualNames := make([]string, len(mediaCopy))
			for i, m := range mediaCopy {
				actualNames[i] = m.Name
			}
			assert.Equal(t, tt.expected, actualNames)
		})
	}
}

func TestGetAllowedMediaSortOptions(t *testing.T) {
	tests := []struct {
		name             string
		contentRatingLimit int
		expectedKeys     []string
	}{
		{
			name:             "content rating allowed (limit >= 3)",
			contentRatingLimit: 3,
			expectedKeys:     []string{"name", "type", "year", "status", "content_rating", "created_at", "updated_at", "read_count", "popularity"},
		},
		{
			name:             "content rating filtered (limit < 3)",
			contentRatingLimit: 2,
			expectedKeys:     []string{"name", "type", "year", "status", "content_rating", "created_at", "updated_at", "read_count", "popularity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock GetAppConfig
			mockDB, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer mockDB.Close()

			originalDB := db
			db = mockDB
			defer func() { db = originalDB }()

			// Clear cache
			configCacheTime = time.Time{}
			cachedConfig = AppConfig{}

			// Mock the config query
			mock.ExpectQuery(`(?s).*SELECT allow_registration, max_users, content_rating_limit.*FROM app_config WHERE id = 1`).
				WillReturnRows(sqlmock.NewRows([]string{
					"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token",
					"rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold",
					"bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality",
					"admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality",
					"image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
					"new_badge_duration", "maintenance_enabled", "maintenance_message",
				}).AddRow(
					0, 50, tt.contentRatingLimit, "mal", "mal-token", "anilist-token",
					0, 200, 120, 0, 10,
					20, 120, 80, 90,
					100, 95, 75, 90,
					10, 7200, 5, 1, 48, 0, "We are currently performing maintenance. Please check back later.",
				))

			options := GetAllowedMediaSortOptions()
			assert.Len(t, options, len(tt.expectedKeys))
			for i, opt := range options {
				assert.Equal(t, tt.expectedKeys[i], opt.Key)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}