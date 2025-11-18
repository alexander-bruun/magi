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

// ConditionalAuthMiddleware attempts to authenticate a user if session cookie is present,
// and falls back to anonymous role permissions for unauthenticated users.
// If anonymous users have no permissions, it enforces authentication.
func ConditionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to authenticate if session cookie is present
		sessionToken := c.Cookies("session_token")
		if sessionToken != "" {
			if err := validateSessionToken(c, sessionToken, "reader"); err == nil {
				return c.Next()
			}
		}

		// No authenticated user - check if anonymous users have any permissions
		// If they do, allow access; otherwise redirect to login
		libraries, err := models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			// If we can't get anonymous permissions, fail open
			return c.Next()
		}

		// If anonymous has no permissions, require authentication
		if len(libraries) == 0 {
			originalURL := c.OriginalURL()
			return c.Redirect("/auth/login?target="+url.QueryEscape(originalURL), fiber.StatusSeeOther)
		}

		// Anonymous has permissions, allow access
		return c.Next()
	}
}

// GetCurrentUsername retrieves the username from the fiber context
func GetCurrentUsername(c *fiber.Ctx) string {
	username, ok := c.Locals("user_name").(string)
	if !ok {
		return ""
	}
	return username
}

// GetUserAccessibleLibraries returns the library slugs accessible to the current user
// Returns libraries based on role permissions for authenticated users or anonymous permissions for unauthenticated users
func GetUserAccessibleLibraries(c *fiber.Ctx) ([]string, error) {
	username := GetCurrentUsername(c)
	
	// If no user is authenticated, return anonymous role permissions
	if username == "" {
		return models.GetAccessibleLibrariesForAnonymous()
	}
	
	// Check user role
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return []string{}, err
	}
	
	// Admins and moderators have access to all libraries
	if user.Role == "admin" || user.Role == "moderator" {
		libraries, err := models.GetLibraries()
		if err != nil {
			return nil, err
		}
		
		slugs := make([]string, len(libraries))
		for i, lib := range libraries {
			slugs[i] = lib.Slug
		}
		return slugs, nil
	}
	
	// Regular users - get accessible libraries based on permissions
	return models.GetAccessibleLibrariesForUser(username)
}

// UserHasLibraryAccess checks if the current user has access to a specific library
func UserHasLibraryAccess(c *fiber.Ctx, librarySlug string) (bool, error) {
	username := GetCurrentUsername(c)
	
	// If no user is authenticated, check anonymous role permissions
	if username == "" {
		return models.AnonymousHasLibraryAccess(librarySlug)
	}
	
	// Check user role
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return false, err
	}
	
	// Admins and moderators have access to all libraries
	if user.Role == "admin" || user.Role == "moderator" {
		return true, nil
	}
	
	// Regular users - check permissions
	return models.UserHasLibraryAccess(username, librarySlug)
}
