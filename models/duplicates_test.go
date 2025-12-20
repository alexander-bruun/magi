package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateFolderSimilarity(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT OR IGNORE INTO folder_similarities \(library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at\) VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WithArgs("test-lib", "folder-a", "folder-b", 0.85, 0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	similarity := FolderSimilarity{
		LibrarySlug:     "test-lib",
		FolderName1:     "folder-a",
		FolderName2:     "folder-b",
		SimilarityScore: 0.85,
		Dismissed:       false,
	}

	err = CreateFolderSimilarity(similarity)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveFolderSimilarities(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "library_slug", "folder_name_1", "folder_name_2", "similarity_score", "dismissed", "created_at"}).
		AddRow(1, "lib1", "folder-a", "folder-b", 0.9, 0, 1640995200).
		AddRow(2, "lib1", "folder-c", "folder-d", 0.8, 0, 1640995300)

	mock.ExpectQuery(`SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at FROM folder_similarities WHERE dismissed = 0 ORDER BY library_slug, similarity_score DESC`).
		WillReturnRows(rows)

	similarities, err := GetActiveFolderSimilarities()
	assert.NoError(t, err)
	assert.Len(t, similarities, 2)

	assert.Equal(t, int64(1), similarities[0].ID)
	assert.Equal(t, "lib1", similarities[0].LibrarySlug)
	assert.Equal(t, "folder-a", similarities[0].FolderName1)
	assert.Equal(t, "folder-b", similarities[0].FolderName2)
	assert.Equal(t, 0.9, similarities[0].SimilarityScore)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDismissFolderSimilarity(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`UPDATE folder_similarities SET dismissed = 1 WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DismissFolderSimilarity(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClearFolderSimilaritiesForLibrary(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM folder_similarities WHERE library_slug = \?`).
		WithArgs("test-lib").
		WillReturnResult(sqlmock.NewResult(0, 5))

	err = ClearFolderSimilaritiesForLibrary("test-lib")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateMediaDuplicate(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`INSERT OR IGNORE INTO media_duplicates \(media_slug, library_slug, folder_path_1, folder_path_2, dismissed, created_at\) VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WithArgs("test-media", "test-lib", "/path/1", "/path/2", 0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	duplicate := MediaDuplicate{
		MediaSlug:   "test-media",
		LibrarySlug: "test-lib",
		FolderPath1: "/path/1",
		FolderPath2: "/path/2",
		Dismissed:   false,
	}

	err = CreateMediaDuplicate(duplicate)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveMediaDuplicates(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media_duplicates WHERE dismissed = 0`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Mock the main query with JOINs
	rows := sqlmock.NewRows([]string{"id", "media_slug", "manga_name", "library_slug", "library_name", "folder_path_1", "folder_path_2", "dismissed", "created_at"}).
		AddRow(1, "manga1", "Manga One", "lib1", "Library One", "/path1", "/path2", 0, 1640995200).
		AddRow(2, "manga2", "Manga Two", "lib1", "Library One", "/path3", "/path4", 0, 1640995300)

	mock.ExpectQuery(`SELECT md\.id, md\.media_slug, m\.name as manga_name, md\.library_slug, l\.name as library_name, md\.folder_path_1, md\.folder_path_2, md\.dismissed, md\.created_at FROM media_duplicates md LEFT JOIN media m ON md\.media_slug = m\.slug LEFT JOIN libraries l ON md\.library_slug = l\.slug WHERE md\.dismissed = 0 ORDER BY md\.created_at DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	duplicates, total, err := GetActiveMediaDuplicates(1, 10)
	assert.NoError(t, err)
	assert.Len(t, duplicates, 2)
	assert.Equal(t, 2, total)

	assert.Equal(t, int64(1), duplicates[0].ID)
	assert.Equal(t, "manga1", duplicates[0].MediaSlug)
	assert.Equal(t, "Manga One", duplicates[0].MediaName)
	assert.Equal(t, "lib1", duplicates[0].LibrarySlug)
	assert.Equal(t, "Library One", duplicates[0].LibraryName)
	assert.Equal(t, "/path1", duplicates[0].FolderPath1)
	assert.Equal(t, "/path2", duplicates[0].FolderPath2)
	assert.False(t, duplicates[0].Dismissed)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDismissMediaDuplicate(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`UPDATE media_duplicates SET dismissed = 1 WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DismissMediaDuplicate(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClearMediaDuplicatesForLibrary(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM media_duplicates WHERE library_slug = \?`).
		WithArgs("test-lib").
		WillReturnResult(sqlmock.NewResult(0, 3))

	err = ClearMediaDuplicatesForLibrary("test-lib")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteMediaDuplicateByID(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the exec
	mock.ExpectExec(`DELETE FROM media_duplicates WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteMediaDuplicateByID(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveFolderSimilaritiesByLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at FROM folder_similarities WHERE dismissed = 0 ORDER BY library_slug, similarity_score DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "library_slug", "folder_name_1", "folder_name_2", "similarity_score", "dismissed", "created_at",
		}).AddRow(
			1, "lib1", "folder-a", "folder-b", 0.85, 0, 1640995200,
		).AddRow(
			2, "lib1", "folder-c", "folder-d", 0.75, 0, 1640995200,
		).AddRow(
			3, "lib2", "folder-e", "folder-f", 0.90, 0, 1640995200,
		))

	result, err := GetActiveFolderSimilaritiesByLibrary()
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result["lib1"], 2)
	assert.Len(t, result["lib2"], 1)
	assert.Equal(t, "folder-a", result["lib1"][0].FolderName1)
	assert.Equal(t, "folder-e", result["lib2"][0].FolderName1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestoreFolderSimilarity(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE folder_similarities SET dismissed = 0 WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = RestoreFolderSimilarity(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveFolderSimilaritiesWithPagination(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM folder_similarities WHERE dismissed = 0`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	// Mock main query
	mock.ExpectQuery(`SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at FROM folder_similarities WHERE dismissed = 0 ORDER BY library_slug, similarity_score DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "library_slug", "folder_name_1", "folder_name_2", "similarity_score", "dismissed", "created_at",
		}).AddRow(
			1, "lib1", "folder-a", "folder-b", 0.85, 0, 1640995200,
		).AddRow(
			2, "lib1", "folder-c", "folder-d", 0.75, 0, 1640995200,
		))

	result, total, err := GetActiveFolderSimilaritiesWithPagination(1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 2)
	assert.Equal(t, "lib1", result[0].LibrarySlug)
	assert.Equal(t, "folder-a", result[0].FolderName1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveFolderSimilaritiesByLibraryWithPagination(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM folder_similarities WHERE dismissed = 0`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	// Mock main query
	mock.ExpectQuery(`SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at FROM folder_similarities WHERE dismissed = 0 ORDER BY library_slug, similarity_score DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "library_slug", "folder_name_1", "folder_name_2", "similarity_score", "dismissed", "created_at",
		}).AddRow(
			1, "lib1", "folder-a", "folder-b", 0.85, 0, 1640995200,
		).AddRow(
			2, "lib2", "folder-c", "folder-d", 0.75, 0, 1640995200,
		))

	result, total, err := GetActiveFolderSimilaritiesByLibraryWithPagination(1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 2)
	assert.Len(t, result["lib1"], 1)
	assert.Len(t, result["lib2"], 1)
	assert.Equal(t, "folder-a", result["lib1"][0].FolderName1)
	assert.Equal(t, "folder-c", result["lib2"][0].FolderName1)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaDuplicateByFolders(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mangaSlug := "test-manga"
	folderPath1 := "folder-b"
	folderPath2 := "folder-a"

	// Since the function ensures consistent order, folder-a comes first
	mock.ExpectQuery(`SELECT id, media_slug, library_slug, folder_path_1, folder_path_2, dismissed, created_at FROM media_duplicates WHERE media_slug = \? AND folder_path_1 = \? AND folder_path_2 = \?`).
		WithArgs(mangaSlug, "folder-a", "folder-b").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_slug", "library_slug", "folder_path_1", "folder_path_2", "dismissed", "created_at",
		}).AddRow(
			1, mangaSlug, "lib1", "folder-a", "folder-b", 0, 1640995200,
		))

	result, err := GetMediaDuplicateByFolders(mangaSlug, folderPath1, folderPath2)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, mangaSlug, result.MediaSlug)
	assert.Equal(t, "folder-a", result.FolderPath1)
	assert.Equal(t, "folder-b", result.FolderPath2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllMediaDuplicates(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT md\.id, md\.media_slug, m\.name as manga_name, md\.library_slug, l\.name as library_name, md\.folder_path_1, md\.folder_path_2, md\.dismissed, md\.created_at FROM media_duplicates md LEFT JOIN media m ON md\.media_slug = m\.slug LEFT JOIN libraries l ON md\.library_slug = l\.slug ORDER BY md\.created_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_slug", "manga_name", "library_slug", "library_name", "folder_path_1", "folder_path_2", "dismissed", "created_at",
		}).AddRow(
			1, "manga1", "Manga One", "lib1", "Library One", "folder-a", "folder-b", 0, 1640995200,
		).AddRow(
			2, "manga2", "Manga Two", "lib2", "Library Two", "folder-c", "folder-d", 1, 1640995100,
		))

	result, err := GetAllMediaDuplicates()
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, "manga1", result[0].MediaSlug)
	assert.Equal(t, "Manga One", result[0].MediaName)
	assert.Equal(t, "Library One", result[0].LibraryName)
	assert.Equal(t, false, result[0].Dismissed)
	assert.Equal(t, true, result[1].Dismissed)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDuplicateFolderInfo(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT md\.id, md\.media_slug, m\.name as manga_name, md\.folder_path_1, md\.folder_path_2 FROM media_duplicates md LEFT JOIN media m ON md\.media_slug = m\.slug WHERE md\.id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_slug", "manga_name", "folder_path_1", "folder_path_2",
		}).AddRow(
			1, "manga1", "Manga One", "/path/to/folder1", "/path/to/folder2",
		))

	result, err := GetDuplicateFolderInfo(1)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.DuplicateID)
	assert.Equal(t, "manga1", result.MediaSlug)
	assert.Equal(t, "Manga One", result.MediaName)
	assert.Equal(t, "/path/to/folder1", result.Folder1.Path)
        assert.Equal(t, "/path/to/folder2", result.Folder2.Path)

        assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteDuplicateFolder(t *testing.T) {
        // Create mock DB
        mockDB, mock, err := sqlmock.New()
        assert.NoError(t, err)
        defer mockDB.Close()

        // Replace global db
        originalDB := db
        db = mockDB
        defer func() { db = originalDB }()

        // Create a temporary directory for testing
        tempDir, err := os.MkdirTemp("", "test_duplicate_*")
        assert.NoError(t, err)
        defer os.RemoveAll(tempDir) // Clean up

        folderPath := filepath.Join(tempDir, "folder1")

        // Mock the SELECT query to get duplicate info
        mock.ExpectQuery(`SELECT folder_path_1, folder_path_2, media_slug FROM media_duplicates WHERE id = \?`).
                WithArgs(int64(1)).
                WillReturnRows(sqlmock.NewRows([]string{"folder_path_1", "folder_path_2", "media_slug"}).
                        AddRow(folderPath, "/path/to/folder2", "test-manga"))

        // Mock DeleteMediaDuplicateByID
        mock.ExpectExec(`DELETE FROM media_duplicates WHERE id = \?`).
                WithArgs(int64(1)).
                WillReturnResult(sqlmock.NewResult(0, 1))

        // Mock GetMediaUnfiltered - return no rows since path doesn't match
        mock.ExpectQuery(`SELECT slug, name, author, description, year, original_language, type, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM media WHERE slug = \?`).
                WithArgs("test-manga").
                WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at"}))

        err = DeleteDuplicateFolder(1, folderPath)
        assert.NoError(t, err)

        assert.NoError(t, mock.ExpectationsWereMet())
}