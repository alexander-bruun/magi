package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetTagsForMedia(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("action").
		AddRow("adventure").
		AddRow("fantasy")

	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs("test-manga").
		WillReturnRows(rows)

	// Call the function
	tags, err := GetTagsForMedia("test-manga")
	assert.NoError(t, err)
	assert.Equal(t, []string{"action", "adventure", "fantasy"}, tags)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagsForMediaNoTags(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	rows := sqlmock.NewRows([]string{"tag"})
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs("empty-manga").
		WillReturnRows(rows)

	tags, err := GetTagsForMedia("empty-manga")
	assert.NoError(t, err)
	assert.Empty(t, tags)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllTags(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query with unsorted tags
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("fantasy").
		AddRow("action").
		AddRow("").
		AddRow("adventure")

	mock.ExpectQuery(`SELECT DISTINCT tag FROM media_tags`).
		WillReturnRows(rows)

	// Call the function
	tags, err := GetAllTags()
	assert.NoError(t, err)
	assert.Equal(t, []string{"action", "adventure", "fantasy"}, tags) // sorted and empty filtered

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagUsageStats(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag", "count"}).
		AddRow("action", 5).
		AddRow("fantasy", 3).
		AddRow("", 2) // empty tag should be filtered

	mock.ExpectQuery(`SELECT tag, COUNT\(\*\) as count FROM media_tags GROUP BY tag ORDER BY count DESC`).
		WillReturnRows(rows)

	// Call the function
	stats, err := GetTagUsageStats()
	assert.NoError(t, err)
	expected := map[string]int{
		"action":  5,
		"fantasy": 3,
	}
	assert.Equal(t, expected, stats)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetTagsForMedia(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock transaction begin
	mock.ExpectBegin()

	// Mock delete
	mock.ExpectExec(`DELETE FROM media_tags WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	// Mock prepare
	mock.ExpectPrepare(`INSERT INTO media_tags \(media_slug, tag\) VALUES \(\?, \?\)`)

	// Mock inserts
	mock.ExpectExec(`INSERT INTO media_tags \(media_slug, tag\) VALUES \(\?, \?\)`).
		WithArgs("test-manga", "action").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`INSERT INTO media_tags \(media_slug, tag\) VALUES \(\?, \?\)`).
		WithArgs("test-manga", "fantasy").
		WillReturnResult(sqlmock.NewResult(2, 1))

	// Mock commit
	mock.ExpectCommit()

	// Call the function
	err = SetTagsForMedia("test-manga", []string{"action", "fantasy"})
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteTagsByMediaSlug(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the delete query
	mock.ExpectExec(`DELETE FROM media_tags WHERE media_slug = \?`).
		WithArgs("test-manga").
		WillReturnResult(sqlmock.NewResult(0, 3)) // 3 rows deleted

	// Call the function
	err = DeleteTagsByMediaSlug("test-manga")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllMediaTagsMap(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"media_slug", "tag"}).
		AddRow("manga1", "action").
		AddRow("manga1", "adventure").
		AddRow("manga2", "fantasy").
		AddRow("manga2", "romance")

	mock.ExpectQuery(`SELECT media_slug, tag FROM media_tags`).
		WillReturnRows(rows)

	// Call the function
	result, err := GetAllMediaTagsMap()
	assert.NoError(t, err)
	expected := map[string][]string{
		"manga1": {"action", "adventure"},
		"manga2": {"fantasy", "romance"},
	}
	assert.Equal(t, expected, result)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagsForUserFavorites(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("action").
		AddRow("adventure").
		AddRow("fantasy")

	mock.ExpectQuery(`SELECT DISTINCT mt\.tag FROM media_tags mt INNER JOIN favorites f ON mt\.media_slug = f\.media_slug WHERE f\.user_username = \? ORDER BY mt\.tag`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Call the function
	tags, err := GetTagsForUserFavorites("testuser")
	assert.NoError(t, err)
	assert.Equal(t, []string{"action", "adventure", "fantasy"}, tags)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagsForUserReading(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("action").
		AddRow("drama")

	mock.ExpectQuery(`SELECT DISTINCT mt\.tag FROM media_tags mt INNER JOIN reading_states rs ON mt\.media_slug = rs\.media_slug WHERE rs\.user_name = \? ORDER BY mt\.tag`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Call the function
	tags, err := GetTagsForUserReading("testuser")
	assert.NoError(t, err)
	assert.Equal(t, []string{"action", "drama"}, tags)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagsForUserUpvoted(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("comedy").
		AddRow("slice-of-life")

	mock.ExpectQuery(`SELECT DISTINCT mt\.tag FROM media_tags mt INNER JOIN votes v ON mt\.media_slug = v\.media_slug WHERE v\.user_username = \? AND v\.value = 1 ORDER BY mt\.tag`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Call the function
	tags, err := GetTagsForUserUpvoted("testuser")
	assert.NoError(t, err)
	assert.Equal(t, []string{"comedy", "slice-of-life"}, tags)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTagsForUserDownvoted(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"tag"}).
		AddRow("horror").
		AddRow("thriller")

	mock.ExpectQuery(`SELECT DISTINCT mt\.tag FROM media_tags mt INNER JOIN votes v ON mt\.media_slug = v\.media_slug WHERE v\.user_username = \? AND v\.value = -1 ORDER BY mt\.tag`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Call the function
	tags, err := GetTagsForUserDownvoted("testuser")
	assert.NoError(t, err)
	assert.Equal(t, []string{"horror", "thriller"}, tags)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}