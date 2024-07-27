package handlers

import (
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
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

	user := models.User{
		Username:            username,
		Password:            password,
		RefreshTokenVersion: 0,
		Role:                "reader", // Default role
	}

	count, _ := models.CountUsers()
	if count == 0 {
		log.Infof("No users has yet been registered, promoting '%s' to 'admin' role.", user.Username)
		user.Role = "admin"
	}

	err := user.CreateUser()
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	c.Set("HX-Redirect", "/login")

	return c.SendStatus(fiber.StatusOK)
}

func LoginUserHandler(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	user, err := models.FindUserByUsername(username)
	if err != nil {
		return HandleView(c, views.WrongCredentials())
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return HandleView(c, views.WrongCredentials())
	}

	accessExpirationTime := time.Now().Add(15 * time.Minute)
	accessToken, err := utils.GenerateToken(user.Username, "access", user.RefreshTokenVersion, accessExpirationTime)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	refreshExpirationTime := time.Now().Add(30 * 24 * time.Hour)
	refreshToken, err := utils.GenerateToken(user.Username, "refresh", user.RefreshTokenVersion, refreshExpirationTime)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Setting the access token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Expires:  accessExpirationTime,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict", // Use "Strict" for SameSite attribute
	})

	// Setting the refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  refreshExpirationTime,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict", // Use "Strict" for SameSite attribute
	})

	c.Set("HX-Redirect", "/")

	return c.SendStatus(fiber.StatusOK)
}
