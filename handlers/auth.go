package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func RegisterHandler(c *fiber.Ctx) error {
	return HandleView(c, views.Register())
}

func LoginHandler(c *fiber.Ctx) error {
	return HandleView(c, views.Login())
}

func CreateUserHandler(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	if err := models.CreateUser(username, password); err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	c.Set("HX-Redirect", "/login")
	return c.SendStatus(fiber.StatusCreated)
}

func LoginUserHandler(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	user, err := models.FindUserByUsername(username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid username or password"})
	}

	accessToken, err := models.CreateAccessToken(user.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create access token"})
	}

	refreshToken, err := models.GenerateNewRefreshToken(user.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create refresh token"})
	}

	setAuthCookies(c, accessToken, refreshToken)
	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusOK)
}

func LogoutHandler(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")

	if claims, err := models.ValidateToken(refreshToken); err == nil && claims != nil {
		if userName, ok := claims["user_name"].(string); ok {
			if user, err := models.FindUserByUsername(userName); err == nil {
				models.IncrementRefreshTokenVersion(user.Username)
			}
		}
	}

	clearAuthCookies(c)
	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusOK)
}
