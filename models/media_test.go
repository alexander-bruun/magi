package models

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeTagSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]struct{}
	}{
		{
			name:     "empty slice returns empty map",
			input:    []string{},
			expected: map[string]struct{}{},
		},
		{
			name:  "normalizes tags correctly",
			input: []string{"Tag1", " TAG2 ", "tag3", ""},
			expected: map[string]struct{}{
				"tag1": {},
				"tag2": {},
				"tag3": {},
			},
		},
		{
			name:  "removes empty strings after trimming",
			input: []string{"tag1", "  ", "", "tag2"},
			expected: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
		},
		{
			name:  "handles duplicate tags",
			input: []string{"tag1", "TAG1", " tag1 "},
			expected: map[string]struct{}{
				"tag1": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTagSet(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeStringSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]struct{}
	}{
		{
			name:     "empty slice returns empty map",
			input:    []string{},
			expected: map[string]struct{}{},
		},
		{
			name:  "normalizes strings correctly",
			input: []string{"String1", " STRING2 ", "string3", ""},
			expected: map[string]struct{}{
				"string1": {},
				"string2": {},
				"string3": {},
			},
		},
		{
			name:  "removes empty strings after trimming",
			input: []string{"string1", "  ", "", "string2"},
			expected: map[string]struct{}{
				"string1": {},
				"string2": {},
			},
		},
		{
			name:  "handles duplicate strings",
			input: []string{"string1", "STRING1", " string1 "},
			expected: map[string]struct{}{
				"string1": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeStringSet(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		required map[string]struct{}
		expected bool
	}{
		{
			name:     "empty required set returns true",
			tags:     []string{"tag1", "tag2"},
			required: map[string]struct{}{},
			expected: true,
		},
		{
			name:     "empty tags slice with non-empty required returns false",
			tags:     []string{},
			required: map[string]struct{}{"required": {}},
			expected: false,
		},
		{
			name:  "has all required tags",
			tags:  []string{"tag1", "tag2", "tag3"},
			required: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
			expected: true,
		},
		{
			name:  "missing one required tag",
			tags:  []string{"tag1", "tag3"},
			required: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
			expected: false,
		},
		{
			name:  "case insensitive matching",
			tags:  []string{"Tag1", "TAG2"},
			required: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
			expected: true,
		},
		{
			name:  "ignores whitespace in tags",
			tags:  []string{" tag1 ", " tag2 "},
			required: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllTags(tt.tags, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAnyTag(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		anySet   map[string]struct{}
		expected bool
	}{
		{
			name:     "empty anySet returns true",
			tags:     []string{"tag1", "tag2"},
			anySet:   map[string]struct{}{},
			expected: true,
		},
		{
			name:   "has at least one matching tag",
			tags:   []string{"tag1", "tag2", "tag3"},
			anySet: map[string]struct{}{"tag2": {}, "tag4": {}},
			expected: true,
		},
		{
			name:   "no matching tags",
			tags:   []string{"tag1", "tag2"},
			anySet: map[string]struct{}{"tag3": {}, "tag4": {}},
			expected: false,
		},
		{
			name:   "case insensitive matching",
			tags:   []string{"Tag1", "TAG2"},
			anySet: map[string]struct{}{"tag1": {}, "tag3": {}},
			expected: true,
		},
		{
			name:   "ignores whitespace in tags",
			tags:   []string{" tag1 ", " tag2 "},
			anySet: map[string]struct{}{"tag1": {}, "tag3": {}},
			expected: true,
		},
		{
			name:   "ignores empty strings",
			tags:   []string{"", "  ", "tag1"},
			anySet: map[string]struct{}{"tag1": {}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnyTag(tt.tags, tt.anySet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	media := Media{
		Slug:             "test-manga",
		Name:             "Test Manga",
		Author:           "Test Author",
		Description:      "Test Description",
		Year:             2023,
		OriginalLanguage: "ja",
		Type:             "manga",
		Status:           "ongoing",
		ContentRating:    "safe",
		LibrarySlug:      "test-lib",
		CoverArtURL:      "/covers/test.jpg",
		Path:             "/path/to/test",
		FileCount:        10,
	}

	// Mock MediaExists check - media doesn't exist
	mock.ExpectQuery(`SELECT 1 FROM media WHERE slug = \?`).
		WithArgs("test-manga").
		WillReturnError(sql.ErrNoRows)

	// Mock the INSERT query
	mock.ExpectExec(`INSERT INTO media`).
		WithArgs(
			"test-manga", "Test Manga", "Test Author", "Test Description", 2023,
			"ja", "manga", "ongoing", "safe", "test-lib", "/covers/test.jpg",
			"/path/to/test", 10, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateMedia(media)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateMediaAlreadyExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	media := Media{
		Slug: "existing-manga",
		Name: "Existing Manga",
	}

	// Mock MediaExists check - media already exists
	mock.ExpectQuery(`SELECT 1 FROM media WHERE slug = \?`).
		WithArgs("existing-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	err = CreateMedia(media)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "media already exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock the media query
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"test-manga", "Test Manga", "Test Author", "Test Description", 2023, "ja", "manga", "ongoing", "safe", "test-lib", "/covers/test.jpg", "/path/to/test", 10, 1609459200, 1704067200,
		))

	// Mock config query to fail (so it defaults to allowing content)
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnError(sql.ErrNoRows)

	// Mock GetTagsForMedia
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"tag"}).AddRow("action").AddRow("shonen"))

	media, err := GetMedia("test-manga")
	assert.NoError(t, err)
	assert.NotNil(t, media)
	assert.Equal(t, "test-manga", media.Slug)
	assert.Equal(t, "Test Manga", media.Name)
	assert.Equal(t, []string{"action", "shonen"}, media.Tags)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the media query - no rows returned
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	media, err := GetMedia("nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, media)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	media := &Media{
		Slug:             "test-manga",
		Name:             "Updated Test Manga",
		Author:           "Updated Author",
		Description:      "Updated Description",
		Year:             2024,
		OriginalLanguage: "ja",
		Type:             "manga",
		Status:           "completed",
		ContentRating:    "safe",
		LibrarySlug:      "test-lib",
		CoverArtURL:      "/covers/updated.jpg",
		Path:             "/path/to/updated",
		FileCount:        15,
	}

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE media`).
		WithArgs(
			"Updated Test Manga", "Updated Author", "Updated Description", 2024,
			"ja", "manga", "completed", "safe", "test-lib", "/covers/updated.jpg",
			"/path/to/updated", 15, sqlmock.AnyArg(), "test-manga",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateMedia(media)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock DeleteChaptersByMediaSlug
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Mock DeleteTagsByMediaSlug
	mock.ExpectExec(`DELETE FROM media_tags WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 3))

	// Mock DeleteRecord for media
	mock.ExpectExec(`DELETE FROM media WHERE slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteMedia("test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaUnfiltered(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	slug := "test-manga"

	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
		WithArgs(slug).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			slug, "Test Manga", "Test Author", "Test Description", 2023, "ja", "manga", "ongoing", "safe", "test-lib", "http://example.com/cover.jpg", "/path/to/media", 10, 1640995200, 1640995300,
		))

	// Mock GetTagsForMedia
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs(slug).
		WillReturnRows(sqlmock.NewRows([]string{"tag"}).AddRow("action").AddRow("fantasy"))

	result, err := GetMediaUnfiltered(slug)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, slug, result.Slug)
	assert.Equal(t, "Test Manga", result.Name)
	assert.Equal(t, "Test Author", result.Author)
	assert.Equal(t, "Test Description", result.Description)
	assert.Equal(t, 2023, result.Year)
	assert.Equal(t, "ja", result.OriginalLanguage)
	assert.Equal(t, "manga", result.Type)
	assert.Equal(t, "ongoing", result.Status)
	assert.Equal(t, "safe", result.ContentRating)
	assert.Equal(t, "test-lib", result.LibrarySlug)
	assert.Equal(t, "http://example.com/cover.jpg", result.CoverArtURL)
	assert.Equal(t, "/path/to/media", result.Path)
	assert.Equal(t, 10, result.FileCount)
	assert.Len(t, result.Tags, 2)
	assert.Contains(t, result.Tags, "action")
	assert.Contains(t, result.Tags, "fantasy")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaBySlugAndLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	slug := "test-manga"
	librarySlug := "test-lib"

	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \? AND library_slug = \?`).
		WithArgs(slug, librarySlug).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			slug, "Test Manga", "Test Author", "Test Description", 2023, "ja", "manga", "ongoing", "safe", librarySlug, "http://example.com/cover.jpg", "/path/to/media", 10, 1640995200, 1640995300,
		))

	// Mock GetTagsForMedia
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs(slug).
		WillReturnRows(sqlmock.NewRows([]string{"tag"}).AddRow("action"))

	result, err := GetMediaBySlugAndLibrary(slug, librarySlug)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, slug, result.Slug)
	assert.Equal(t, librarySlug, result.LibrarySlug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaBySlugAndLibraryFiltered(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	slug := "test-manga"
	librarySlug := "test-lib"

	// Mock the media query (called first)
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \? AND library_slug = \?`).
		WithArgs(slug, librarySlug).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			slug, "Test Manga", "Test Author", "Test Description", 2023, "ja", "manga", "ongoing", "safe", librarySlug, "http://example.com/cover.jpg", "/path/to/media", 10, 1640995200, 1640995300,
		))

	// Mock GetAppConfig for content rating check (called after media query)
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit, COALESCE\(metadata_provider, 'mangadex'\), COALESCE\(mal_api_token, ''\), COALESCE\(anilist_api_token, ''\), COALESCE\(rate_limit_enabled, 1\), COALESCE\(rate_limit_requests, 100\), COALESCE\(rate_limit_window, 60\), COALESCE\(bot_detection_enabled, 1\), COALESCE\(bot_series_threshold, 5\), COALESCE\(bot_chapter_threshold, 10\), COALESCE\(bot_detection_window, 60\), COALESCE\(reader_compression_quality, 70\), COALESCE\(moderator_compression_quality, 85\), COALESCE\(admin_compression_quality, 100\), COALESCE\(premium_compression_quality, 90\), COALESCE\(anonymous_compression_quality, 70\), COALESCE\(processed_image_quality, 85\), COALESCE\(image_token_validity_minutes, 5\), COALESCE\(premium_early_access_duration, 3600\), COALESCE\(max_premium_chapters, 3\), COALESCE\(premium_cooldown_scaling_enabled, 0\), COALESCE\(maintenance_enabled, 0\), COALESCE\(maintenance_message, 'We are currently performing maintenance\. Please check back later\.'\), COALESCE\(new_badge_duration, 48\) FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 100, 0, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 0, "We are currently performing maintenance. Please check back later.", 48,
		))

	// Mock GetTagsForMedia
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs(slug).
		WillReturnRows(sqlmock.NewRows([]string{"tag"}).AddRow("action"))

	result, err := GetMediaBySlugAndLibraryFiltered(slug, librarySlug)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, slug, result.Slug)
	assert.Equal(t, librarySlug, result.LibrarySlug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMediaMetadata(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	media := &Media{
		Slug:             "test-manga",
		Name:             "Updated Manga",
		Author:           "Updated Author",
		Description:      "Updated Description",
		Year:             2024,
		OriginalLanguage: "en",
		Status:           "completed",
		ContentRating:    "safe",
		Type:             "manhwa",
		CoverArtURL:      "http://example.com/new-cover.jpg",
	}

	mock.ExpectExec(`UPDATE media SET name = \?, author = \?, description = \?, year = \?, original_language = \?, type = \?, status = \?, content_rating = \?, cover_art_url = \? WHERE slug = \?`).
		WithArgs(media.Name, media.Author, media.Description, media.Year, media.OriginalLanguage, media.Type, media.Status, media.ContentRating, media.CoverArtURL, media.Slug).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateMediaMetadata(media)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchMediasWithTags(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock loadAllMedias query - complex query with JOINs for read counts and vote scores
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 5, 8, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 3, 7, 1640995200, 1640995300,
		))

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	// Mock GetAllMediaTagsMap
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	result, total, err := SearchMediasWithTags("", 1, 10, "name", "asc", "", "lib1", []string{"action"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
	assert.Equal(t, "manga1", result[0].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMediaExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test exists
	mock.ExpectQuery(`SELECT 1 FROM media WHERE slug = \?`).
		WithArgs("existing-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := MediaExists("existing-manga")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test not exists
	mock.ExpectQuery(`SELECT 1 FROM media WHERE slug = \?`).
		WithArgs("non-existing-manga").
		WillReturnRows(sqlmock.NewRows([]string{}))

	exists, err = MediaExists("non-existing-manga")
	assert.NoError(t, err)
	assert.False(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMediaCount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock loadAllMedias query - complex query with JOINs for read counts and vote scores
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 5, 8, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 3, 7, 1640995200, 1640995300,
		))

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	// Test count all
	count, err := MediaCount("", "")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediasBySlugs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	slugs := []string{"manga1", "manga2"}

	// Mock the query with IN clause
	mock.ExpectQuery(`SELECT.*FROM media WHERE slug IN`).
		WithArgs("manga1", "manga2").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	// Mock GetAllMediaTagsMap
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	result, err := GetMediasBySlugs(slugs)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)
	assert.Equal(t, []string{"action", "fantasy"}, result[0].Tags)
	assert.Equal(t, []string{"romance"}, result[1].Tags)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopMedias(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	limit := 5

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration, max_users, content_rating_limit, COALESCE\(metadata_provider, 'mangadex'\), COALESCE\(mal_api_token, ''\), COALESCE\(anilist_api_token, ''\), COALESCE\(rate_limit_enabled, 1\), COALESCE\(rate_limit_requests, 100\), COALESCE\(rate_limit_window, 60\), COALESCE\(bot_detection_enabled, 1\), COALESCE\(bot_series_threshold, 5\), COALESCE\(bot_chapter_threshold, 10\), COALESCE\(bot_detection_window, 60\), COALESCE\(reader_compression_quality, 70\), COALESCE\(moderator_compression_quality, 85\), COALESCE\(admin_compression_quality, 100\), COALESCE\(premium_compression_quality, 90\), COALESCE\(anonymous_compression_quality, 70\), COALESCE\(processed_image_quality, 85\), COALESCE\(image_token_validity_minutes, 5\), COALESCE\(premium_early_access_duration, 3600\), COALESCE\(max_premium_chapters, 3\), COALESCE\(premium_cooldown_scaling_enabled, 0\), COALESCE\(maintenance_enabled, 0\), COALESCE\(maintenance_message, 'We are currently performing maintenance\. Please check back later\.'\), COALESCE\(new_badge_duration, 48\) FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message", "new_badge_duration",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 0, "We are currently performing maintenance. Please check back later.", 48,
		))

	// Mock the complex query with IN clause for content ratings
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, m\.created_at, m\.updated_at FROM media m LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) v ON v\.media_slug = m\.slug WHERE m\.content_rating IN \(\?\,\?\) ORDER BY v\.score DESC LIMIT \?`).
		WithArgs("safe", "suggestive", limit).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"top-manga1", "Top Manga One", "Top Author One", "Top Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "top1.jpg", "/path/top1", 15, 1640995200, 1640995300,
		).AddRow(
			"top-manga2", "Top Manga Two", "Top Author Two", "Top Description Two", 2022, "en", "manhwa", "completed", "suggestive", "lib1", "top2.jpg", "/path/top2", 12, 1640995200, 1640995300,
		))

	// Mock GetAllMediaTagsMap
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("top-manga1", "action").
			AddRow("top-manga2", "romance"))

	result, err := GetTopMedias(limit)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "top-manga1", result[0].Slug)
	assert.Equal(t, "top-manga2", result[1].Slug)
	assert.Equal(t, []string{"action"}, result[0].Tags)
	assert.Equal(t, []string{"romance"}, result[1].Tags)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchMedias(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock loadAllMedias query (complex query with JOINs)
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 5, 8, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 3, 7, 1640995200, 1640995300,
		))

	// Mock GetAppConfig for content rating check
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	result, total, err := SearchMedias("", 1, 10, "name", "asc", "", "lib1")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchMediasWithAnyTags(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock loadAllMedias query
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 5, 8, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 3, 7, 1640995200, 1640995300,
		).AddRow(
			"manga3", "Manga Three", "Author Three", "Description Three", 2021, "ko", "manhua", "ongoing", "safe", "lib1", "cover3.jpg", "/path3", 12, 6, 9, 1640995200, 1640995300,
		))

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 100, 1, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	// Mock GetAllMediaTagsMap
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance").
			AddRow("manga3", "horror"))

	result, total, err := SearchMediasWithAnyTags("", 1, 10, "name", "asc", "", "lib1", []string{"action", "romance"})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug) // has action
	assert.Equal(t, "manga2", result[1].Slug) // has romance

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediasByLibrarySlug(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	librarySlug := "test-library"

	// Mock the media query
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE library_slug = \?`).
		WithArgs(librarySlug).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", librarySlug, "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", librarySlug, "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAppConfig
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled", "maintenance_enabled", "maintenance_message",
		}).AddRow(
			1, 100, 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0, 0, "We are currently performing maintenance. Please check back later.",
		))

	result, err := GetMediasByLibrarySlug(librarySlug)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "Manga One", result[0].Name)
	assert.Equal(t, librarySlug, result[0].LibrarySlug)
	assert.Equal(t, "manga2", result[1].Slug)
	assert.Equal(t, "Manga Two", result[1].Name)
	assert.Equal(t, librarySlug, result[1].LibrarySlug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllMediaTypes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the DISTINCT query for media types
	mock.ExpectQuery(`SELECT DISTINCT LOWER\(TRIM\(type\)\) FROM media WHERE type IS NOT NULL AND TRIM\(type\) <> ''`).
		WillReturnRows(sqlmock.NewRows([]string{"type"}).
			AddRow("manga").
			AddRow("manhwa").
			AddRow("manhua").
			AddRow("novel"))

	result, err := GetAllMediaTypes()
	assert.NoError(t, err)
	assert.Len(t, result, 4)
	assert.Equal(t, []string{"manga", "manhua", "manhwa", "novel"}, result) // Should be sorted

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaVotes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the complex vote aggregation query
	mock.ExpectQuery(`SELECT.*FROM votes WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"score", "upvotes", "downvotes"}).
			AddRow(7, 7, 3)) // 7 upvotes, 3 downvotes = score of 7 (70% positive)

	score, upvotes, downvotes, err := GetMediaVotes("test-manga")
	assert.NoError(t, err)
	assert.Equal(t, 7, score)
	assert.Equal(t, 7, upvotes)
	assert.Equal(t, 3, downvotes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaVotes_NoVotes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query returning zeros for no votes
	mock.ExpectQuery(`SELECT.*FROM votes WHERE media_slug = \?`).
		WithArgs("no-votes-manga").
		WillReturnRows(sqlmock.NewRows([]string{"score", "upvotes", "downvotes"}).
			AddRow(0, 0, 0))

	score, upvotes, downvotes, err := GetMediaVotes("no-votes-manga")
	assert.NoError(t, err)
	assert.Equal(t, 0, score)
	assert.Equal(t, 0, upvotes)
	assert.Equal(t, 0, downvotes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserVoteForMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning an upvote
	mock.ExpectQuery(`SELECT value FROM votes WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1))

	vote, err := GetUserVoteForMedia("testuser", "test-manga")
	assert.NoError(t, err)
	assert.Equal(t, 1, vote)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserVoteForMedia_NoVote(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning no rows (no vote)
	mock.ExpectQuery(`SELECT value FROM votes WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"value"}))

	vote, err := GetUserVoteForMedia("testuser", "test-manga")
	assert.NoError(t, err)
	assert.Equal(t, 0, vote)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetVote_Update(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock UPDATE that affects 1 row (existing vote)
	mock.ExpectExec(`UPDATE votes SET value = \?, updated_at = \? WHERE user_username = \? AND media_slug = \?`).
		WithArgs(1, sqlmock.AnyArg(), "testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = SetVote("testuser", "test-manga", 1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetVote_Insert(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock UPDATE that affects 0 rows (no existing vote)
	mock.ExpectExec(`UPDATE votes SET value = \?, updated_at = \? WHERE user_username = \? AND media_slug = \?`).
		WithArgs(-1, sqlmock.AnyArg(), "testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock INSERT for new vote
	mock.ExpectExec(`INSERT INTO votes \(user_username, media_slug, value, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("testuser", "test-manga", -1, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SetVote("testuser", "test-manga", -1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetVote_InvalidValue(t *testing.T) {
	// No mocks needed since validation happens before DB calls
	err := SetVote("testuser", "test-manga", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid vote value")
}

func TestRemoveVote(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock DELETE query
	mock.ExpectExec(`DELETE FROM votes WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = RemoveVote("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetFavorite_Update(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock UPDATE that affects 1 row (existing favorite)
	mock.ExpectExec(`UPDATE favorites SET updated_at = \? WHERE user_username = \? AND media_slug = \?`).
		WithArgs(sqlmock.AnyArg(), "testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = SetFavorite("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetFavorite_Insert(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock UPDATE that affects 0 rows (no existing favorite)
	mock.ExpectExec(`UPDATE favorites SET updated_at = \? WHERE user_username = \? AND media_slug = \?`).
		WithArgs(sqlmock.AnyArg(), "testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock INSERT for new favorite
	mock.ExpectExec(`INSERT INTO favorites \(user_username, media_slug, created_at, updated_at\) VALUES \(\?, \?, \?, \?\)`).
		WithArgs("testuser", "test-manga", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SetFavorite("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveFavorite(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock DELETE query
	mock.ExpectExec(`DELETE FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = RemoveFavorite("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsFavoriteForUser_True(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning a row (favorite exists)
	mock.ExpectQuery(`SELECT 1 FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	isFavorite, err := IsFavoriteForUser("testuser", "test-manga")
	assert.NoError(t, err)
	assert.True(t, isFavorite)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsFavoriteForUser_False(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning no rows (no favorite)
	mock.ExpectQuery(`SELECT 1 FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	isFavorite, err := IsFavoriteForUser("testuser", "test-manga")
	assert.NoError(t, err)
	assert.False(t, isFavorite)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleFavorite_Add(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock IsFavoriteForUser returning false
	mock.ExpectQuery(`SELECT 1 FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	// Mock SetFavorite - UPDATE that affects 0 rows, then INSERT
	mock.ExpectExec(`UPDATE favorites SET updated_at = \? WHERE user_username = \? AND media_slug = \?`).
		WithArgs(sqlmock.AnyArg(), "testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`INSERT INTO favorites \(user_username, media_slug, created_at, updated_at\) VALUES \(\?, \?, \?, \?\)`).
		WithArgs("testuser", "test-manga", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = ToggleFavorite("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleFavorite_Remove(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock IsFavoriteForUser returning true
	mock.ExpectQuery(`SELECT 1 FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	// Mock RemoveFavorite
	mock.ExpectExec(`DELETE FROM favorites WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = ToggleFavorite("testuser", "test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFavoritesCount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock COUNT query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM favorites WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	count, err := GetFavoritesCount("test-manga")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFavoritesForUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning favorite slugs
	mock.ExpectQuery(`SELECT media_slug FROM favorites WHERE user_username = \? ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2").
			AddRow("manga3"))

	slugs, err := GetFavoritesForUser("testuser")
	assert.NoError(t, err)
	assert.Len(t, slugs, 3)
	assert.Equal(t, []string{"manga1", "manga2", "manga3"}, slugs)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReadingMediasForUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning reading media slugs
	mock.ExpectQuery(`SELECT DISTINCT media_slug FROM reading_states WHERE user_name = \? ORDER BY created_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	slugs, err := GetReadingMediasForUser("testuser")
	assert.NoError(t, err)
	assert.Len(t, slugs, 2)
	assert.Equal(t, []string{"manga1", "manga2"}, slugs)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUpvotedMediasForUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning upvoted media slugs
	mock.ExpectQuery(`SELECT media_slug FROM votes WHERE user_username = \? AND value = 1 ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga3"))

	slugs, err := GetUpvotedMediasForUser("testuser")
	assert.NoError(t, err)
	assert.Len(t, slugs, 2)
	assert.Equal(t, []string{"manga1", "manga3"}, slugs)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDownvotedMediasForUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock query returning downvoted media slugs
	mock.ExpectQuery(`SELECT media_slug FROM votes WHERE user_username = \? AND value = -1 ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga2"))

	slugs, err := GetDownvotedMediasForUser("testuser")
	assert.NoError(t, err)
	assert.Len(t, slugs, 1)
	assert.Equal(t, []string{"manga2"}, slugs)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteMediasByLibrarySlug(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock SELECT query to get media slugs
	mock.ExpectQuery(`SELECT slug FROM media WHERE library_slug = \?`).
		WithArgs("lib1").
		WillReturnRows(sqlmock.NewRows([]string{"slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	// For each media slug, mock the DeleteMedia operations
	// manga1 operations
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \?`).
		WithArgs("manga1").
		WillReturnResult(sqlmock.NewResult(0, 2)) // 2 chapters deleted

	mock.ExpectExec(`DELETE FROM media_tags WHERE media_slug = \?`).
		WithArgs("manga1").
		WillReturnResult(sqlmock.NewResult(0, 3)) // 3 tags deleted

	mock.ExpectExec(`DELETE FROM media WHERE slug = \?`).
		WithArgs("manga1").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 media deleted

	// manga2 operations
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \?`).
		WithArgs("manga2").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 chapter deleted

	mock.ExpectExec(`DELETE FROM media_tags WHERE media_slug = \?`).
		WithArgs("manga2").
		WillReturnResult(sqlmock.NewResult(0, 2)) // 2 tags deleted

	mock.ExpectExec(`DELETE FROM media WHERE slug = \?`).
		WithArgs("manga2").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 media deleted

	err = DeleteMediasByLibrarySlug("lib1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFirstAndLastChapterSlugs(t *testing.T) {
	chapters := []Chapter{
		{Slug: "chapter-001", Name: "Chapter 1"},
		{Slug: "chapter-002", Name: "Chapter 2"},
		{Slug: "chapter-003", Name: "Chapter 3"},
	}

	firstSlug, lastSlug := GetFirstAndLastChapterSlugs(chapters)
	assert.Equal(t, "chapter-003", firstSlug) // Last chapter (highest number)
	assert.Equal(t, "chapter-001", lastSlug)  // First chapter (lowest number)
}

func TestGetFirstAndLastChapterSlugs_Empty(t *testing.T) {
	chapters := []Chapter{}
	firstSlug, lastSlug := GetFirstAndLastChapterSlugs(chapters)
	assert.Equal(t, "", firstSlug)
	assert.Equal(t, "", lastSlug)
}

func TestGetFirstAndLastChapterSlugs_Single(t *testing.T) {
	chapters := []Chapter{
		{Slug: "chapter-001", Name: "Chapter 1"},
	}

	firstSlug, lastSlug := GetFirstAndLastChapterSlugs(chapters)
	assert.Equal(t, "chapter-001", firstSlug)
	assert.Equal(t, "chapter-001", lastSlug)
}

func TestFilterMediasByTags_AnyMode(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "Manga 1", Tags: []string{"action", "fantasy"}},
		{Slug: "manga2", Name: "Manga 2", Tags: []string{"romance", "drama"}},
		{Slug: "manga3", Name: "Manga 3", Tags: []string{"action", "comedy"}},
	}

	// Filter by "action" OR "romance"
	result := FilterMediasByTags(mediaList, []string{"action", "romance"}, "any")
	assert.Len(t, result, 3) // All should match since each has at least one of the tags
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)
	assert.Equal(t, "manga3", result[2].Slug)
}

func TestFilterMediasByTags_AllMode(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "Manga 1", Tags: []string{"action", "fantasy"}},
		{Slug: "manga2", Name: "Manga 2", Tags: []string{"romance", "drama"}},
		{Slug: "manga3", Name: "Manga 3", Tags: []string{"action", "comedy"}},
	}

	// Filter by "action" AND "fantasy"
	result := FilterMediasByTags(mediaList, []string{"action", "fantasy"}, "all")
	assert.Len(t, result, 1) // Only manga1 has both tags
	assert.Equal(t, "manga1", result[0].Slug)
}

func TestFilterMediasByTags_NoTags(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "Manga 1", Tags: []string{"action", "fantasy"}},
		{Slug: "manga2", Name: "Manga 2", Tags: []string{"romance", "drama"}},
	}

	// No tags to filter by - should return all
	result := FilterMediasByTags(mediaList, []string{}, "any")
	assert.Len(t, result, 2)
	assert.Equal(t, mediaList, result)
}

func TestFilterMediasByTags_CaseInsensitive(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "Manga 1", Tags: []string{"ACTION", "Fantasy"}},
	}

	// Filter should be case insensitive
	result := FilterMediasByTags(mediaList, []string{"action"}, "any")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga1", result[0].Slug)
}

func TestFilterMediasBySearch_ExactMatch(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "Naruto"},
		{Slug: "manga3", Name: "Attack on Titan"},
	}

	result := FilterMediasBySearch(mediaList, "Naruto")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga2", result[0].Slug)
}

func TestFilterMediasBySearch_Substring(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "Naruto"},
		{Slug: "manga3", Name: "Attack on Titan"},
	}

	result := FilterMediasBySearch(mediaList, "Piece")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga1", result[0].Slug)
}

func TestFilterMediasBySearch_Prefix(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "Naruto"},
		{Slug: "manga3", Name: "Attack on Titan"},
	}

	result := FilterMediasBySearch(mediaList, "Att")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga3", result[0].Slug)
}

func TestFilterMediasBySearch_MultipleWords(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "Naruto Shippuden"},
		{Slug: "manga3", Name: "Attack on Titan"},
	}

	result := FilterMediasBySearch(mediaList, "Attack Titan")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga3", result[0].Slug)
}

func TestFilterMediasBySearch_CaseInsensitive(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "NARUTO"},
	}

	result := FilterMediasBySearch(mediaList, "naruto")
	assert.Len(t, result, 1)
	assert.Equal(t, "manga2", result[0].Slug)
}

func TestFilterMediasBySearch_EmptySearch(t *testing.T) {
	mediaList := []Media{
		{Slug: "manga1", Name: "One Piece"},
		{Slug: "manga2", Name: "Naruto"},
	}

	result := FilterMediasBySearch(mediaList, "")
	assert.Len(t, result, 2)
	assert.Equal(t, mediaList, result)
}

func TestGetUserFavoritesWithOptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetFavoritesForUser
	mock.ExpectQuery(`SELECT media_slug FROM favorites WHERE user_username = \? ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	// Mock GetMediasBySlugs - complex query with JOINs for tags
	mock.ExpectQuery(`SELECT.*slug IN.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAllMediaTagsMap for tag loading
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	opts := UserMediaListOptions{
		Username:  "testuser",
		Page:      1,
		PageSize:  10,
		SortBy:    "name",
		SortOrder: "asc",
	}

	result, total, err := GetUserFavoritesWithOptions(opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserReadingWithOptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetReadingForUser
	mock.ExpectQuery(`SELECT DISTINCT media_slug FROM reading_states WHERE user_name = \? ORDER BY created_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	// Mock GetMediasBySlugs - complex query with JOINs for tags
	mock.ExpectQuery(`SELECT.*slug IN.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAllMediaTagsMap for tag loading
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	opts := UserMediaListOptions{
		Username:  "testuser",
		Page:      1,
		PageSize:  10,
		SortBy:    "name",
		SortOrder: "asc",
	}

	result, total, err := GetUserReadingWithOptions(opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserUpvotedWithOptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetUpvotesForUser
	mock.ExpectQuery(`SELECT media_slug FROM votes WHERE user_username = \? AND value = 1 ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	// Mock GetMediasBySlugs - complex query with JOINs for tags
	mock.ExpectQuery(`SELECT.*slug IN.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAllMediaTagsMap for tag loading
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	opts := UserMediaListOptions{
		Username:  "testuser",
		Page:      1,
		PageSize:  10,
		SortBy:    "name",
		SortOrder: "asc",
	}

	result, total, err := GetUserUpvotedWithOptions(opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserDownvotedWithOptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetDownvotesForUser
	mock.ExpectQuery(`SELECT media_slug FROM votes WHERE user_username = \? AND value = -1 ORDER BY updated_at DESC`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug"}).
			AddRow("manga1").
			AddRow("manga2"))

	// Mock GetMediasBySlugs - complex query with JOINs for tags
	mock.ExpectQuery(`SELECT.*slug IN.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"manga1", "Manga One", "Author One", "Description One", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover1.jpg", "/path1", 10, 1640995200, 1640995300,
		).AddRow(
			"manga2", "Manga Two", "Author Two", "Description Two", 2022, "en", "manhwa", "completed", "safe", "lib1", "cover2.jpg", "/path2", 8, 1640995200, 1640995300,
		))

	// Mock GetAllMediaTagsMap for tag loading
	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "tag"}).
			AddRow("manga1", "action").
			AddRow("manga1", "fantasy").
			AddRow("manga2", "romance"))

	opts := UserMediaListOptions{
		Username:  "testuser",
		Page:      1,
		PageSize:  10,
		SortBy:    "name",
		SortOrder: "asc",
	}

	result, total, err := GetUserDownvotedWithOptions(opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Slug)
	assert.Equal(t, "manga2", result[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaAndChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetMedia query
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at",
		}).AddRow(
			"test-manga", "Test Manga", "Test Author", "Test Description", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover.jpg", "/path/to/media", 10, 1640995200, 1640995300,
		))

	// Mock GetTagsForMedia
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"tag"}).
			AddRow("action").
			AddRow("fantasy"))

	// Mock GetChapters query
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at",
		}).AddRow(
			"chapter-1", "Chapter 1", "chapter", "chapter1.cbz", "", "test-manga", 1640995200, nil,
		).AddRow(
			"chapter-2", "Chapter 2", "chapter", "chapter2.cbz", "", "test-manga", 1640995200, nil,
		))

	media, chapters, err := GetMediaAndChapters("test-manga")
	assert.NoError(t, err)
	assert.NotNil(t, media)
	assert.Equal(t, "test-manga", media.Slug)
	assert.Len(t, chapters, 2)
	assert.Equal(t, "chapter-1", chapters[0].Slug)
	assert.Equal(t, "chapter-2", chapters[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaAndChapters_MediaNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetMedia query returning no rows
	mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{}))

	media, chapters, err := GetMediaAndChapters("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, media)
	assert.Nil(t, chapters)
	assert.Contains(t, err.Error(), "media not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapterImages(t *testing.T) {
	// Mock database for GetAppConfig call
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock GetAppConfig for GetImageTokenValidityMinutes
	mock.ExpectQuery(`SELECT.*FROM app_config WHERE id = 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"allow_registration", "max_users", "content_rating_limit", "metadata_provider", "mal_api_token", "anilist_api_token", "rate_limit_enabled", "rate_limit_requests", "rate_limit_window", "bot_detection_enabled", "bot_series_threshold", "bot_chapter_threshold", "bot_detection_window", "reader_compression_quality", "moderator_compression_quality", "admin_compression_quality", "premium_compression_quality", "anonymous_compression_quality", "processed_image_quality", "image_token_validity_minutes", "premium_early_access_duration", "max_premium_chapters", "premium_cooldown_scaling_enabled",
		}).AddRow(
			1, 1000, 3, "mangadex", "", "", 1, 100, 60, 1, 5, 10, 60, 70, 85, 100, 90, 70, 85, 5, 3600, 3, 0,
		))

	// Create temporary directory and files for testing
	tempDir, err := os.MkdirTemp("", "magi_test_*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test media directory
	mediaDir := filepath.Join(tempDir, "media")
	err = os.MkdirAll(mediaDir, 0755)
	assert.NoError(t, err)

	// Create test chapter file (CBZ) and some image files inside it
	chapterFile := filepath.Join(mediaDir, "chapter1.cbz")
	err = os.MkdirAll(chapterFile, 0755)
	assert.NoError(t, err)

	// Create some test image files inside the chapter directory
	image1 := filepath.Join(chapterFile, "001.jpg")
	image2 := filepath.Join(chapterFile, "002.jpg")
	image3 := filepath.Join(chapterFile, "003.jpg")
	err = os.WriteFile(image1, []byte("fake image 1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(image2, []byte("fake image 2"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(image3, []byte("fake image 3"), 0644)
	assert.NoError(t, err)

	media := &Media{
		Slug: "test-manga",
		Path: mediaDir, // Directory containing chapter files
	}

	chapter := &Chapter{
		Slug:  "chapter-1",
		File:  "chapter1.cbz",
		Name: "Chapter 1",
	}

	images, err := GetChapterImages(media, chapter)
	assert.NoError(t, err)
	assert.Len(t, images, 3)

	// Check that URLs contain the expected pattern
	for i, url := range images {
		assert.Contains(t, url, "/api/image?token=")
		// The token should be valid for the correct page number (1-indexed)
		token := strings.TrimPrefix(url, "/api/image?token=")
		// We can't easily validate the token content without decrypting,
		// but we can check it's not empty
		assert.NotEmpty(t, token)
		_ = i // Use i to avoid unused variable warning
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapterImages_SingleFileMedia(t *testing.T) {
	// For single-file media, we can't easily test with actual CBZ files
	// So we'll test the path resolution logic instead
	tempDir, err := os.MkdirTemp("", "magi_test_*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock CBZ file
	cbzFile := filepath.Join(tempDir, "chapter1.cbz")
	err = os.WriteFile(cbzFile, []byte("fake cbz"), 0644)
	assert.NoError(t, err)

	media := &Media{
		Slug: "test-manga",
		Path: cbzFile, // Single CBZ file
	}

	chapter := &Chapter{
		Slug:  "chapter-1",
		File:  "", // No separate file for single-file media
		Name: "Chapter 1",
	}

	// Since we can't mock CountImageFiles easily, we'll test that the function
	// at least doesn't crash and handles the path correctly
	// The actual image counting would require creating valid CBZ archives
	images, err := GetChapterImages(media, chapter)
	// We expect this to fail because our fake CBZ file isn't a real archive
	// But it should fail gracefully
	assert.Error(t, err)
	assert.Nil(t, images)
}

func TestGetChapterImages_MediaPathNotExist(t *testing.T) {
	media := &Media{
		Slug: "test-manga",
		Path: "/nonexistent/path",
	}

	chapter := &Chapter{
		Slug:  "chapter-1",
		File:  "chapter1.cbz",
		Name: "Chapter 1",
	}

	images, err := GetChapterImages(media, chapter)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGetChapterImages_ChapterFileNotExist(t *testing.T) {
	// Create temporary directory but no chapter file
	tempDir, err := os.MkdirTemp("", "magi_test_*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	media := &Media{
		Slug: "test-manga",
		Path: tempDir,
	}

	chapter := &Chapter{
		Slug:  "chapter-1",
		File:  "nonexistent.cbz",
		Name: "Chapter 1",
	}

	images, err := GetChapterImages(media, chapter)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGetChapterImages_NoImages(t *testing.T) {
	// Create temporary directory and empty chapter directory
	tempDir, err := os.MkdirTemp("", "magi_test_*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	chapterFile := filepath.Join(tempDir, "chapter1.cbz")
	err = os.MkdirAll(chapterFile, 0755)
	assert.NoError(t, err)

	// Don't create any image files

	media := &Media{
		Slug: "test-manga",
		Path: tempDir,
	}

	chapter := &Chapter{
		Slug:  "chapter-1",
		File:  "chapter1.cbz",
		Name: "Chapter 1",
	}

	images, err := GetChapterImages(media, chapter)
	assert.NoError(t, err)
	assert.Len(t, images, 0)
}
// Tests for Media setter methods
func TestMediaSetName(t *testing.T) {
media := &Media{}
media.SetName("New Title")
assert.Equal(t, "New Title", media.Name)
}

func TestMediaSetDescription(t *testing.T) {
media := &Media{}
media.SetDescription("A new description")
assert.Equal(t, "A new description", media.Description)
}

func TestMediaSetYear(t *testing.T) {
media := &Media{}
media.SetYear(2023)
assert.Equal(t, 2023, media.Year)
}

func TestMediaSetOriginalLanguage(t *testing.T) {
media := &Media{}
media.SetOriginalLanguage("ja")
assert.Equal(t, "ja", media.OriginalLanguage)
}

func TestMediaSetStatus(t *testing.T) {
media := &Media{}
media.SetStatus("ongoing")
assert.Equal(t, "ongoing", media.Status)
}

func TestMediaSetContentRating(t *testing.T) {
media := &Media{}
media.SetContentRating("safe")
assert.Equal(t, "safe", media.ContentRating)
}

func TestMediaSetType(t *testing.T) {
media := &Media{}
media.SetType("manga")
assert.Equal(t, "manga", media.Type)
}

func TestMediaSetCoverArtURL(t *testing.T) {
media := &Media{}
media.SetCoverArtURL("https://example.com/cover.jpg")
assert.Equal(t, "https://example.com/cover.jpg", media.CoverArtURL)
}
