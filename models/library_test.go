package models

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestLibrary_GetFolderNames(t *testing.T) {
	tests := []struct {
		name     string
		folders  []string
		expected string
	}{
		{
			name:     "empty folders",
			folders:  []string{},
			expected: "",
		},
		{
			name:     "single folder",
			folders:  []string{"folder1"},
			expected: "folder1",
		},
		{
			name:     "multiple folders",
			folders:  []string{"folder1", "folder2", "folder3"},
			expected: "folder1, folder2, folder3",
		},
		{
			name:     "folders with spaces",
			folders:  []string{"my folder", "another folder"},
			expected: "my folder, another folder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Library{Folders: tt.folders}
			result := l.GetFolderNames()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLibrary_Validate(t *testing.T) {
	tests := []struct {
		name        string
		library     Library
		expectError bool
		errorMsg    string
		expectedSlug string
	}{
		{
			name: "valid library",
			library: Library{
				Name:        "Test Library",
				Description: "A test library",
				Cron:        "0 0 * * *",
			},
			expectError:  false,
			expectedSlug: "test-library",
		},
		{
			name: "empty name",
			library: Library{
				Name:        "",
				Description: "A test library",
				Cron:        "0 0 * * *",
			},
			expectError: true,
			errorMsg:    "library name cannot be empty",
		},
		{
			name: "empty description",
			library: Library{
				Name:        "Test Library",
				Description: "",
				Cron:        "0 0 * * *",
			},
			expectError: true,
			errorMsg:    "library description cannot be empty",
		},
		{
			name: "empty cron",
			library: Library{
				Name:        "Test Library",
				Description: "A test library",
				Cron:        "",
			},
			expectError: true,
			errorMsg:    "library cron cannot be empty",
		},
		{
			name: "name with special characters",
			library: Library{
				Name:        "Test Library!@#$%",
				Description: "A test library",
				Cron:        "0 0 * * *",
			},
			expectError:  false,
			expectedSlug: "test-library",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := tt.library
			err := l.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSlug, l.Slug)
			}
		})
	}
}

func TestCreateLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	library := Library{
		Name:             "Test Library",
		Description:      "A test library",
		Cron:             "0 0 * * *",
		Folders:          []string{"/path/to/folder1", "/path/to/folder2"},
		MetadataProvider: sql.NullString{String: "mangadex", Valid: true},
	}

	// Mock LibraryExists check - library doesn't exist
	mock.ExpectQuery(`SELECT 1 FROM libraries WHERE slug = \?`).
		WithArgs("test-library").
		WillReturnError(sql.ErrNoRows)

	// Mock the INSERT query
	mock.ExpectExec(`INSERT INTO libraries`).
		WithArgs(
			"test-library", "Test Library", "A test library", "0 0 * * *",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateLibrary(library)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateLibraryAlreadyExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	library := Library{
		Name:        "Existing Library",
		Description: "An existing library",
		Cron:        "0 0 * * *",
	}

	// Mock LibraryExists check - library already exists
	mock.ExpectQuery(`SELECT 1 FROM libraries WHERE slug = \?`).
		WithArgs("existing-library").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	err = CreateLibrary(library)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "library already exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLibraries(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query
	mock.ExpectQuery(`SELECT slug, name, description, cron, folders, metadata_provider, created_at, updated_at FROM libraries`).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "description", "cron", "folders", "metadata_provider", "created_at", "updated_at",
		}).AddRow(
			"test-library", "Test Library", "A test library", "0 0 * * *",
			`["/path/to/folder1","/path/to/folder2"]`, "mangadex", 1609459200, 1704067200,
		).AddRow(
			"another-library", "Another Library", "Another test library", "0 0 * * *",
			`["/path/to/folder3"]`, nil, 1609459200, 1704067200,
		))

	libraries, err := GetLibraries()
	assert.NoError(t, err)
	assert.Len(t, libraries, 2)
	assert.Equal(t, "test-library", libraries[0].Slug)
	assert.Equal(t, "Test Library", libraries[0].Name)
	assert.Equal(t, []string{"/path/to/folder1", "/path/to/folder2"}, libraries[0].Folders)
	assert.Equal(t, "mangadex", libraries[0].MetadataProvider.String)
	assert.Equal(t, "another-library", libraries[1].Slug)
	assert.Equal(t, []string{"/path/to/folder3"}, libraries[1].Folders)
	assert.False(t, libraries[1].MetadataProvider.Valid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query
	mock.ExpectQuery(`SELECT slug, name, description, cron, folders, metadata_provider, created_at, updated_at FROM libraries WHERE slug = \?`).
		WithArgs("test-library").
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "description", "cron", "folders", "metadata_provider", "created_at", "updated_at",
		}).AddRow(
			"test-library", "Test Library", "A test library", "0 0 * * *",
			`["/path/to/folder1","/path/to/folder2"]`, "mangadex", 1609459200, 1704067200,
		))

	library, err := GetLibrary("test-library")
	assert.NoError(t, err)
	assert.NotNil(t, library)
	assert.Equal(t, "test-library", library.Slug)
	assert.Equal(t, "Test Library", library.Name)
	assert.Equal(t, []string{"/path/to/folder1", "/path/to/folder2"}, library.Folders)
	assert.Equal(t, "mangadex", library.MetadataProvider.String)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLibraryNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query - no rows returned
	mock.ExpectQuery(`SELECT slug, name, description, cron, folders, metadata_provider, created_at, updated_at FROM libraries WHERE slug = \?`).
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	library, err := GetLibrary("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, library)
	assert.Contains(t, err.Error(), "library with slug nonexistent not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	library := &Library{
		Slug:             "test-library",
		Name:             "Test Library", // Keep same name so slug doesn't change
		Description:      "An updated test library",
		Cron:             "0 0 * * *",
		Folders:          []string{"/path/to/updated/folder"},
		MetadataProvider: sql.NullString{String: "anilist", Valid: true},
	}

	// Mock the UPDATE query
	mock.ExpectExec(`UPDATE libraries`).
		WithArgs(
			"Test Library", "An updated test library", "0 0 * * *",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "test-library",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateLibrary(library)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteLibrary(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetLibrary query
	mock.ExpectQuery(`SELECT slug, name, description, cron, folders, metadata_provider, created_at, updated_at FROM libraries WHERE slug = \?`).
		WithArgs("test-library").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "description", "cron", "folders", "metadata_provider", "created_at", "updated_at"}).
			AddRow("test-library", "Test Library", "A test library", "0 0 * * *", "[]", nil, 1672531200, 1672531200))

	// Mock DELETE from libraries
	mock.ExpectExec(`DELETE FROM libraries WHERE slug = \?`).
		WithArgs("test-library").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock DeleteMediasByLibrarySlug - SELECT query returns no media
	mock.ExpectQuery(`SELECT slug FROM media WHERE library_slug = \?`).
		WithArgs("test-library").
		WillReturnRows(sqlmock.NewRows([]string{"slug"}))

	err = DeleteLibrary("test-library")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteLibraryNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetLibrary query returning no rows
	mock.ExpectQuery(`SELECT slug, name, description, cron, folders, metadata_provider, created_at, updated_at FROM libraries WHERE slug = \?`).
		WithArgs("nonexistent-library").
		WillReturnRows(sqlmock.NewRows([]string{"slug", "name", "description", "cron", "folders", "metadata_provider", "created_at", "updated_at"}))

	err = DeleteLibrary("nonexistent-library")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "library with slug nonexistent-library not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLibraryExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the EXISTS query - library exists
	mock.ExpectQuery(`SELECT 1 FROM libraries WHERE slug = \?`).
		WithArgs("existing-library").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := LibraryExists("existing-library")
	assert.NoError(t, err)
	assert.True(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLibraryExistsNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the EXISTS query - library doesn't exist
	mock.ExpectQuery(`SELECT 1 FROM libraries WHERE slug = \?`).
		WithArgs("nonexistent-library").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	exists, err := LibraryExists("nonexistent-library")
	assert.NoError(t, err)
	assert.False(t, exists)

	assert.NoError(t, mock.ExpectationsWereMet())
}