package handlers

import (
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
)

const (
	accessTokenDuration  = 15 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
)

var roleHierarchy = map[string]int{
	"reader":    1,
	"moderator": 2,
	"admin":     3,
}

// AuthMiddleware handles token validation and refreshing
func AuthMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		if accessToken != "" {
			if err := validateAccessToken(c, accessToken, requiredRole); err == nil {
				return c.Next()
			}
		}

		if refreshToken != "" {
			if err := refreshAndValidateTokens(c, refreshToken, requiredRole); err == nil {
				return c.Next()
			}
		}

		return c.Redirect("/auth/login", fiber.StatusSeeOther)
	}
}

func validateAccessToken(c *fiber.Ctx, accessToken, requiredRole string) error {
	claims, err := models.ValidateToken(accessToken)
	if err != nil || claims == nil {
		return fiber.ErrUnauthorized
	}

	userName, ok := claims["user_name"].(string)
	if !ok {
		return fiber.ErrUnauthorized
	}

	return validateUserRole(c, userName, requiredRole)
}

func refreshAndValidateTokens(c *fiber.Ctx, refreshToken, requiredRole string) error {
	newAccessToken, userName, err := models.RefreshAccessToken(refreshToken)
	if err != nil || newAccessToken == "" {
		return fiber.ErrUnauthorized
	}

	newRefreshToken, err := models.GenerateNewRefreshToken(userName)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	setAuthCookies(c, newAccessToken, newRefreshToken)

	return validateUserRole(c, userName, requiredRole)
}

func validateUserRole(c *fiber.Ctx, userName, requiredRole string) error {
	user, err := models.FindUserByUsername(userName)
	if err != nil {
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

func clearAuthCookies(c *fiber.Ctx) {
	expiredTime := time.Now().Add(-time.Hour)
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:    "access_token",
		Value:   "",
		Expires: expiredTime,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:    "refresh_token",
		Value:   "",
		Expires: expiredTime,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
}

func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string) {
	// Note: Secure requires HTTPS; we detect TLS or X-Forwarded-Proto to set it.
	// Using Lax so top-level navigations send cookies.
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Expires:  time.Now().Add(accessTokenDuration),
		MaxAge:   int(accessTokenDuration.Seconds()),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  time.Now().Add(refreshTokenDuration),
		MaxAge:   int(refreshTokenDuration.Seconds()),
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

// OptionalAuthMiddleware attempts to authenticate a user if auth cookies are present
// but does not enforce authentication. It sets c.Locals("user_name") when a valid
// token is found so handlers can optionally adapt views for logged-in users.
func OptionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		// If access token exists, try validate; if invalid and refresh exists, try refresh.
		if accessToken != "" {
			if err := validateAccessToken(c, accessToken, "reader"); err == nil {
				return c.Next()
			}
		}

		if refreshToken != "" {
			// Try to refresh tokens and set cookies/locals; ignore errors
			_ = refreshAndValidateTokens(c, refreshToken, "reader")
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
			accessToken := c.Cookies("access_token")
			refreshToken := c.Cookies("refresh_token")

			if accessToken != "" {
				if err := validateAccessToken(c, accessToken, "reader"); err == nil {
					return c.Next()
				}
			}

			if refreshToken != "" {
				if err := refreshAndValidateTokens(c, refreshToken, "reader"); err == nil {
					return c.Next()
				}
			}

			// No valid authentication - redirect to login
			return c.Redirect("/auth/login", fiber.StatusSeeOther)
		}

		// Login not required, but still try to authenticate if cookies present
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		if accessToken != "" {
			if err := validateAccessToken(c, accessToken, "reader"); err == nil {
				return c.Next()
			}
		}

		if refreshToken != "" {
			_ = refreshAndValidateTokens(c, refreshToken, "reader")
		}

		return c.Next()
	}
}
