package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleHome(t *testing.T) {
	t.Skip("Requires database mocking infrastructure")
	app := fiber.New()
	app.Get("/home", HandleHome)

	req := httptest.NewRequest("GET", "/home", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleNotFound(t *testing.T) {
	app := fiber.New()
	app.Get("/notfound", HandleNotFound)

	req := httptest.NewRequest("GET", "/notfound", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // HandleNotFound renders a page, doesn't set 404 status
}
