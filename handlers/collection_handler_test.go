package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleCollectionInvalidID(t *testing.T) {
	app := fiber.New()
	app.Get("/collection/:id", HandleCollection)

	// Test with invalid ID (not a number)
	req := httptest.NewRequest("GET", "/collection/abc", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode) // Bad Request
}

func TestHandleCollectionValidID(t *testing.T) {
	app := fiber.New()
	app.Get("/collection/:id", HandleCollection)

	// Test with valid ID format - collection doesn't exist so returns 404
	req := httptest.NewRequest("GET", "/collection/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandleCollections(t *testing.T) {
	app := fiber.New()
	app.Get("/collections", HandleCollections)

	// Test GET request - returns collections page
	req := httptest.NewRequest("GET", "/collections", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleUserCollections(t *testing.T) {
	app := fiber.New()
	app.Get("/user/collections", HandleUserCollections)

	// Test GET request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("GET", "/user/collections", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleCreateCollection(t *testing.T) {
	app := fiber.New()
	app.Post("/collections", HandleCreateCollection)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/collections", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleUpdateCollection(t *testing.T) {
	app := fiber.New()
	app.Put("/collection/:id", HandleUpdateCollection)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid collection ID",
			url:            "/collection/123",
			expectedStatus: 404, // Collection not found
		},
		{
			name:           "invalid collection ID",
			url:            "/collection/abc",
			expectedStatus: 400, // Invalid ID format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", tt.url, nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestHandleDeleteCollection(t *testing.T) {
	app := fiber.New()
	app.Delete("/collection/:id", HandleDeleteCollection)

	// Test DELETE request - collection doesn't exist
	req := httptest.NewRequest("DELETE", "/collection/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}