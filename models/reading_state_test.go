package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestMarkChapterRead(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`INSERT INTO reading_states \(user_name, media_slug, chapter_slug, created_at\) VALUES \(\?, \?, \?, CURRENT_TIMESTAMP\) ON CONFLICT\(user_name, media_slug, chapter_slug\) DO UPDATE SET created_at = CURRENT_TIMESTAMP`).
		WithArgs("testuser", "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock RecordDailyStatistics calls
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM chapters`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))
	mock.ExpectExec(`INSERT OR REPLACE INTO daily_statistics \(date, total_media, total_chapters, total_chapters_read\) VALUES \(\?, \?, \?, \?\)`).
		WithArgs(sqlmock.AnyArg(), 100, 1000, 500).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = MarkChapterRead("testuser", "test-manga", "chapter-1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUnmarkChapterRead(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM reading_states WHERE user_name = \? AND media_slug = \? AND chapter_slug = \?`).
		WithArgs("testuser", "test-manga", "chapter-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock RecordDailyStatistics calls
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM chapters`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))
	mock.ExpectExec(`INSERT OR REPLACE INTO daily_statistics \(date, total_media, total_chapters, total_chapters_read\) VALUES \(\?, \?, \?, \?\)`).
		WithArgs(sqlmock.AnyArg(), 100, 1000, 500).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = UnmarkChapterRead("testuser", "test-manga", "chapter-1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReadChaptersForUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"chapter_slug"}).
		AddRow("chapter-1").
		AddRow("chapter-2").
		AddRow("chapter-3")

	mock.ExpectQuery(`SELECT chapter_slug FROM reading_states WHERE user_name = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(rows)

	result, err := GetReadChaptersForUser("testuser", "test-manga")
	assert.NoError(t, err)
	expected := map[string]bool{
		"chapter-1": true,
		"chapter-2": true,
		"chapter-3": true,
	}
	assert.Equal(t, expected, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserReadCount(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"count"}).
		AddRow(5)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE user_name = \? AND media_slug = \?`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(rows)

	count, err := GetUserReadCount("testuser", "test-manga")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLastReadChapter(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"chapter_slug"}).
		AddRow("chapter-5")

	mock.ExpectQuery(`SELECT chapter_slug FROM reading_states WHERE user_name = \? AND media_slug = \? ORDER BY created_at DESC LIMIT 1`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(rows)

	chapter, err := GetLastReadChapter("testuser", "test-manga")
	assert.NoError(t, err)
	assert.Equal(t, "chapter-5", chapter)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLastReadChapter_NoReads(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	mock.ExpectQuery(`SELECT chapter_slug FROM reading_states WHERE user_name = \? AND media_slug = \? ORDER BY created_at DESC LIMIT 1`).
		WithArgs("testuser", "test-manga").
		WillReturnRows(sqlmock.NewRows([]string{"chapter_slug"}))

	chapter, err := GetLastReadChapter("testuser", "test-manga")
	assert.NoError(t, err)
	assert.Equal(t, "", chapter)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChapterProgress(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"image_index"}).
		AddRow(5)

	mock.ExpectQuery(`SELECT image_index FROM reading_states WHERE user_name = \? AND media_slug = \? AND chapter_slug = \?`).
		WithArgs("testuser", "test-manga", "chapter-3").
		WillReturnRows(rows)

	progress, err := GetChapterProgress("testuser", "test-manga", "chapter-3")
	assert.NoError(t, err)
	assert.Equal(t, 5, progress)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReadingStatesByUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM reading_states WHERE user_name = \?`).
		WithArgs("testuser").
		WillReturnResult(sqlmock.NewResult(0, 10))

	err = DeleteReadingStatesByUser("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}