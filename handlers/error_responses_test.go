package handlers

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestTriggerNotification(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request
	c.Request().Header.Set("HX-Request", "true")
	triggerNotification(c, "Test message", "success")

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)

	var notification map[string]interface{}
	err := json.Unmarshal([]byte(hxTrigger), &notification)
	assert.NoError(t, err)

	showNotification, ok := notification["showNotification"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Test message", showNotification["message"])
	assert.Equal(t, "success", showNotification["status"])
}

func TestTriggerNotification_NonHTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test non-HTMX request (no header)
	triggerNotification(c, "Test message", "success")

	hxTrigger := c.Response().Header.Peek("HX-Trigger")
	assert.Empty(t, hxTrigger)
}

func TestTriggerCustomNotification(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request with custom event
	c.Request().Header.Set("HX-Request", "true")
	triggerCustomNotification(c, "customEvent", map[string]interface{}{
		"key": "value",
	})

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)

	var notification map[string]interface{}
	err := json.Unmarshal([]byte(hxTrigger), &notification)
	assert.NoError(t, err)

	customEvent, ok := notification["customEvent"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", customEvent["key"])
}

func TestTriggerCustomNotification_EmptyEventName(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request with empty event name (direct data)
	c.Request().Header.Set("HX-Request", "true")
	triggerCustomNotification(c, "", map[string]interface{}{
		"closeModal": true,
		"showNotification": map[string]string{
			"message": "Test",
			"status":  "success",
		},
	})

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)

	var notification map[string]interface{}
	err := json.Unmarshal([]byte(hxTrigger), &notification)
	assert.NoError(t, err)

	assert.True(t, notification["closeModal"].(bool))
	showNotification := notification["showNotification"].(map[string]interface{})
	assert.Equal(t, "Test", showNotification["message"])
	assert.Equal(t, "success", showNotification["status"])
}

func TestSendValidationError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request
	c.Request().Header.Set("HX-Request", "true")
	err := sendValidationError(c, "Invalid input")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnprocessableEntity, c.Response().StatusCode())
	assert.Empty(t, c.Response().Body())

	// Check notification was triggered
	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendValidationError_API(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test API request (no HX-Request header)
	err := sendValidationError(c, "Invalid input")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnprocessableEntity, c.Response().StatusCode())

	var response map[string]string
	jsonErr := json.Unmarshal(c.Response().Body(), &response)
	assert.NoError(t, jsonErr)
	assert.Equal(t, "Invalid input", response["error"])
}

func TestSendInternalServerError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request
	c.Request().Header.Set("HX-Request", "true")
	testErr := errors.New("database connection failed")
	err := sendInternalServerError(c, "Operation failed", testErr)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, c.Response().StatusCode())
	assert.Empty(t, c.Response().Body())

	// Check notification was triggered
	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendInternalServerError_API(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test API request
	testErr := errors.New("database connection failed")
	err := sendInternalServerError(c, "Operation failed", testErr)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, c.Response().StatusCode())

	var response map[string]string
	jsonErr := json.Unmarshal(c.Response().Body(), &response)
	assert.NoError(t, jsonErr)
	assert.Equal(t, "Operation failed", response["error"])
}

func TestSendUnauthorizedError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendUnauthorizedError(c, "Authentication required")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendForbiddenError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendForbiddenError(c, "Access denied")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendNotFoundError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendNotFoundError(c, "Resource not found")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendConflictError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendConflictError(c, "Resource already exists")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusConflict, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendBadRequestError_HTMX(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendBadRequestError(c, "Invalid request")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)
}

func TestSendError_Generic(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("HX-Request", "true")
	err := sendError(c, "Custom error", "warning")

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, c.Response().StatusCode())

	hxTrigger := string(c.Response().Header.Peek("HX-Trigger"))
	assert.NotEmpty(t, hxTrigger)

	var notification map[string]interface{}
	jsonErr := json.Unmarshal([]byte(hxTrigger), &notification)
	assert.NoError(t, jsonErr)

	showNotification := notification["showNotification"].(map[string]interface{})
	assert.Equal(t, "Custom error", showNotification["message"])
	assert.Equal(t, "warning", showNotification["status"])
}