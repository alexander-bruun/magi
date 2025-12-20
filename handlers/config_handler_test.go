package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleConfiguration(t *testing.T) {
	app := fiber.New()
	app.Get("/config", HandleConfiguration)

	// Test GET request - returns config page
	req := httptest.NewRequest("GET", "/config", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleConfigurationUpdate(t *testing.T) {
	app := fiber.New()
	app.Post("/config", HandleConfigurationUpdate)

	// Test POST request - will fail due to auth
	req := httptest.NewRequest("POST", "/config", strings.NewReader("app_name=Test App"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // Returns config page
}

func TestHandleConsoleLogsWebSocketUpgrade(t *testing.T) {
	app := fiber.New()
	app.Get("/config/console-logs", HandleConsoleLogsWebSocketUpgrade)

	// Test GET request - will fail due to WebSocket upgrade requirements but tests routing
	req := httptest.NewRequest("GET", "/config/console-logs", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Will return 426 due to WebSocket upgrade required
	assert.Equal(t, 426, resp.StatusCode)
}