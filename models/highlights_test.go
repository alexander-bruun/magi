package models

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Create a temporary directory for test database
	tempDir, err := os.MkdirTemp("", "magi_test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	// Find project root from test file location
	_, testFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(testFile)) // Go up one level from models/ to project root

	// Change to project root directory so migrations can be found
	err = os.Chdir(projectRoot)
	if err != nil {
		panic(err)
	}

	// Initialize test database with migrations to create all tables
	err = InitializeWithMigration(tempDir, true)
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Close database
	Close()

	os.Exit(code)
}

func TestCreateHighlight(t *testing.T) {
	// Clean up any existing data
	db.Exec("DELETE FROM highlights")
	db.Exec("DELETE FROM media")

	// Create a test media first
	media := &Media{
		Slug:        "test-manga",
		Name:        "Test Manga",
		Description: "Test description",
		CoverArtURL: "/test/cover.jpg",
		Author:      "Test Author",
		Type:        "manga",
		Status:      "ongoing",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path",
	}
	err := CreateMedia(*media)
	assert.NoError(t, err)

	// Test creating a highlight
	highlight, err := CreateHighlight("test-manga", "/test/background.jpg", "Test description", 1)
	assert.NoError(t, err)
	assert.NotNil(t, highlight)
	assert.Equal(t, "test-manga", highlight.MediaSlug)
	assert.Equal(t, "/test/background.jpg", highlight.BackgroundImageURL)
	assert.Equal(t, "Test description", highlight.Description)
	assert.Equal(t, 1, highlight.DisplayOrder)
	assert.NotZero(t, highlight.CreatedAt)
	assert.NotZero(t, highlight.UpdatedAt)
}

func TestGetHighlights(t *testing.T) {
	// Clean up any existing data
	db.Exec("DELETE FROM highlights")
	db.Exec("DELETE FROM media")

	// Create test media
	media1 := &Media{
		Slug:        "test-manga-1",
		Name:        "Test Manga 1",
		Description: "Test description 1",
		CoverArtURL: "/test/cover1.jpg",
		Author:      "Test Author 1",
		Type:        "manga",
		Status:      "ongoing",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path1",
	}
	err := CreateMedia(*media1)
	assert.NoError(t, err)

	media2 := &Media{
		Slug:        "test-manga-2",
		Name:        "Test Manga 2",
		Description: "Test description 2",
		CoverArtURL: "/test/cover2.jpg",
		Author:      "Test Author 2",
		Type:        "manga",
		Status:      "completed",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path2",
	}
	err = CreateMedia(*media2)
	assert.NoError(t, err)

	// Create highlights
	_, err = CreateHighlight("test-manga-1", "/test/background1.jpg", "Test description 1", 2)
	assert.NoError(t, err)
	_, err = CreateHighlight("test-manga-2", "/test/background2.jpg", "Test description 2", 1)
	assert.NoError(t, err)

	// Test getting highlights
	highlights, err := GetHighlights()
	assert.NoError(t, err)
	assert.Len(t, highlights, 2)

	// Should be ordered by display_order
	assert.Equal(t, "test-manga-2", highlights[0].Highlight.MediaSlug)
	assert.Equal(t, "test-manga-1", highlights[1].Highlight.MediaSlug)
}

func TestUpdateHighlight(t *testing.T) {
	// Clean up any existing data
	db.Exec("DELETE FROM highlights")
	db.Exec("DELETE FROM media")

	// Create test media
	media := &Media{
		Slug:        "test-manga",
		Name:        "Test Manga",
		Description: "Test description",
		CoverArtURL: "/test/cover.jpg",
		Author:      "Test Author",
		Type:        "manga",
		Status:      "ongoing",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path",
	}
	err := CreateMedia(*media)
	assert.NoError(t, err)

	// Create highlight
	highlight, err := CreateHighlight("test-manga", "/test/background.jpg", "Test description", 1)
	assert.NoError(t, err)

	// Update highlight
	err = UpdateHighlight(highlight.ID, "test-manga", "/test/new-background.jpg", "Updated description", 2)
	assert.NoError(t, err)

	// Verify update
	updatedHighlight, err := GetHighlightByID(highlight.ID)
	assert.NoError(t, err)
	assert.Equal(t, "/test/new-background.jpg", updatedHighlight.BackgroundImageURL)
	assert.Equal(t, "Updated description", updatedHighlight.Description)
	assert.Equal(t, 2, updatedHighlight.DisplayOrder)
}

func TestDeleteHighlight(t *testing.T) {
	// Clean up any existing data
	db.Exec("DELETE FROM highlights")
	db.Exec("DELETE FROM media")

	// Create test media
	media := &Media{
		Slug:        "test-manga",
		Name:        "Test Manga",
		Description: "Test description",
		CoverArtURL: "/test/cover.jpg",
		Author:      "Test Author",
		Type:        "manga",
		Status:      "ongoing",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path",
	}
	err := CreateMedia(*media)
	assert.NoError(t, err)

	// Create highlight
	highlight, err := CreateHighlight("test-manga", "/test/background.jpg", "Test description", 1)
	assert.NoError(t, err)

	// Delete highlight
	err = DeleteHighlight(highlight.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = GetHighlightByID(highlight.ID)
	assert.Error(t, err)
}

func TestIsMediaHighlighted(t *testing.T) {
	// Clean up any existing data
	db.Exec("DELETE FROM highlights")
	db.Exec("DELETE FROM media")

	// Create test media
	media := &Media{
		Slug:        "test-manga",
		Name:        "Test Manga",
		Description: "Test description",
		CoverArtURL: "/test/cover.jpg",
		Author:      "Test Author",
		Type:        "manga",
		Status:      "ongoing",
		ContentRating: "safe",
		LibrarySlug: "test-library",
		Path:        "/test/path",
	}
	err := CreateMedia(*media)
	assert.NoError(t, err)

	// Test before highlighting
	highlighted, err := IsMediaHighlighted("test-manga")
	assert.NoError(t, err)
	assert.False(t, highlighted)

	// Create highlight
	_, err = CreateHighlight("test-manga", "/test/background.jpg", "Test description", 1)
	assert.NoError(t, err)

	// Test after highlighting
	highlighted, err = IsMediaHighlighted("test-manga")
	assert.NoError(t, err)
	assert.True(t, highlighted)
}