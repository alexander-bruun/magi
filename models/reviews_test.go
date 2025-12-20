package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateReview(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT INTO reviews \(user_username, media_slug, rating, content, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?, \?\) ON CONFLICT\(user_username, media_slug\) DO UPDATE SET rating = excluded\.rating, content = excluded\.content, updated_at = excluded\.updated_at`).
		WithArgs("testuser", "test-media", 8, "Great media!", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	review := Review{
		UserUsername: "testuser",
		MediaSlug:    "test-media",
		Rating:       8,
		Content:      "Great media!",
	}

	err = CreateReview(review)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReviewsByMedia(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query - sqlmock returns rows in the order added, so add them in DESC order
	rows := sqlmock.NewRows([]string{"id", "user_username", "media_slug", "rating", "content", "created_at", "updated_at"}).
		AddRow(2, "user2", "test-media", 7, "Good", 1640995300, 1640995300).
		AddRow(1, "user1", "test-media", 9, "Excellent!", 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT id, user_username, media_slug, rating, content, created_at, updated_at FROM reviews WHERE media_slug = \? ORDER BY created_at DESC`).
		WithArgs("test-media").
		WillReturnRows(rows)

	reviews, err := GetReviewsByMedia("test-media")
	assert.NoError(t, err)
	assert.Len(t, reviews, 2)

	assert.Equal(t, 2, reviews[0].ID)
	assert.Equal(t, "user2", reviews[0].UserUsername)
	assert.Equal(t, 7, reviews[0].Rating)
	assert.Equal(t, "Good", reviews[0].Content)

	assert.Equal(t, 1, reviews[1].ID)
	assert.Equal(t, "user1", reviews[1].UserUsername)
	assert.Equal(t, 9, reviews[1].Rating)
	assert.Equal(t, "Excellent!", reviews[1].Content)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReviewByUserAndMedia(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "user_username", "media_slug", "rating", "content", "created_at", "updated_at"}).
		AddRow(1, "testuser", "test-media", 8, "Nice!", 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT id, user_username, media_slug, rating, content, created_at, updated_at FROM reviews WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-media").
		WillReturnRows(rows)

	review, err := GetReviewByUserAndMedia("testuser", "test-media")
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, 1, review.ID)
	assert.Equal(t, "testuser", review.UserUsername)
	assert.Equal(t, "test-media", review.MediaSlug)
	assert.Equal(t, 8, review.Rating)
	assert.Equal(t, "Nice!", review.Content)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReviewByUserAndMedia_NotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	mock.ExpectQuery(`SELECT id, user_username, media_slug, rating, content, created_at, updated_at FROM reviews WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-media").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_username", "media_slug", "rating", "content", "created_at", "updated_at"}))

	review, err := GetReviewByUserAndMedia("testuser", "test-media")
	assert.NoError(t, err)
	assert.Nil(t, review)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReviewByID(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "user_username", "media_slug", "rating", "content", "created_at", "updated_at"}).
		AddRow(1, "testuser", "test-media", 8, "Nice!", 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT id, user_username, media_slug, rating, content, created_at, updated_at FROM reviews WHERE id = \?`).
		WithArgs(1).
		WillReturnRows(rows)

	review, err := GetReviewByID(1)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, 1, review.ID)
	assert.Equal(t, "testuser", review.UserUsername)
	assert.Equal(t, 8, review.Rating)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAverageRating(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"avg_rating", "count"}).
		AddRow(7.5, 4)

	mock.ExpectQuery(`SELECT COALESCE\(AVG\(rating\), 0\), COUNT\(\*\) FROM reviews WHERE media_slug = \?`).
		WithArgs("test-media").
		WillReturnRows(rows)

	avgRating, count, err := GetAverageRating("test-media")
	assert.NoError(t, err)
	assert.Equal(t, 7.5, avgRating)
	assert.Equal(t, 4, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReview(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM reviews WHERE user_username = \? AND media_slug = \?`).
		WithArgs("testuser", "test-media").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteReview("testuser", "test-media")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReviewByID(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM reviews WHERE id = \?`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteReviewByID(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReviewByID(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`UPDATE reviews SET rating = \?, content = \? WHERE id = \?`).
		WithArgs(9, "Updated review", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateReviewByID(1, 9, "Updated review")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}