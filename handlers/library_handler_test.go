package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestValidateLibraryFormData(t *testing.T) {
	tests := []struct {
		name        string
		formData    LibraryFormData
		expectError bool
	}{
		{
			name: "valid data",
			formData: LibraryFormData{
				Name:             "Test Library",
				Description:      "Test description",
				Cron:             "0 0 * * *",
				Folders:          []string{"/tmp"},
				MetadataProvider: "anilist",
			},
			expectError: false,
		},
		{
			name: "invalid cron",
			formData: LibraryFormData{
				Name: "Test Library",
				Cron: "invalid",
			},
			expectError: true,
		},
		{
			name: "non-existent folder",
			formData: LibraryFormData{
				Name:    "Test Library",
				Folders: []string{"/nonexistent"},
			},
			expectError: true,
		},
		{
			name: "file instead of directory",
			formData: LibraryFormData{
				Name:    "Test Library",
				Folders: []string{"/etc/hosts"}, // assuming it's a file
			},
			expectError: true,
		},
		{
			name: "empty folder",
			formData: LibraryFormData{
				Name:    "Test Library",
				Folders: []string{""},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLibraryFormData(tt.formData)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSubdirectories(t *testing.T) {
	// Create a temp directory with subdirs
	tempDir := t.TempDir()

	// Create subdirectories
	subdir1 := filepath.Join(tempDir, "subdir1")
	subdir2 := filepath.Join(tempDir, "subdir2")
	file1 := filepath.Join(tempDir, "file1.txt")

	os.MkdirAll(subdir1, 0755)
	os.MkdirAll(subdir2, 0755)
	os.WriteFile(file1, []byte("test"), 0644)

	// Test
	result, err := getSubdirectories(tempDir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"subdir1", "subdir2"}, result)
}

func TestGetSubdirectoriesNonExistent(t *testing.T) {
	_, err := getSubdirectories("/nonexistent")
	assert.Error(t, err)
}

func TestHandleBrowseDirectory(t *testing.T) {
	// Create a temp directory with some entries
	tempDir := t.TempDir()

	// Create subdir and file
	subdir := filepath.Join(tempDir, "testdir")
	file := filepath.Join(tempDir, "testfile.txt")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(file, []byte("test"), 0644)

	// Create Fiber app and test request
	app := fiber.New()
	app.Get("/browse", HandleBrowseDirectory)

	req := httptest.NewRequest("GET", "/browse?path="+tempDir, nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	var entries []FileEntry
	err = json.NewDecoder(resp.Body).Decode(&entries)
	assert.NoError(t, err)

	// Should have 2 entries: testdir and testfile.txt
	assert.Len(t, entries, 2)

	// Check entries (order may vary)
	entryMap := make(map[string]FileEntry)
	for _, entry := range entries {
		entryMap[entry.Name] = entry
	}

	assert.Contains(t, entryMap, "testdir")
	assert.Contains(t, entryMap, "testfile.txt")

	dirEntry := entryMap["testdir"]
	assert.True(t, dirEntry.IsDir)
	assert.Equal(t, filepath.Join(tempDir, "testdir"), dirEntry.Path)

	fileEntry := entryMap["testfile.txt"]
	assert.False(t, fileEntry.IsDir)
	assert.Equal(t, filepath.Join(tempDir, "testfile.txt"), fileEntry.Path)
}

func TestHandleCancelEdit(t *testing.T) {
	app := fiber.New()
	app.Get("/cancel-edit", HandleCancelEdit)

	t.Run("non-HTMX request redirects", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cancel-edit", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, 302, resp.StatusCode)

		location := resp.Header.Get("Location")
		assert.Equal(t, "/admin/libraries", location)
	})

	t.Run("HTMX request returns form HTML", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cancel-edit", nil)
		req.Header.Set("HX-Request", "true")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		contentType := resp.Header.Get("Content-Type")
		assert.Equal(t, "text/html", contentType)

		// Check that response contains HTML content (basic check)
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		htmlContent := string(body[:n])
		assert.Contains(t, htmlContent, "<") // Basic check for HTML tags
		assert.NotEmpty(t, htmlContent)
	})
}

func TestSetCommonHeaders(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		setCommonHeaders(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "text/html", contentType)
}

func TestRenderLibraryTable(t *testing.T) {
	// Test with empty libraries
	html, err := renderLibraryTable([]models.Library{})
	assert.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "<") // Should contain HTML

	// Test with sample library
	libraries := []models.Library{
		{
			Name:        "Test Library",
			Description: "Test description",
			Folders:     []string{"/path1", "/path2"},
		},
	}
	html, err = renderLibraryTable(libraries)
	assert.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "Test Library")
}

func TestFindDuplicatesInLibrary(t *testing.T) {
	// Create temp directories for testing
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	// Create subdirectories in tempDir1
	subdir1 := filepath.Join(tempDir1, "manga_series")
	subdir2 := filepath.Join(tempDir1, "manga_serie") // similar name
	os.MkdirAll(subdir1, 0755)
	os.MkdirAll(subdir2, 0755)

	// Create subdirectories in tempDir2
	subdir3 := filepath.Join(tempDir2, "different_name")
	os.MkdirAll(subdir3, 0755)

	tests := []struct {
		name        string
		library     models.Library
		threshold   float64
		expectGroups int
	}{
		{
			name: "no folders",
			library: models.Library{
				Folders: []string{},
			},
			threshold:    0.8,
			expectGroups: 0,
		},
		{
			name: "single folder with no duplicates",
			library: models.Library{
				Folders: []string{tempDir2},
			},
			threshold:    0.8,
			expectGroups: 0,
		},
		{
			name: "folders with similar names",
			library: models.Library{
				Folders: []string{tempDir1},
			},
			threshold:    0.8,
			expectGroups: 1, // "manga_series" and "manga_serie" should be grouped
		},
		{
			name: "low threshold no duplicates",
			library: models.Library{
				Folders: []string{tempDir1},
			},
			threshold:    0.95, // Very high threshold
			expectGroups: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := findDuplicatesInLibrary(tt.library, tt.threshold)
			assert.Len(t, groups, tt.expectGroups)
		})
	}
}

