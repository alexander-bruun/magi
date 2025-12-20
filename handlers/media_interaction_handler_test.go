package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleMediaVote(t *testing.T) {
	app := fiber.New()
	app.Post("/media/:media/vote", HandleMediaVote)

	tests := []struct {
		name           string
		url            string
		body           string
		expectedStatus int
	}{
		{
			name:           "upvote media",
			url:            "/media/manga1/vote",
			body:           "vote=up",
			expectedStatus: 401, // Auth required
		},
		{
			name:           "downvote media",
			url:            "/media/manga1/vote",
			body:           "vote=down",
			expectedStatus: 401, // Auth required
		},
		{
			name:           "remove vote",
			url:            "/media/manga1/vote",
			body:           "vote=remove",
			expectedStatus: 401, // Auth required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestHandleMediaVoteFragment(t *testing.T) {
	app := fiber.New()
	app.Get("/media/:media/vote-fragment", HandleMediaVoteFragment)

	// Test GET request - will redirect due to no HTMX header
	req := httptest.NewRequest("GET", "/media/manga1/vote-fragment", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 302 redirect to media page
	assert.Equal(t, 302, resp.StatusCode)
}

func TestHandleMediaFavorite(t *testing.T) {
	app := fiber.New()
	app.Post("/media/:media/favorite", HandleMediaFavorite)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/media/manga1/favorite", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleMediaFavoriteFragment(t *testing.T) {
	app := fiber.New()
	app.Get("/media/:media/favorite-fragment", HandleMediaFavoriteFragment)

	// Test GET request - will redirect due to no HTMX header
	req := httptest.NewRequest("GET", "/media/manga1/favorite-fragment", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 302 redirect to media page
	assert.Equal(t, 302, resp.StatusCode)
}