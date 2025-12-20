package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleUpdateMetadataMedia(t *testing.T) {
	t.Skip("Skipping metadata handler test - requires HTTP client mocking for external API calls")
	
	app := fiber.New()
	app.Post("/metadata/:media/update", HandleUpdateMetadataMedia)

	// Test POST request - should work with database setup and search parameter
	req := httptest.NewRequest("POST", "/metadata/manga1/update?search=test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Should return 200 OK now that database is available and search parameter is provided
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleEditMetadataMedia(t *testing.T) {
	app := fiber.New()
	app.Get("/metadata/:media/edit", HandleEditMetadataMedia)

	// Test GET request - will fail due to missing slug parameter
	req := httptest.NewRequest("GET", "/metadata/manga1/edit", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 400 due to missing required slug parameter
	assert.Equal(t, 400, resp.StatusCode)
}

func TestHandleManualEditMetadata(t *testing.T) {
	app := fiber.New()
	app.Post("/metadata/:media/manual-edit", HandleManualEditMetadata)

	// Test POST request - media doesn't exist in test DB
	req := httptest.NewRequest("POST", "/metadata/manga1/manual-edit", strings.NewReader("title=New Title"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Returns 404 because media doesn't exist
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandleReindexChapters(t *testing.T) {
	app := fiber.New()
	app.Post("/metadata/:media/reindex-chapters", HandleReindexChapters)

	// Test POST request - media doesn't exist in test DB
	req := httptest.NewRequest("POST", "/metadata/manga1/reindex-chapters", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Returns 404 because media doesn't exist
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandleRefreshMetadata(t *testing.T) {
	app := fiber.New()
	app.Post("/metadata/:media/refresh", HandleRefreshMetadata)

	// Test POST request - media doesn't exist in test DB
	req := httptest.NewRequest("POST", "/metadata/manga1/refresh", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Returns 404 because media doesn't exist
	assert.Equal(t, 404, resp.StatusCode)
}