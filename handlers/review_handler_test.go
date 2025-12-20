package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetReviews(t *testing.T) {
	app := fiber.New()
	app.Get("/:media/reviews", HandleGetReviews)

	// Test GET request - media doesn't exist in test DB
	req := httptest.NewRequest("GET", "/manga1/reviews", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Returns 200 OK with empty reviews list for non-existent media
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleCreateReview(t *testing.T) {
	app := fiber.New()
	app.Post("/:media/reviews", HandleCreateReview)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/manga1/reviews", strings.NewReader("rating=5&content=Great manga!"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleGetUserReview(t *testing.T) {
	app := fiber.New()
	app.Get("/:media/reviews/user", HandleGetUserReview)

	// Test GET request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("GET", "/manga1/reviews/user", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleDeleteReview(t *testing.T) {
	app := fiber.New()
	app.Delete("/reviews/:id", HandleDeleteReview)

	// Test DELETE request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("DELETE", "/reviews/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}