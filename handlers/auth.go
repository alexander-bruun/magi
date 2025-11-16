package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler renders the registration form page.
func RegisterHandler(c *fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return handleError(c, err)
	}
	count, err := models.CountUsers()
	if err != nil {
		return handleError(c, err)
	}
	if !cfg.AllowRegistration || (cfg.MaxUsers > 0 && count >= cfg.MaxUsers) {
		return HandleView(c, views.Error("Registration is currently disabled."))
	}
	return HandleView(c, views.Register())
}

// LoginHandler renders the login page.
func LoginHandler(c *fiber.Ctx) error {
	target := c.Query("target", "")
	return HandleView(c, views.Login(target))
}

// CreateUserHandler processes a registration submission and redirects to login on success.
func CreateUserHandler(c *fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return handleError(c, err)
	}
	count, err := models.CountUsers()
	if err != nil {
		return handleError(c, err)
	}
	if !cfg.AllowRegistration || (cfg.MaxUsers > 0 && count >= cfg.MaxUsers) {
		return HandleView(c, views.Error("Registration is currently disabled."))
	}
	username := c.FormValue("username")
	password := c.FormValue("password")

	if err := models.CreateUser(username, password); err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	c.Set("HX-Redirect", "/auth/login")
	return c.SendStatus(fiber.StatusCreated)
}

// LoginUserHandler validates credentials and issues access/refresh tokens.
func LoginUserHandler(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		// Return the WrongCredentials view for HTMX requests
		return HandleView(c, views.WrongCredentials())
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
	
	// Redirect to target page if provided, otherwise home
	target := c.FormValue("target")
	if target == "" {
		target = "/"
	}
	c.Set("HX-Redirect", target)
	return c.SendStatus(fiber.StatusOK)
}

// LogoutHandler revokes refresh tokens and clears authentication cookies.
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
