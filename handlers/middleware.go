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

		return c.Redirect("/login", fiber.StatusSeeOther)
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
	c.Cookie(&fiber.Cookie{
		Name:    "access_token",
		Value:   "",
		Expires: expiredTime,
	})
	c.Cookie(&fiber.Cookie{
		Name:    "refresh_token",
		Value:   "",
		Expires: expiredTime,
	})
}

func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string) {
	c.Cookie(&fiber.Cookie{
		Name:    "access_token",
		Value:   accessToken,
		Expires: time.Now().Add(accessTokenDuration),
	})
	c.Cookie(&fiber.Cookie{
		Name:    "refresh_token",
		Value:   refreshToken,
		Expires: time.Now().Add(refreshTokenDuration),
	})
}

// OptionalAuthMiddleware attempts to authenticate a user if auth cookies are present
// but does not enforce authentication. It sets c.Locals("user_name") when a valid
// token is found so handlers can optionally adapt views for logged-in users.
func OptionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		if accessToken != "" {
			// Try to validate token and set user info; ignore errors
			_ = validateAccessToken(c, accessToken, "reader")
			return c.Next()
		}

		if refreshToken != "" {
			// Try to refresh tokens and set cookies/locals; ignore errors
			_ = refreshAndValidateTokens(c, refreshToken, "reader")
			return c.Next()
		}

		return c.Next()
	}
}
