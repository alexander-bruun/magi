package handlers

import (
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// RefererValidationMiddleware checks that requests have valid Referer headers
// for internal navigation. This helps prevent direct access to certain endpoints
// by scripts that don't properly simulate browser navigation.
func RefererValidationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for referer validation: %v", err)
			return c.Next()
		}

		// Skip if referer validation is not enabled
		if !cfg.RefererValidationEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Skip for allowed paths (static assets, API auth, health checks, etc.)
		path := c.Path()
		if isRefererExemptPath(path) {
			return c.Next()
		}

		// Skip for GET requests to root/entry points (users can bookmark these)
		if c.Method() == "GET" && isEntryPointPath(path) {
			return c.Next()
		}

		// Get the Referer header
		referer := c.Get("Referer")

		// For non-entry-point pages, require a referer
		if referer == "" {
			// No referer - could be direct access or privacy settings
			// For now, we'll be lenient on GET requests but strict on others
			if c.Method() != "GET" {
				log.Warnf("Blocked request without Referer: %s %s from %s", c.Method(), path, getRealIP(c))
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Invalid request origin",
				})
			}
			// For GET requests without referer, allow but mark as suspicious
			c.Locals("suspicious_referer", true)
			return c.Next()
		}

		// Validate that referer is from our own domain
		host := c.Hostname()
		if !isValidReferer(referer, host) {
			log.Warnf("Blocked request with external Referer: %s %s from %s (referer: %s)",
				c.Method(), path, getRealIP(c), referer)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Invalid request origin",
			})
		}

		return c.Next()
	}
}

// isRefererExemptPath returns true for paths that don't require referer validation
func isRefererExemptPath(path string) bool {
	exemptPrefixes := []string{
		"/api/auth/",              // Auth endpoints need to work from login forms
		"/api/browser-challenge/", // Challenge endpoints
		"/assets/",                // Static assets
		"/captcha",                // CAPTCHA endpoints
		"/health",                 // Health checks
		"/ready",                  // Readiness checks
		"/robots.txt",             // Robots
		"/favicon",                // Favicon
	}

	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isEntryPointPath returns true for paths that users might access directly
// (bookmarks, typed URLs, shared links)
func isEntryPointPath(path string) bool {
	entryPoints := []string{
		"/",
		"/login",
		"/register",
		"/home",
		"/search",
		"/library",
		"/browse",
	}

	for _, ep := range entryPoints {
		if path == ep {
			return true
		}
	}

	// Also allow direct access to media pages (they might be shared)
	if strings.HasPrefix(path, "/manga/") ||
		strings.HasPrefix(path, "/comic/") ||
		strings.HasPrefix(path, "/novel/") ||
		strings.HasPrefix(path, "/media/") {
		return true
	}

	return false
}

// isValidReferer checks if the referer header points to our own domain
func isValidReferer(referer, host string) bool {
	// Handle common referer formats
	// - https://example.com/page
	// - http://example.com:3000/page
	// - //example.com/page (protocol-relative, rare)

	// Normalize host (remove port for comparison if needed)
	hostWithoutPort := strings.Split(host, ":")[0]

	// Check if referer contains our host
	lowerReferer := strings.ToLower(referer)

	// Match patterns like "://host/" or "://host:" or "://host" at end
	patterns := []string{
		"://" + strings.ToLower(host) + "/",
		"://" + strings.ToLower(host) + ":",
		"://" + strings.ToLower(hostWithoutPort) + "/",
		"://" + strings.ToLower(hostWithoutPort) + ":",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerReferer, pattern) {
			return true
		}
	}

	// Also check if referer ends with just the host (no trailing slash)
	if strings.HasSuffix(lowerReferer, "://"+strings.ToLower(host)) ||
		strings.HasSuffix(lowerReferer, "://"+strings.ToLower(hostWithoutPort)) {
		return true
	}

	// Check for localhost variations
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		if strings.Contains(lowerReferer, "localhost") || strings.Contains(lowerReferer, "127.0.0.1") {
			return true
		}
	}

	return false
}
