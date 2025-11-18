package handlers

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

const (
	sessionTokenCookie = "session_token"
)

// HandleView wraps a page component with the layout unless the request is an HTMX fragment.
func HandleView(c *fiber.Ctx, content templ.Component) error {
	// Check if this is an HTMX request but NOT a history restore request
	// History restore requests should get the full layout
	isHistoryRestore := c.Get("HX-History-Restore-Request") == "true"
	
	if IsHTMXRequest(c) && !isHistoryRestore {
		return renderComponent(c, content)
	}

	userRole, err := getUserRole(c)
	if err != nil {
		// Log the error, but continue with an empty user role
		// This allows the page to render for non-authenticated users
		log.Errorf("Error getting user role: %v", err)
	}

	// pass current request path so templates can mark active nav items
	base := views.Layout(content, userRole, c.Path())
	return renderComponent(c, base)
}

// HandleHome renders the landing page with recent manga activity and aggregate stats.
func HandleHome(c *fiber.Ctx) error {
	return HandleView(c, views.Home())
}

// HandleNotFound renders the generic not-found page for unrouted paths.
func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

// Helper functions

func renderComponent(c *fiber.Ctx, component templ.Component) error {
	// Preserve the status code if it was already set
	statusCode := c.Response().StatusCode()
	if statusCode == 0 {
		statusCode = fiber.StatusOK
	}
	
	handler := adaptor.HTTPHandler(templ.Handler(component))
	err := handler(c)
	
	// Restore the status code after rendering
	c.Status(statusCode)
	return err
}

func getUserRole(c *fiber.Ctx) (string, error) {
	sessionToken := c.Cookies(sessionTokenCookie)
	if sessionToken == "" {
		return "", nil
	}

	userName, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return "", fmt.Errorf("invalid session token")
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %s", userName)
	}
	if user == nil {
		return "", fmt.Errorf("user not found: %s", userName)
	}

	return user.Role, nil
}

// handleError renders an error view with an appropriate HTTP status code
func handleError(c *fiber.Ctx, err error) error {
	return handleErrorWithStatus(c, err, fiber.StatusInternalServerError)
}

// handleErrorWithStatus renders an error view with a custom HTTP status code
func handleErrorWithStatus(c *fiber.Ctx, err error, status int) error {
	c.Status(status)
	return HandleView(c, views.ErrorWithStatus(status, err.Error()))
}
