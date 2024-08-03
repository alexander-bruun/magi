package handlers

import (
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware handles token validation and refreshing
func AuthMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		if accessToken != "" {
			claims, err := models.ValidateToken(accessToken)
			if err == nil && claims != nil {
				userName := claims["user_name"].(string)
				// Fetch the user using the models package
				user, err := models.FindUserByUsername(userName)
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

				c.Locals("user_name")
				return c.Next()
			}
		}

		if refreshToken != "" {
			newAccessToken, userName, err := models.RefreshAccessToken(refreshToken)
			if err == nil && newAccessToken != "" {
				newRefreshToken, _ := models.GenerateNewRefreshToken(userName)

				c.Cookie(&fiber.Cookie{
					Name:    "access_token",
					Value:   newAccessToken,
					Expires: time.Now().Add(time.Minute * 15),
				})
				c.Cookie(&fiber.Cookie{
					Name:    "refresh_token",
					Value:   newRefreshToken,
					Expires: time.Now().Add(time.Hour * 24 * 7),
				})

				// Fetch the user using the models package
				user, err := models.FindUserByUsername(userName)
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

				c.Locals("user_name", userName)
				return c.Next()
			}
		}

		return c.Redirect("/login", fiber.StatusSeeOther)
	}
}
