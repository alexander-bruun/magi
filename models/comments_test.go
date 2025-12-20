package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateComment(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT INTO comments \(user_username, target_type, target_slug, media_slug, content, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?, \?, \?\)`).
		WithArgs("testuser", "media", "test-media", "test-media", "This is a test comment", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	comment := Comment{
		UserUsername: "testuser",
		TargetType:   "media",
		TargetSlug:   "test-media",
		MediaSlug:    "test-media",
		Content:      "This is a test comment",
	}

	err = CreateComment(comment)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateComment_Error(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec to return error
	mock.ExpectExec(`INSERT INTO comments \(user_username, target_type, target_slug, media_slug, content, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?, \?, \?\)`).
		WithArgs("testuser", "media", "test-media", "test-media", "This is a test comment", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sqlmock.ErrCancelled)

	comment := Comment{
		UserUsername: "testuser",
		TargetType:   "media",
		TargetSlug:   "test-media",
		MediaSlug:    "test-media",
		Content:      "This is a test comment",
	}

	err = CreateComment(comment)
	assert.Error(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCommentsByTarget(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query - sqlmock returns rows in the order added, so add them in DESC order
	rows := sqlmock.NewRows([]string{"id", "user_username", "target_type", "target_slug", "media_slug", "content", "created_at", "updated_at"}).
		AddRow(2, "user2", "media", "test-media", "test-media", "Another comment", 1640995300, 1640995300).
		AddRow(1, "testuser", "media", "test-media", "test-media", "This is a test comment", 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT id, user_username, target_type, target_slug, media_slug, content, created_at, updated_at FROM comments WHERE target_type = \? AND target_slug = \? ORDER BY created_at DESC`).
		WithArgs("media", "test-media").
		WillReturnRows(rows)

	comments, err := GetCommentsByTarget("media", "test-media")
	assert.NoError(t, err)
	assert.Len(t, comments, 2)

	// Comments should be ordered by created_at DESC (newest first)
	assert.Equal(t, 2, comments[0].ID)
	assert.Equal(t, "user2", comments[0].UserUsername)
	assert.Equal(t, "media", comments[0].TargetType)
	assert.Equal(t, "test-media", comments[0].TargetSlug)
	assert.Equal(t, "test-media", comments[0].MediaSlug)
	assert.Equal(t, "Another comment", comments[0].Content)
	assert.Equal(t, time.Unix(1640995300, 0), comments[0].CreatedAt)
	assert.Equal(t, time.Unix(1640995300, 0), comments[0].UpdatedAt)

	assert.Equal(t, 1, comments[1].ID)
	assert.Equal(t, "testuser", comments[1].UserUsername)
	assert.Equal(t, "This is a test comment", comments[1].Content)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCommentsByTarget_Error(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query to return error
	mock.ExpectQuery(`SELECT id, user_username, target_type, target_slug, media_slug, content, created_at, updated_at FROM comments WHERE target_type = \? AND target_slug = \? ORDER BY created_at DESC`).
		WithArgs("media", "test-media").
		WillReturnError(sqlmock.ErrCancelled)

	comments, err := GetCommentsByTarget("media", "test-media")
	assert.Error(t, err)
	assert.Nil(t, comments)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCommentsByTargetAndMedia(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "user_username", "target_type", "target_slug", "media_slug", "content", "created_at", "updated_at"}).
		AddRow(1, "testuser", "chapter", "chapter-1", "test-media", "Comment on chapter", 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT id, user_username, target_type, target_slug, media_slug, content, created_at, updated_at FROM comments WHERE target_type = \? AND target_slug = \? AND media_slug = \? ORDER BY created_at DESC`).
		WithArgs("chapter", "chapter-1", "test-media").
		WillReturnRows(rows)

	comments, err := GetCommentsByTargetAndMedia("chapter", "chapter-1", "test-media")
	assert.NoError(t, err)
	assert.Len(t, comments, 1)

	assert.Equal(t, 1, comments[0].ID)
	assert.Equal(t, "testuser", comments[0].UserUsername)
	assert.Equal(t, "chapter", comments[0].TargetType)
	assert.Equal(t, "chapter-1", comments[0].TargetSlug)
	assert.Equal(t, "test-media", comments[0].MediaSlug)
	assert.Equal(t, "Comment on chapter", comments[0].Content)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteComment(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM comments WHERE id = \? AND user_username = \?`).
		WithArgs(1, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteComment(1, "testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteComment_NotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec - no rows affected
	mock.ExpectExec(`DELETE FROM comments WHERE id = \? AND user_username = \?`).
		WithArgs(1, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = DeleteComment(1, "testuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "comment not found or not authorized")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteComment_Error(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec to return error
	mock.ExpectExec(`DELETE FROM comments WHERE id = \? AND user_username = \?`).
		WithArgs(1, "testuser").
		WillReturnError(sqlmock.ErrCancelled)

	err = DeleteComment(1, "testuser")
	assert.Error(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}