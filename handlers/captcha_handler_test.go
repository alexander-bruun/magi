package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleCaptchaNew(t *testing.T) {
	app := fiber.New()
	app.Get("/captcha/new", HandleCaptchaNew)

	req := httptest.NewRequest("GET", "/captcha/new", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)

	// Should have captcha_id field
	captchaID, exists := result["captcha_id"]
	assert.True(t, exists)
	assert.NotEmpty(t, captchaID)
	// captcha.NewLen generates IDs of various lengths, just check it's not empty
	assert.Greater(t, len(captchaID), 0)
}