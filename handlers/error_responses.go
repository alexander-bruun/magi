package handlers

import (
	"encoding/json"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// triggerNotification triggers an HTMX notification if the request is HTMX
func triggerNotification(c *fiber.Ctx, message string, status string) {
	if IsHTMXRequest(c) {
		notification := map[string]interface{}{
			"showNotification": map[string]string{
				"message": message,
				"status":  status,
			},
		}
		jsonBytes, _ := json.Marshal(notification)
		c.Set("HX-Trigger", string(jsonBytes))
	}
}

// triggerCustomNotification triggers a custom HTMX notification with any event name
func triggerCustomNotification(c *fiber.Ctx, eventName string, data map[string]interface{}) {
	if IsHTMXRequest(c) {
		var notification map[string]interface{}
		if eventName == "" {
			notification = data
		} else {
			notification = map[string]interface{}{
				eventName: data,
			}
		}
		jsonBytes, _ := json.Marshal(notification)
		c.Set("HX-Trigger", string(jsonBytes))
	}
}

// sendError sends a generic error response with HX-Trigger notification for HTMX requests
func sendError(c *fiber.Ctx, message string, status string) error {
	triggerNotification(c, message, status)
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusInternalServerError).SendString("")
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// sendValidationError sends a validation error with HX-Trigger notification for HTMX requests
func sendValidationError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusUnprocessableEntity).SendString("")
	}
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"error": message,
	})
}

// sendUnauthorizedError sends an unauthorized error with HX-Trigger notification for HTMX requests
func sendUnauthorizedError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("")
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": message,
	})
}

// sendForbiddenError sends a forbidden error with HX-Trigger notification for HTMX requests
func sendForbiddenError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusForbidden).SendString("")
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": message,
	})
}

// sendConflictError sends a conflict error with HX-Trigger notification for HTMX requests
func sendConflictError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusConflict).SendString("")
	}
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"error": message,
	})
}

// sendNotFoundError sends a not found error with HX-Trigger notification for HTMX requests
func sendNotFoundError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusNotFound).SendString("")
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": message,
	})
}

// sendInternalServerError sends an internal server error with HX-Trigger notification for HTMX requests
func sendInternalServerError(c *fiber.Ctx, message string, err error) error {
	// Log the actual error for debugging
	log.Errorf("Internal server error: %v", err)

	triggerNotification(c, message, "destructive")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusInternalServerError).SendString("")
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// sendBadRequestError sends a bad request error with HX-Trigger notification for HTMX requests
func sendBadRequestError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if IsHTMXRequest(c) {
		return c.Status(fiber.StatusBadRequest).SendString("")
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": message,
	})
}