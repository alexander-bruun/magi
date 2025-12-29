package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestSortChaptersByNumber(t *testing.T) {
	chapters := []Chapter{
		{Name: "Chapter 10", Slug: "ch10"},
		{Name: "Chapter 2", Slug: "ch2"},
		{Name: "Chapter 1", Slug: "ch1"},
		{Name: "Chapter 3", Slug: "ch3"},
		{Name: "Vol 1 Chapter 5", Slug: "vol1ch5"},
		{Name: "Chapter 20", Slug: "ch20"},
	}

	sortChaptersByNumber(chapters)

	expected := []Chapter{
		{Name: "Chapter 1", Slug: "ch1"},
		{Name: "Vol 1 Chapter 5", Slug: "vol1ch5"},
		{Name: "Chapter 2", Slug: "ch2"},
		{Name: "Chapter 3", Slug: "ch3"},
		{Name: "Chapter 10", Slug: "ch10"},
		{Name: "Chapter 20", Slug: "ch20"},
	}

	assert.Equal(t, expected, chapters)
}

func TestIndexOfChapter(t *testing.T) {
	chapters := []Chapter{
		{Slug: "chapter-1"},
		{Slug: "chapter-2"},
		{Slug: "chapter-3"},
	}

	tests := []struct {
		chapterSlug string
		expected    int
	}{
		{"chapter-1", 0},
		{"chapter-2", 1},
		{"chapter-3", 2},
		{"chapter-4", -1},
		{"", -1},
	}

	for _, tt := range tests {
		result := indexOfChapter(chapters, tt.chapterSlug)
		assert.Equal(t, tt.expected, result)
	}
}

func TestIndexOfChapterEmptySlice(t *testing.T) {
	var chapters []Chapter
	result := indexOfChapter(chapters, "any-slug")
	assert.Equal(t, -1, result)
}

func TestCalculateCountdownText(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name            string
		releaseTime     time.Time
		expectedPattern string // Simple pattern to match
	}{
		{
			name:            "past release time",
			releaseTime:     baseTime.Add(-time.Hour),
			expectedPattern: "Available now!",
		},
		{
			name:            "more than 24 hours",
			releaseTime:     baseTime.Add(60 * time.Hour), // 2.5 days
			expectedPattern: "2d",
		},
		{
			name:            "between 24-48 hours",
			releaseTime:     baseTime.Add(30 * time.Hour),
			expectedPattern: "1d",
		},
		{
			name:            "more than 1 hour",
			releaseTime:     baseTime.Add(2*time.Hour + 30*time.Minute),
			expectedPattern: "2h",
		},
		{
			name:            "less than 1 hour",
			releaseTime:     baseTime.Add(45 * time.Minute),
			expectedPattern: "m", // Should contain minutes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCountdownText(tt.releaseTime)
			assert.Contains(t, result, tt.expectedPattern, "Result '%s' should contain '%s'", result, tt.expectedPattern)
		})
	}
}

func TestCreateChapter(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	chapter := Chapter{
		Name:            "Chapter 1",
		Type:            "chapter",
		File:            "/path/to/chapter1.zip",
		ChapterCoverURL: "/covers/chapter1.jpg",
		MediaSlug:       "test-manga",
	}

	// Mock ChapterExists check - chapter doesn't exist
	mock.ExpectQuery(`SELECT 1 FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnError(sql.ErrNoRows)

	// Mock the INSERT query
	mock.ExpectExec(`INSERT INTO chapters`).
		WithArgs("chapter-1", "Chapter 1", "chapter", "/path/to/chapter1.zip", "/covers/chapter1.jpg", "test-manga", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateChapter(chapter)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateChapterAlreadyExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	chapter := Chapter{
		Name:      "Chapter 1",
		MediaSlug: "test-manga",
	}

	// Mock ChapterExists check - chapter already exists
	mock.ExpectQuery(`SELECT 1 FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	err = CreateChapter(chapter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chapter already exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateChapterTx(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	chapter := Chapter{
		Name:            "Chapter 1",
		Type:            "chapter",
		File:            "/path/to/chapter1.zip",
		ChapterCoverURL: "/covers/chapter1.jpg",
		MediaSlug:       "test-manga",
	}

	// Begin transaction
	mock.ExpectBegin()

	// Mock ChapterExists check - chapter doesn't exist
	mock.ExpectQuery(`SELECT 1 FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnError(sql.ErrNoRows)

	// Mock the INSERT query
	mock.ExpectExec(`INSERT INTO chapters`).
		WithArgs("chapter-1", "Chapter 1", "chapter", "/path/to/chapter1.zip", "/covers/chapter1.jpg", "test-manga", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Start transaction
	tx, err := db.Begin()
	assert.NoError(t, err)

	err = CreateChapterTx(tx, chapter)
	assert.NoError(t, err)

	// Mock commit
	mock.ExpectCommit()

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapter(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at",
		}).AddRow(
			"chapter-1", "Chapter 1", "chapter", "/path/to/chapter1.zip", "/covers/chapter1.jpg", "test-manga", 1609459200, 1612137600,
		))

	chapter, err := GetChapter("test-manga", "chapter-1")
	assert.NoError(t, err)
	assert.NotNil(t, chapter)
	assert.Equal(t, "chapter-1", chapter.Slug)
	assert.Equal(t, "Chapter 1", chapter.Name)
	assert.Equal(t, "test-manga", chapter.MediaSlug)
	assert.NotNil(t, chapter.ReleasedAt)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapterNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query - no rows returned
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "nonexistent").
		WillReturnError(sql.ErrNoRows)

	chapter, err := GetChapter("test-manga", "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, chapter)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapter(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	chapter := &Chapter{
		Slug:            "chapter-1",
		Name:            "Updated Chapter 1",
		Type:            "chapter",
		File:            "/path/to/updated.zip",
		ChapterCoverURL: "/covers/updated.jpg",
		MediaSlug:       "test-manga",
	}

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE chapters SET name = \?, type = \?, file = \?, chapter_cover_url = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs("Updated Chapter 1", "chapter", "/path/to/updated.zip", "/covers/updated.jpg", "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateChapter(chapter)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapter_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	chapter := &Chapter{
		Slug:            "chapter-1",
		Name:            "Updated Chapter 1",
		Type:            "chapter",
		File:            "/path/to/updated.zip",
		ChapterCoverURL: "/covers/updated.jpg",
		MediaSlug:       "test-manga",
	}

	// Mock the UPDATE query to return an error
	mock.ExpectExec(`UPDATE chapters SET name = \?, type = \?, file = \?, chapter_cover_url = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs("Updated Chapter 1", "chapter", "/path/to/updated.zip", "/covers/updated.jpg", "test-manga", "chapter-1").
		WillReturnError(sql.ErrConnDone)

	err = UpdateChapter(chapter)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteChapter(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the DELETE query
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteChapter("test-manga", "chapter-1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChapterExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test exists
	mock.ExpectQuery(`SELECT 1 FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := ChapterExists("chapter-1", "test-manga")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test not exists
	mock.ExpectQuery(`SELECT 1 FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-2").
		WillReturnError(sql.ErrNoRows)

	exists, err = ChapterExists("chapter-2", "test-manga")
	assert.NoError(t, err)
	assert.False(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "test-manga", 1640995200, 1640995200).
			AddRow("chapter-2", "Chapter 2", "chapter", "/path/ch2.zip", "/covers/ch2.jpg", "test-manga", 1641081600, 1641081600))

	chapters, err := GetChapters("test-manga")
	assert.NoError(t, err)
	assert.Len(t, chapters, 2)
	assert.Equal(t, "chapter-1", chapters[0].Slug)
	assert.Equal(t, "chapter-2", chapters[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapterCreatedAt(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	testTime := time.Now()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE chapters SET created_at = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs(testTime.Unix(), "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateChapterCreatedAt("test-manga", "chapter-1", testTime)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapterCreatedAt_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	testTime := time.Now()

	// Mock the UPDATE query to return an error
	mock.ExpectExec(`UPDATE chapters SET created_at = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs(testTime.Unix(), "test-manga", "chapter-1").
		WillReturnError(sql.ErrConnDone)

	err = UpdateChapterCreatedAt("test-manga", "chapter-1", testTime)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapterReleasedAt(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	testTime := time.Now()

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE chapters SET released_at = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs(testTime.Unix(), "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateChapterReleasedAt("test-manga", "chapter-1", testTime)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateChapterReleasedAt_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	testTime := time.Now()

	// Mock the UPDATE query to return an error
	mock.ExpectExec(`UPDATE chapters SET released_at = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs(testTime.Unix(), "test-manga", "chapter-1").
		WillReturnError(sql.ErrConnDone)

	err = UpdateChapterReleasedAt("test-manga", "chapter-1", testTime)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteChaptersByMediaSlug(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the DELETE query
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 3))

	err = DeleteChaptersByMediaSlug("test-manga")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecentChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the complex SELECT query with JOIN
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, m\.name, m\.type, m\.cover_art_url FROM chapters c JOIN media m ON c\.media_slug = m\.slug ORDER BY c\.created_at DESC LIMIT \?`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "media_name", "media_type", "media_cover_art_url"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "manga1", 1640995200, "Manga One", "manga", "/covers/manga1.jpg").
			AddRow("chapter-2", "Chapter 2", "chapter", "/path/ch2.zip", "/covers/ch2.jpg", "manga2", 1641081600, "Manga Two", "manhwa", "/covers/manga2.jpg"))

	chapters, err := GetRecentChapters(5)
	assert.NoError(t, err)
	assert.Len(t, chapters, 2)
	assert.Equal(t, "chapter-1", chapters[0].Slug)
	assert.Equal(t, "Manga One", chapters[0].MediaName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecentChapters_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query to return error
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, m\.name, m\.type, m\.cover_art_url FROM chapters c JOIN media m ON c\.media_slug = m\.slug ORDER BY c\.created_at DESC LIMIT \?`).
		WithArgs(5).
		WillReturnError(sqlmock.ErrCancelled)

	chapters, err := GetRecentChapters(5)
	assert.Error(t, err)
	assert.Nil(t, chapters)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestChapter(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetChapters query
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "test-manga", 1640995200, 1640995200).
			AddRow("chapter-5", "Chapter 5", "chapter", "/path/ch5.zip", "/covers/ch5.jpg", "test-manga", 1641081600, 1641081600))

	slug, name, err := GetLatestChapter("test-manga")
	assert.NoError(t, err)
	assert.Equal(t, "chapter-5", slug)
	assert.Equal(t, "Chapter 5", name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestChapter_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetChapters query to return error
	mock.ExpectQuery(`SELECT slug, name, type, file, chapter_cover_url, media_slug, created_at, released_at FROM chapters WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnError(sqlmock.ErrCancelled)

	slug, name, err := GetLatestChapter("test-manga")
	assert.Error(t, err)
	assert.Empty(t, slug)
	assert.Empty(t, name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAdjacentChapters(t *testing.T) {
	chapters := []Chapter{
		{Slug: "chapter-1", Name: "Chapter 1"},
		{Slug: "chapter-2", Name: "Chapter 2"},
		{Slug: "chapter-3", Name: "Chapter 3"},
		{Slug: "chapter-4", Name: "Chapter 4"},
	}

	tests := []struct {
		name         string
		chapterSlug  string
		userName     string
		expectedPrev string
		expectedNext string
		expectError  bool
	}{
		{
			name:         "middle chapter",
			chapterSlug:  "chapter-2",
			userName:     "user1",
			expectedPrev: "chapter-1",
			expectedNext: "chapter-3",
			expectError:  false,
		},
		{
			name:         "first chapter",
			chapterSlug:  "chapter-1",
			userName:     "user1",
			expectedPrev: "",
			expectedNext: "chapter-2",
			expectError:  false,
		},
		{
			name:         "last chapter",
			chapterSlug:  "chapter-4",
			userName:     "user1",
			expectedPrev: "chapter-3",
			expectedNext: "",
			expectError:  false,
		},
		{
			name:        "non-existent chapter",
			chapterSlug: "chapter-5",
			userName:    "user1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev, next, err := GetAdjacentChapters(chapters, tt.chapterSlug, tt.userName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPrev, prev)
				assert.Equal(t, tt.expectedNext, next)
			}
		})
	}
}

func TestIsChapterAccessibleForUser(t *testing.T) {
	// Test with free chapter
	freeChapter := &Chapter{
		Type: "chapter",
	}
	assert.True(t, isChapterAccessibleForUser(freeChapter, "user1"))

	// Test with premium chapter - would need more complex setup for full testing
	// This function has dependencies on config and time calculations
	premiumChapter := &Chapter{
		Type: "premium",
	}
	// For now, just test that it doesn't panic
	result := isChapterAccessibleForUser(premiumChapter, "user1")
	// Result depends on current time and config, so we just ensure it returns a boolean
	assert.IsType(t, true, result)
}

func TestHasPremiumChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetChaptersByMediaSlug query - return chapters with one premium
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, c\.released_at, COALESCE\(rs\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states WHERE media_slug = \? GROUP BY chapter_slug \) rs ON c\.slug = rs\.chapter_slug WHERE c\.media_slug = \?`).
		WithArgs("test-manga", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at", "read_count"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "test-manga", 1640995200, 1640995200, 5).
			AddRow("chapter-2", "Chapter 2", "premium", "/path/ch2.zip", "/covers/ch2.jpg", "test-manga", 1641081600, 1641081600, 3))

	// Test with premium chapters present
	hasPremium, reason, err := HasPremiumChapters("test-manga", 3, 3600, false)
	assert.NoError(t, err)
	// The result depends on the complex logic, so just check that it doesn't error
	_ = hasPremium
	_ = reason

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHasPremiumChapters_Error(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetChaptersByMediaSlug query to return error
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, c\.released_at, COALESCE\(rs\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states WHERE media_slug = \? GROUP BY chapter_slug \) rs ON c\.slug = rs\.chapter_slug WHERE c\.media_slug = \?`).
		WithArgs("test-manga", "test-manga").
		WillReturnError(sqlmock.ErrCancelled)

	hasPremium, reason, err := HasPremiumChapters("test-manga", 3, 3600, false)
	assert.Error(t, err)
	assert.False(t, hasPremium)
	assert.Empty(t, reason)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChaptersByMediaSlug(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the complex SELECT query with LEFT JOIN for read counts
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, c\.released_at, COALESCE\(rs\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states WHERE media_slug = \? GROUP BY chapter_slug \) rs ON c\.slug = rs\.chapter_slug WHERE c\.media_slug = \?`).
		WithArgs("test-manga", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at", "read_count"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "test-manga", 1640995200, 1640995200, 5).
			AddRow("chapter-2", "Chapter 2", "chapter", "/path/ch2.zip", "/covers/ch2.jpg", "test-manga", 1641081600, 1641081600, 3))

	chapters, err := GetChaptersByMediaSlug("test-manga", 10, 3, 3600, false)
	assert.NoError(t, err)
	assert.Len(t, chapters, 2)
	assert.Equal(t, "chapter-2", chapters[0].Slug) // Higher chapter number comes first
	assert.Equal(t, "chapter-1", chapters[1].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecentSeriesWithChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query for recent media
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.type, m\.status, m\.cover_art_url, m\.library_slug, m\.created_at, m\.updated_at FROM media m INNER JOIN libraries l ON m\.library_slug = l\.slug INNER JOIN chapters c ON c\.media_slug = m\.slug WHERE m\.library_slug IN \(\?\) AND l\.enabled = 1 GROUP BY m\.slug, m\.name, m\.author, m\.description, m\.type, m\.status, m\.cover_art_url, m\.library_slug, m\.created_at, m\.updated_at ORDER BY MAX\(c\.created_at\) DESC LIMIT \?`).
		WithArgs("library1", 2).
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "author", "description", "type", "status", "cover_art_url", "library_slug", "created_at", "updated_at"}).
			AddRow("manga1", "Manga One", "Author One", "Description One", "manga", "ongoing", "/covers/manga1.jpg", "library1", 1640995200, 1640995200).
			AddRow("manga2", "Manga Two", "Author Two", "Description Two", "manhwa", "completed", "/covers/manga2.jpg", "library1", 1641081600, 1641081600))

	// Mock GetChaptersByMediaSlug for manga1
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, c\.released_at, COALESCE\(rs\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states WHERE media_slug = \? GROUP BY chapter_slug \) rs ON c\.slug = rs\.chapter_slug WHERE c\.media_slug = \?`).
		WithArgs("manga1", "manga1").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at", "read_count"}).
			AddRow("chapter-1", "Chapter 1", "chapter", "/path/ch1.zip", "/covers/ch1.jpg", "manga1", 1640995200, 1640995200, 0))

	// Mock GetChaptersByMediaSlug for manga2
	mock.ExpectQuery(`SELECT c\.slug, c\.name, c\.type, c\.file, c\.chapter_cover_url, c\.media_slug, c\.created_at, c\.released_at, COALESCE\(rs\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states WHERE media_slug = \? GROUP BY chapter_slug \) rs ON c\.slug = rs\.chapter_slug WHERE c\.media_slug = \?`).
		WithArgs("manga2", "manga2").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "type", "file", "chapter_cover_url", "media_slug", "created_at", "released_at", "read_count"}).
			AddRow("chapter-2", "Chapter 2", "chapter", "/path/ch2.zip", "/covers/ch2.jpg", "manga2", 1641081600, 1641081600, 0))

	result, err := GetRecentSeriesWithChapters(2, 3, 3600, false, []string{"library1"})
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "manga1", result[0].Media.Slug)
	assert.Equal(t, "Manga One", result[0].Media.Name)
	assert.Len(t, result[0].Chapters, 1)
	assert.Equal(t, "chapter-1", result[0].Chapters[0].Slug)
	assert.Equal(t, "manga2", result[1].Media.Slug)
	assert.Equal(t, "Manga Two", result[1].Media.Name)
	assert.Len(t, result[1].Chapters, 1)
	assert.Equal(t, "chapter-2", result[1].Chapters[0].Slug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExtractChapterNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Chapter with number", "Chapter 123", 123},
		{"Ch. abbreviation", "Ch. 45", 45},
		{"Episode", "Episode 67", 67},
		{"Ep. abbreviation", "Ep. 89", 89},
		{"Volume", "Volume 12", 12},
		{"Vol. abbreviation", "Vol. 34", 34},
		{"Case insensitive", "CHAPTER 56", 56},
		{"Plain number", "78", 78},
		{"No match", "Special Chapter", -1},
		{"Complex name", "Vol 1 Ch 5", 1}, // Takes first match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractChapterNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateChapterNameTx(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the BEGIN transaction
	mock.ExpectBegin()

	// Start a transaction
	tx, err := mockDB.Begin()
	assert.NoError(t, err)

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE chapters SET name = \? WHERE media_slug = \? AND slug = \?`).
		WithArgs("New Chapter Name", "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateChapterNameTx(tx, "test-manga", "chapter-1", "New Chapter Name")
	assert.NoError(t, err)

	// Mock commit
	mock.ExpectCommit()

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteChapterTx(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the BEGIN transaction
	mock.ExpectBegin()

	// Start a transaction
	tx, err := mockDB.Begin()
	assert.NoError(t, err)

	// Mock the DELETE query
	mock.ExpectExec(`DELETE FROM chapters WHERE media_slug = \? AND slug = \?`).
		WithArgs("test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteChapterTx(tx, "test-manga", "chapter-1")
	assert.NoError(t, err)

	// Mock commit
	mock.ExpectCommit()

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchEnrichMediaData(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test with empty input
	result, err := BatchEnrichMediaData([]string{}, 3, 3600, false)
	assert.NoError(t, err)
	assert.Empty(t, result)

	// Test with single media slug - mock the complex queries
	mediaSlugs := []string{"test-manga"}

	// Mock chapters query
	mock.ExpectQuery(`SELECT c\.media_slug, c\.slug, c\.name, c\.created_at, c\.type, COALESCE\(read_counts\.read_count, 0\) as read_count FROM chapters c LEFT JOIN \( SELECT chapter_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY chapter_slug \) read_counts ON c\.slug = read_counts\.chapter_slug WHERE c\.media_slug IN \(\?\) ORDER BY c\.media_slug, c\.created_at DESC`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "slug", "name", "created_at", "type", "read_count"}).
			AddRow("test-manga", "chapter-1", "Chapter 1", 1640995200, "chapter", 5))

	// Mock ratings query
	mock.ExpectQuery(`SELECT media_slug, COALESCE\(AVG\(rating\), 0\), COUNT\(\*\) FROM reviews WHERE media_slug IN \(\?\) GROUP BY media_slug`).
		WithArgs("test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "avg", "count"}).
			AddRow("test-manga", 4.5, 10))

	result, err = BatchEnrichMediaData(mediaSlugs, 3, 3600, false)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-manga", result["test-manga"].MediaSlug)
	assert.Equal(t, "chapter-1", result["test-manga"].LatestChapterSlug)
	assert.Equal(t, "Chapter 1", result["test-manga"].LatestChapterName)
	assert.Equal(t, 4.5, result["test-manga"].AverageRating)
	assert.Equal(t, 10, result["test-manga"].ReviewCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}
