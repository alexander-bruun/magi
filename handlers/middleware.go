package handlers

import (
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware is a Fiber middleware for authentication and authorization
func AuthMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		accessToken := c.Cookies("access_token")
		if accessToken == "" {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}

		claims, err := utils.ValidateToken(accessToken)
		if err != nil {
			// Access token is invalid, try to refresh
			refreshToken := c.Cookies("refresh_token")
			if refreshToken == "" {
				return c.Redirect("/login", fiber.StatusSeeOther)
			}

			// Use the models package to find user
			user, err := models.FindUserByUsername(claims.Username)
			if err != nil {
				return c.Redirect("/login", fiber.StatusSeeOther)
			}

			newAccessToken, newRefreshToken, err := utils.RefreshToken(refreshToken, user.RefreshTokenVersion)
			if err != nil {
				return c.Redirect("/login", fiber.StatusSeeOther)
			}

			// Set new cookies
			c.Cookie(&fiber.Cookie{
				Name:     "access_token",
				Value:    newAccessToken,
				Expires:  time.Now().Add(15 * time.Minute),
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict", // Use "Strict" for SameSite attribute
			})

			c.Cookie(&fiber.Cookie{
				Name:     "refresh_token",
				Value:    newRefreshToken,
				Expires:  time.Now().Add(30 * 24 * time.Hour),
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict", // Use "Strict" for SameSite attribute
			})

			// Validate the new access token
			claims, err = utils.ValidateToken(newAccessToken)
			if err != nil {
				return c.Redirect("/login", fiber.StatusSeeOther)
			}
		}

		// Fetch the user using the models package
		user, err := models.FindUserByUsername(claims.Username)
		if err != nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}

		roles := map[string]int{
			"reader":    1,
			"moderator": 2,
			"admin":     3,
		}

		if roles[user.Role] < roles[requiredRole] {
			return c.SendStatus(fiber.StatusForbidden)
		}

		// Set user data in context
		c.Locals("Username", claims.Username)

		return c.Next()
	}
}
