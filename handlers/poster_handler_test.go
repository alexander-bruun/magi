package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandlePosterChapterSelect(t *testing.T) {
	app := fiber.New()
	app.Get("/poster/:media/chapter-select", HandlePosterChapterSelect)

	// Test GET request - will fail due to DB but tests routing
	req := httptest.NewRequest("GET", "/poster/manga1/chapter-select", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 404 due to media not found
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlePosterSelector(t *testing.T) {
	app := fiber.New()
	app.Get("/poster/:media/selector", HandlePosterSelector)

	// Test GET request - will fail due to DB but tests routing
	req := httptest.NewRequest("GET", "/poster/manga1/selector", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 404 due to media not found
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlePosterPreview(t *testing.T) {
	app := fiber.New()
	app.Get("/poster/:media/preview", HandlePosterPreview)

	// Test GET request - will fail due to DB but tests routing
	req := httptest.NewRequest("GET", "/poster/manga1/preview", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 404 due to media not found
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlePosterSet(t *testing.T) {
	app := fiber.New()
	app.Post("/poster/:media/set", HandlePosterSet)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/poster/manga1/set", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 500 due to cache not initialized
	assert.Equal(t, 500, resp.StatusCode)
}