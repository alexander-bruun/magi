package handlers

import (
	"fmt"
	"time"

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

	err := models.CreateUser(username, password)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	c.Set("HX-Redirect", "/login")

	return c.SendStatus(fiber.StatusUnauthorized)
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

	c.Cookie(&fiber.Cookie{
		Name:    "access_token",
		Value:   accessToken,
		Expires: time.Now().Add(time.Minute * 15),
	})
	c.Cookie(&fiber.Cookie{
		Name:    "refresh_token",
		Value:   refreshToken,
		Expires: time.Now().Add(time.Hour * 24 * 7),
	})

	c.Set("HX-Redirect", "/")

	return c.SendStatus(fiber.StatusOK)
}

// LogoutHandler handles user logout by clearing cookies and invalidating tokens
func LogoutHandler(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")

	claims, err := models.ValidateToken(refreshToken)
	if err == nil && claims != nil {
		userName := claims["user_name"].(string)
		user, err := models.FindUserByUsername(userName)
		if err != nil {
			return fmt.Errorf("failed to find user: %s", userName)
		} else {
			models.IncrementRefreshTokenVersion(user)
		}
	}

	c.Cookie(&fiber.Cookie{
		Name:    "access_token",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	})
	c.Cookie(&fiber.Cookie{
		Name:    "refresh_token",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	})

	c.Set("HX-Redirect", "/")

	return c.SendStatus(fiber.StatusOK)
}
