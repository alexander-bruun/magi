package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleAvatarRequest(t *testing.T) {
	app := fiber.New()
	app.Get("/avatar/:path", handleAvatarRequest)

	// Test with a path
	req := httptest.NewRequest("GET", "/avatar/test.jpg", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Should return 500 since data backend is not initialized
	assert.Equal(t, 500, resp.StatusCode)
}

func TestHandlePosterRequest(t *testing.T) {
	app := fiber.New()
	app.Get("/poster/:path", handlePosterRequest)

	// Test with a path
	req := httptest.NewRequest("GET", "/poster/test.jpg", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Should return 500 since data backend is not initialized
	assert.Equal(t, 500, resp.StatusCode)
}

func TestImageHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/image", ImageHandler)

	// Test without proper parameters
	req := httptest.NewRequest("GET", "/image", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Should return 400 or similar
	assert.NotEqual(t, 200, resp.StatusCode)
}
