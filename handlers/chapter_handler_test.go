package handlers

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/alexander-bruun/magi/models"
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
	projectRoot := filepath.Dir(filepath.Dir(testFile)) // Go up two levels from handlers/ to project root

	// Change to project root directory so migrations can be found
	err = os.Chdir(projectRoot)
	if err != nil {
		panic(err)
	}

	// Debug: print current working directory
	cwd, _ := os.Getwd()
	println("TestMain CWD:", cwd)

	// Initialize test database with migrations to create all tables
	err = models.InitializeWithMigration(tempDir, true)
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Close database
	models.Close()

	os.Exit(code)
}

func TestIsChapterAccessible(t *testing.T) {
	// Test cases
	tests := []struct {
		name       string
		chapter    *models.Chapter
		userName   string
		expected   bool
	}{
		{
			name: "released chapter accessible to everyone",
			chapter: &models.Chapter{
				ReleasedAt: &time.Time{}, // Set to some time
				IsPremium:  false,
			},
			userName: "",
			expected: true,
		},
		{
			name: "non-premium chapter accessible to anonymous",
			chapter: &models.Chapter{
				ReleasedAt: nil,
				IsPremium:  false,
			},
			userName: "",
			expected: true,
		},
		{
			name: "non-premium chapter accessible to logged user",
			chapter: &models.Chapter{
				ReleasedAt: nil,
				IsPremium:  false,
			},
			userName: "testuser",
			expected: true,
		},
		{
			name: "premium chapter not accessible to anonymous without access",
			chapter: &models.Chapter{
				ReleasedAt: nil,
				IsPremium:  true,
			},
			userName: "",
			expected: false, // This will depend on RoleHasAccess implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChapterAccessible(tt.chapter, tt.userName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleChapterInvalidMediaSlug(t *testing.T) {
	app := fiber.New()
	app.Get("/chapter/:media/:chapter", HandleChapter)

	// Test with invalid media slug containing comma
	req := httptest.NewRequest("GET", "/chapter/media,invalid/chapter1", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode) // Bad Request
}

func TestHandleMarkRead(t *testing.T) {
	app := fiber.New()
	app.Post("/chapter/:media/:chapter/mark-read", HandleMarkRead)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/chapter/manga1/chapter1/mark-read", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleMarkUnread(t *testing.T) {
	app := fiber.New()
	app.Post("/chapter/:media/:chapter/mark-unread", HandleMarkUnread)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/chapter/manga1/chapter1/mark-unread", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleMediaChapterTOC(t *testing.T) {
	app := fiber.New()
	app.Get("/chapter/:media/:chapter/toc", HandleMediaChapterTOC)

	// Test GET request - media doesn't exist so returns 404
	req := httptest.NewRequest("GET", "/chapter/manga1/chapter1/toc", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandleChapter(t *testing.T) {
	app := fiber.New()
	app.Get("/chapter/:media/:chapter", HandleChapter)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid chapter request",
			url:            "/chapter/manga1/chapter1",
			expectedStatus: 500, // DB error expected
		},
		{
			name:           "chapter with special characters",
			url:            "/chapter/manga-1/chapter-001",
			expectedStatus: 500, // DB error expected
		},
		{
			name:           "empty media slug",
			url:            "/chapter//chapter1",
			expectedStatus: 404, // Not found due to empty param
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestHandleChapterImage(t *testing.T) {
	app := fiber.New()
	app.Get("/chapter/:media/:chapter/:page", HandleMediaChapterAsset)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid chapter image request",
			url:            "/chapter/manga1/chapter1/1",
			expectedStatus: 400, // Missing token parameter
		},
		{
			name:           "chapter image with page 0",
			url:            "/chapter/manga1/chapter1/0",
			expectedStatus: 400, // Missing token parameter
		},
		{
			name:           "invalid page number",
			url:            "/chapter/manga1/chapter1/abc",
			expectedStatus: 400, // Missing token parameter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}