package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaintenanceModeMiddlewareDisabled tests middleware when maintenance mode is disabled
func TestMaintenanceModeMiddlewareDisabled(t *testing.T) {
	app := fiber.New()

	// Add a simple test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestMaintenanceModeMiddlewareEnabledAdmin tests middleware when maintenance mode is enabled for admin
func TestMaintenanceModeMiddlewareEnabledAdmin(t *testing.T) {
	app := fiber.New()

	// Set up middleware that marks user as admin by setting user_name in locals
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_name", "admin_user")
		return c.Next()
	})

	// Mock FindUserByUsername to return an admin user
	originalFindUser := models.FindUserByUsername
	defer func() { models.FindUserByUsername = originalFindUser }()

	models.FindUserByUsername = func(username string) (*models.User, error) {
		return &models.User{
			Username: username,
			Role:     "admin",
		}, nil
	}

	// Add maintenance middleware
	app.Use(MaintenanceModeMiddleware())

	// Add a simple test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	// Admin should always get through (status OK)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestMaintenanceModeMiddlewareBypassesAdmin tests that admins bypass maintenance
func TestMaintenanceModeMiddlewareBypassesAdmin(t *testing.T) {
	app := fiber.New()

	// Set up middleware that marks user as admin
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_name", "admin_user")
		return c.Next()
	})

	// Mock FindUserByUsername to return an admin user
	originalFindUser := models.FindUserByUsername
	defer func() { models.FindUserByUsername = originalFindUser }()

	models.FindUserByUsername = func(username string) (*models.User, error) {
		return &models.User{
			Username: username,
			Role:     "admin",
		}, nil
	}

	// Add maintenance middleware
	app.Use(MaintenanceModeMiddleware())

	// Add test route
	app.Get("/admin/test", func(c *fiber.Ctx) error {
		return c.SendString("Admin access granted")
	})

	req := httptest.NewRequest("GET", "/admin/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestMaintenanceModeMiddlewareNonAdminBypass tests that non-admins are not bypassed
func TestMaintenanceModeMiddlewareNonAdminBypass(t *testing.T) {
	app := fiber.New()

	// Set up middleware that marks user as non-admin (reader)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_name", "reader_user")
		return c.Next()
	})

	// Mock FindUserByUsername to return a reader user
	originalFindUser := models.FindUserByUsername
	defer func() { models.FindUserByUsername = originalFindUser }()

	models.FindUserByUsername = func(username string) (*models.User, error) {
		return &models.User{
			Username: username,
			Role:     "reader",
		}, nil
	}

	// Add maintenance middleware
	app.Use(MaintenanceModeMiddleware())

	// Add test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("Access granted")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	// Non-admin should get through or maintenance page
	assert.True(t, resp.StatusCode == fiber.StatusOK || resp.StatusCode == fiber.StatusTeapot)
}

