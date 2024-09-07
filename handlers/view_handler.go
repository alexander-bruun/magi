package handlers

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

const (
	htmxRequestHeader = "HX-Request"
	accessTokenCookie = "access_token"
)

func HandleView(c *fiber.Ctx, content templ.Component) error {
	if c.Get(htmxRequestHeader) != "" {
		return renderComponent(c, content)
	}

	userRole, err := getUserRole(c)
	if err != nil {
		// Log the error, but continue with an empty user role
		// This allows the page to render for non-authenticated users
		fmt.Printf("Error getting user role: %v\n", err)
	}

	base := views.Layout(content, userRole)
	return renderComponent(c, base)
}

func HandleHome(c *fiber.Ctx) error {
	recentlyAdded, err := getRecentMangas("created_at")
	if err != nil {
		return handleError(c, err)
	}

	recentlyUpdated, err := getRecentMangas("updated_at")
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.Home(recentlyAdded, recentlyUpdated))
}

func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

func HandleAdmin(c *fiber.Ctx) error {
	libraries, err := models.GetLibraries()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Admin(libraries))
}

func HandleUsers(c *fiber.Ctx) error {
	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Users(users))
}

// Helper functions

func renderComponent(c *fiber.Ctx, component templ.Component) error {
	handler := adaptor.HTTPHandler(templ.Handler(component))
	return handler(c)
}

func getUserRole(c *fiber.Ctx) (string, error) {
	accessToken := c.Cookies(accessTokenCookie)
	if accessToken == "" {
		return "", nil
	}

	claims, err := models.ValidateToken(accessToken)
	if err != nil || claims == nil {
		return "", fmt.Errorf("invalid access token")
	}

	userName, ok := claims["user_name"].(string)
	if !ok {
		return "", fmt.Errorf("user_name not found in token claims")
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %s", userName)
	}

	return user.Role, nil
}

func getRecentMangas(sortBy string) ([]models.Manga, error) {
	mangas, _, err := models.SearchMangas("", 1, 10, sortBy, "desc", "", "")
	return mangas, err
}

func handleError(c *fiber.Ctx, err error) error {
	return HandleView(c, views.Error(err.Error()))
}
