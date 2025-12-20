package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetNotifications(t *testing.T) {
	app := fiber.New()
	app.Get("/notifications", HandleGetNotifications)

	// Test GET request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("GET", "/notifications", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleGetUnreadCount(t *testing.T) {
	app := fiber.New()
	app.Get("/notifications/unread-count", HandleGetUnreadCount)

	// Test GET request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("GET", "/notifications/unread-count", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleMarkNotificationRead(t *testing.T) {
	app := fiber.New()
	app.Post("/notifications/:id/read", HandleMarkNotificationRead)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/notifications/123/read", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleMarkAllNotificationsRead(t *testing.T) {
	app := fiber.New()
	app.Post("/notifications/mark-all-read", HandleMarkAllNotificationsRead)

	// Test POST request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("POST", "/notifications/mark-all-read", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleDeleteNotification(t *testing.T) {
	app := fiber.New()
	app.Delete("/notifications/:id", HandleDeleteNotification)

	// Test DELETE request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("DELETE", "/notifications/123", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandleClearReadNotifications(t *testing.T) {
	app := fiber.New()
	app.Delete("/notifications/clear-read", HandleClearReadNotifications)

	// Test DELETE request - will fail due to auth/DB but tests routing
	req := httptest.NewRequest("DELETE", "/notifications/clear-read", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 401 due to no auth, but tests the handler exists
	assert.Equal(t, 401, resp.StatusCode)
}