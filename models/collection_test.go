package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT INTO collections \(name, description, created_by, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("Test Collection", "A test collection", "testuser", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	collection, err := CreateCollection("Test Collection", "A test collection", "testuser")
	assert.NoError(t, err)
	assert.NotNil(t, collection)
	assert.Equal(t, 1, collection.ID)
	assert.Equal(t, "Test Collection", collection.Name)
	assert.Equal(t, "A test collection", collection.Description)
	assert.Equal(t, "testuser", collection.CreatedBy)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCollection_Error(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec to return error
	mock.ExpectExec(`INSERT INTO collections \(name, description, created_by, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("Test Collection", "A test collection", "testuser", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sqlmock.ErrCancelled)

	collection, err := CreateCollection("Test Collection", "A test collection", "testuser")
	assert.Error(t, err)
	assert.Nil(t, collection)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionByID(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "updated_at", "media_count"}).
		AddRow(1, "Test Collection", "A test collection", "testuser", 1640995200, 1640995200, 5)

	mock.ExpectQuery(`SELECT c\.id, c\.name, c\.description, c\.created_by, c\.created_at, c\.updated_at, COUNT\(cm\.media_slug\) as media_count FROM collections c LEFT JOIN collection_media cm ON c\.id = cm\.collection_id WHERE c\.id = \? GROUP BY c\.id`).
		WithArgs(1).
		WillReturnRows(rows)

	collection, err := GetCollectionByID(1)
	assert.NoError(t, err)
	assert.NotNil(t, collection)
	assert.Equal(t, 1, collection.ID)
	assert.Equal(t, "Test Collection", collection.Name)
	assert.Equal(t, "A test collection", collection.Description)
	assert.Equal(t, "testuser", collection.CreatedBy)
	assert.Equal(t, time.Unix(1640995200, 0), collection.CreatedAt)
	assert.Equal(t, time.Unix(1640995200, 0), collection.UpdatedAt)
	assert.Equal(t, 5, collection.MediaCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionByID_NotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	mock.ExpectQuery(`SELECT c\.id, c\.name, c\.description, c\.created_by, c\.created_at, c\.updated_at, COUNT\(cm\.media_slug\) as media_count FROM collections c LEFT JOIN collection_media cm ON c\.id = cm\.collection_id WHERE c\.id = \? GROUP BY c\.id`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "updated_at", "media_count"}))

	collection, err := GetCollectionByID(1)
	assert.NoError(t, err)
	assert.Nil(t, collection)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionsByUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "updated_at", "media_count"}).
		AddRow(1, "Collection 1", "Description 1", "testuser", 1640995200, 1640995200, 5).
		AddRow(2, "Collection 2", "Description 2", "testuser", 1640995300, 1640995300, 3)

	mock.ExpectQuery(`SELECT c\.id, c\.name, c\.description, c\.created_by, c\.created_at, c\.updated_at, COUNT\(cm\.media_slug\) as media_count FROM collections c LEFT JOIN collection_media cm ON c\.id = cm\.collection_id WHERE c\.created_by = \? GROUP BY c\.id ORDER BY c\.created_at DESC`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Mock GetTopMediaInCollection calls for each collection
	topMediaRows := sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at"}).
		AddRow("manga1", "Manga One", "Author One", "Description", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover.jpg", "/path", 10, 5, 8, 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m INNER JOIN collection_media cm ON m\.slug = cm\.media_slug LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug WHERE cm\.collection_id = \? ORDER BY vote_scores\.score DESC, cm\.added_at DESC LIMIT 4`).
		WithArgs(1).
		WillReturnRows(topMediaRows)

	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m INNER JOIN collection_media cm ON m\.slug = cm\.media_slug LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug WHERE cm\.collection_id = \? ORDER BY vote_scores\.score DESC, cm\.added_at DESC LIMIT 4`).
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at"}))

	collections, err := GetCollectionsByUser("testuser", []string{"lib1", "lib2"})
	assert.NoError(t, err)
	assert.Len(t, collections, 2)

	assert.Equal(t, 1, collections[0].ID)
	assert.Equal(t, "Collection 1", collections[0].Name)
	assert.Equal(t, 5, collections[0].MediaCount)

	assert.Equal(t, 2, collections[1].ID)
	assert.Equal(t, "Collection 2", collections[1].Name)
	assert.Equal(t, 3, collections[1].MediaCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllCollections(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "updated_at", "media_count"}).
		AddRow(1, "Public Collection", "A public collection", "user1", 1640995200, 1640995200, 10).
		AddRow(2, "Another Collection", "Another collection", "user2", 1640995300, 1640995300, 7)

	mock.ExpectQuery(`SELECT c\.id, c\.name, c\.description, c\.created_by, c\.created_at, c\.updated_at, COUNT\(cm\.media_slug\) as media_count FROM collections c LEFT JOIN collection_media cm ON c\.id = cm\.collection_id GROUP BY c\.id ORDER BY c\.created_at DESC`).
		WillReturnRows(rows)

	// Mock GetTopMediaInCollection calls for each collection
	topMediaRows := sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at"}).
		AddRow("manga1", "Manga One", "Author One", "Description", 2023, "ja", "manga", "ongoing", "safe", "lib1", "cover.jpg", "/path", 10, 5, 8, 1640995200, 1640995200)

	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m INNER JOIN collection_media cm ON m\.slug = cm\.media_slug LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug WHERE cm\.collection_id = \? ORDER BY vote_scores\.score DESC, cm\.added_at DESC LIMIT 4`).
		WithArgs(1).
		WillReturnRows(topMediaRows)

	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m INNER JOIN collection_media cm ON m\.slug = cm\.media_slug LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug WHERE cm\.collection_id = \? ORDER BY vote_scores\.score DESC, cm\.added_at DESC LIMIT 4`).
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at"}))

	collections, err := GetAllCollections([]string{"lib1", "lib2"})
	assert.NoError(t, err)
	assert.Len(t, collections, 2)

	assert.Equal(t, 1, collections[0].ID)
	assert.Equal(t, "Public Collection", collections[0].Name)
	assert.Equal(t, 10, collections[0].MediaCount)

	assert.Equal(t, 2, collections[1].ID)
	assert.Equal(t, "Another Collection", collections[1].Name)
	assert.Equal(t, 7, collections[1].MediaCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`UPDATE collections SET name = \?, description = \?, updated_at = \? WHERE id = \?`).
		WithArgs("Updated Name", "Updated Description", sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateCollection(1, "Updated Name", "Updated Description")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM collections WHERE id = \?`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteCollection(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionMedia(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, COALESCE\(read_counts\.read_count, 0\) as read_count, COALESCE\(vote_scores\.score, 0\) as vote_score, m\.created_at, m\.updated_at FROM media m INNER JOIN collection_media cm ON m\.slug = cm\.media_slug LEFT JOIN \( SELECT media_slug, COUNT\(\*\) as read_count FROM reading_states GROUP BY media_slug \) read_counts ON m\.slug = read_counts\.media_slug LEFT JOIN \( SELECT media_slug, CASE WHEN COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\) > 0 THEN ROUND\(\(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \* 1\.0 / \(COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\),0\) \+ COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\),0\)\)\) \* 10\) ELSE 0 END as score FROM votes GROUP BY media_slug \) vote_scores ON m\.slug = vote_scores\.media_slug WHERE cm\.collection_id = \? ORDER BY cm\.added_at DESC`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "vote_score", "created_at", "updated_at"}).
			AddRow("test-media", "Test Media", "Author", "Description", 2023, "en", "manga", "ongoing", "safe", "test-lib", "/cover.jpg", "/path", 10, 5, 8, 1640995200, 1640995200))

	media, err := GetCollectionMedia(1, []string{}) // Empty slice means no filtering for test
	assert.NoError(t, err)
	assert.Len(t, media, 1)
	assert.Equal(t, "test-media", media[0].Slug)
	assert.Equal(t, "Test Media", media[0].Name)
	assert.Equal(t, 5, media[0].ReadCount)
	assert.Equal(t, 8, media[0].VoteScore)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAddMediaToCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT OR IGNORE INTO collection_media \(collection_id, media_slug, added_at\) VALUES \(\?, \?, \?\)`).
		WithArgs(1, "test-media", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = AddMediaToCollection(1, "test-media")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveMediaFromCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM collection_media WHERE collection_id = \? AND media_slug = \?`).
		WithArgs(1, "test-media").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = RemoveMediaFromCollection(1, "test-media")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsMediaInCollection(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query - media exists
	rows := sqlmock.NewRows([]string{"count"}).
		AddRow(1)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM collection_media WHERE collection_id = \? AND media_slug = \?`).
		WithArgs(1, "test-media").
		WillReturnRows(rows)

	exists, err := IsMediaInCollection(1, "test-media")
	assert.NoError(t, err)
	assert.True(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsMediaInCollection_NotExists(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM collection_media WHERE collection_id = \? AND media_slug = \?`).
		WithArgs(1, "test-media").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	exists, err := IsMediaInCollection(1, "test-media")
	assert.NoError(t, err)
	assert.False(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchCheckMediaInCollections(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query - media exists in collection 1 but not 2
	rows := sqlmock.NewRows([]string{"collection_id"}).
		AddRow(1)

	mock.ExpectQuery(`SELECT DISTINCT collection_id FROM collection_media WHERE collection_id IN.*AND media_slug = \?`).
		WithArgs(1, 2, "test-media").
		WillReturnRows(rows)

	result, err := BatchCheckMediaInCollections([]int{1, 2}, "test-media")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.True(t, result[1])
	assert.False(t, result[2])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchCheckMediaInCollections_Empty(t *testing.T) {
	result, err := BatchCheckMediaInCollections([]int{}, "test-media")
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}
