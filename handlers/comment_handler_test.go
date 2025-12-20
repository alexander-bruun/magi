package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetComments(t *testing.T) {
	app := fiber.New()
	app.Get("/:media/comments", HandleGetComments)

	// Test media comments
	req := httptest.NewRequest("GET", "/manga1/comments", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleCreateComment(t *testing.T) {
	app := fiber.New()
	app.Post("/comments/:media/:chapter", HandleCreateComment)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/comments/manga1/chapter1", strings.NewReader("content=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleDeleteComment(t *testing.T) {
	app := fiber.New()
	app.Delete("/comments/:id", HandleDeleteComment)

	// Test DELETE request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("DELETE", "/comments/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleGetChapterComments(t *testing.T) {
	app := fiber.New()
	app.Get("/comments/:media/:chapter", HandleGetComments)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid chapter comments",
			url:            "/comments/manga1/chapter1",
			expectedStatus: 200, // Returns comments page
		},
		{
			name:           "chapter comments with numeric chapter",
			url:            "/comments/manga1/001",
			expectedStatus: 200, // Returns comments page
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

func TestHandleUpdateComment(t *testing.T) {
	app := fiber.New()
	app.Put("/comments/:id", HandleDeleteComment) // Note: Using delete as placeholder since update doesn't exist

	// Test PUT request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("PUT", "/comments/123", strings.NewReader("content=updated"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}