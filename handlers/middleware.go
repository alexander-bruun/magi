package handlers

import (
	"net/url"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
)

const (
	sessionTokenDuration = 30 * 24 * time.Hour // 1 month
)

var roleHierarchy = map[string]int{
	"reader":    1,
	"moderator": 2,
	"admin":     3,
}

// AuthMiddleware handles session token validation
func AuthMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Cookies("session_token")

		if sessionToken != "" {
			if err := validateSessionToken(c, sessionToken, requiredRole); err == nil {
				return c.Next()
			}
		}

		originalURL := c.OriginalURL()
		return c.Redirect("/auth/login?target="+url.QueryEscape(originalURL), fiber.StatusSeeOther)
	}
}

func validateSessionToken(c *fiber.Ctx, sessionToken, requiredRole string) error {
	username, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return fiber.ErrUnauthorized
	}

	return validateUserRole(c, username, requiredRole)
}

func validateUserRole(c *fiber.Ctx, userName, requiredRole string) error {
	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return fiber.ErrUnauthorized
	}

	if roleHierarchy[user.Role] < roleHierarchy[requiredRole] {
		return fiber.ErrForbidden
	}

	if user.Banned {
		return fiber.ErrForbidden
	}

	c.Locals("user_name", userName)
	return nil
}

func clearSessionCookie(c *fiber.Ctx) {
	expiredTime := time.Now().Add(-time.Hour)
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  expiredTime,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
}

func setSessionCookie(c *fiber.Ctx, sessionToken string) {
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(sessionTokenDuration),
		MaxAge:   int(sessionTokenDuration.Seconds()),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
}

// isSecureRequest returns true if the request is using HTTPS or forwarded as HTTPS.
func isSecureRequest(c *fiber.Ctx) bool {
	if c.Secure() || c.Protocol() == "https" {
		return true
	}
	// Respect common proxy headers
	if proto := c.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}
	if https := c.Get("X-Forwarded-SSL"); https == "on" || https == "1" {
		return true
	}
	return false
}

// OptionalAuthMiddleware attempts to authenticate a user if session cookie is present
// but does not enforce authentication. It sets c.Locals("user_name") when a valid
// token is found so handlers can optionally adapt views for logged-in users.
func OptionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Cookies("session_token")

		if sessionToken != "" {
			// Try to validate; ignore errors for optional auth
			_ = validateSessionToken(c, sessionToken, "reader")
		}

		return c.Next()
	}
}

// ConditionalAuthMiddleware checks the global configuration to determine
// if authentication is required for viewing manga content. If RequireLoginForContent
// is enabled, it enforces authentication; otherwise, it acts like OptionalAuthMiddleware.
func ConditionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get the app configuration
		cfg, err := models.GetAppConfig()
		if err != nil {
			// If we can't get config, allow access (fail open)
			return c.Next()
		}

		// If login is required for content, enforce authentication
		if cfg.RequireLoginForContent {
			sessionToken := c.Cookies("session_token")

			if sessionToken != "" {
				if err := validateSessionToken(c, sessionToken, "reader"); err == nil {
					return c.Next()
				}
			}

			// No valid authentication - redirect to login
			originalURL := c.OriginalURL()
			return c.Redirect("/auth/login?target="+url.QueryEscape(originalURL), fiber.StatusSeeOther)
		}

		// Login not required, but still try to authenticate if cookie present
		sessionToken := c.Cookies("session_token")
		if sessionToken != "" {
			_ = validateSessionToken(c, sessionToken, "reader")
		}

		return c.Next()
	}
}
