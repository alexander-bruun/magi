package handlers

import (
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// SendError sends a generic error response with HX-Trigger notification for HTMX requests
func SendError(c fiber.Ctx, message string, status string) error {
	triggerNotification(c, message, status)
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusInternalServerError).SendString("")
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// SendValidationError sends a validation error with HX-Trigger notification for HTMX requests
func SendValidationError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusUnprocessableEntity).SendString("")
	}
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"error": message,
	})
}

// SendUnauthorizedError sends an unauthorized error with HX-Trigger notification for HTMX requests
func SendUnauthorizedError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("")
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": message,
	})
}

// SendForbiddenError sends a forbidden error with HX-Trigger notification for HTMX requests
func SendForbiddenError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusForbidden).SendString("")
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": message,
	})
}

// SendConflictError sends a conflict error with HX-Trigger notification for HTMX requests
func SendConflictError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusConflict).SendString("")
	}
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"error": message,
	})
}

// SendNotFoundError sends a not found error with HX-Trigger notification for HTMX requests
func SendNotFoundError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusNotFound).SendString("")
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": message,
	})
}

// SendInternalServerError sends an internal server error with HX-Trigger notification for HTMX requests
func SendInternalServerError(c fiber.Ctx, message string, err error) error {
	// Log the actual error for debugging
	log.Errorf("Internal server error: %v", err)

	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusInternalServerError).SendString("")
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// SendBadRequestError sends a bad request error with HX-Trigger notification for HTMX requests
func SendBadRequestError(c fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusBadRequest).SendString("")
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": message,
	})
}
