package handlers

import (
	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

// MaintenanceModeMiddleware checks if maintenance mode is enabled and shows maintenance page if needed
// Admins can still access the site even during maintenance mode
func MaintenanceModeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if user is authenticated as admin - admins bypass maintenance mode
		// First check if user_name is set in locals (from auth middleware)
		userName, ok := c.Locals("user_name").(string)
		if ok && userName != "" {
			// User is authenticated, get their role
			user, err := models.FindUserByUsername(userName)
			if err == nil && user != nil && user.Role == "admin" {
				return c.Next()
			}
		}

		// Check if maintenance mode is enabled
		enabled, message, err := models.GetMaintenanceStatus()
		if err != nil {
			// If we can't check maintenance status, allow the request
			return c.Next()
		}

		if !enabled {
			// Maintenance mode is not enabled, proceed normally
			return c.Next()
		}

		// Maintenance mode is enabled - show maintenance page
		return adaptor.HTTPHandler(templ.Handler(views.MaintenancePage(message)))(c)
	}
}
