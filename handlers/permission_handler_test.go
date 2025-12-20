package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetPermissions(t *testing.T) {
	app := fiber.New()
	app.Get("/permissions", HandleGetPermissions)

	// Test GET request - will redirect due to no HTMX header
	req := httptest.NewRequest("GET", "/permissions", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 302 redirect to /admin/permissions if not HTMX request
	assert.Equal(t, 302, resp.StatusCode)
}

func TestHandleCreatePermission(t *testing.T) {
	app := fiber.New()
	app.Post("/permissions", HandleCreatePermission)

	// Test POST request - will fail due to missing name field
	req := httptest.NewRequest("POST", "/permissions", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 400 due to missing required name field
	assert.Equal(t, 400, resp.StatusCode)
}

func TestHandleUpdatePermission(t *testing.T) {
	app := fiber.New()
	app.Put("/permissions/:id", HandleUpdatePermission)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid permission ID",
			url:            "/permissions/123",
			expectedStatus: 400, // Missing required name field
		},
		{
			name:           "invalid permission ID",
			url:            "/permissions/abc",
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

func TestHandleDeletePermission(t *testing.T) {
	app := fiber.New()
	app.Delete("/permissions/:id", HandleDeletePermission)

	// Test DELETE request - permission doesn't exist in test DB
	req := httptest.NewRequest("DELETE", "/permissions/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Returns 200 OK for successful deletion (even if permission doesn't exist)
	assert.Equal(t, 200, resp.StatusCode)
}