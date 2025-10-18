package handlers

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

const (
	accessTokenCookie = "access_token"
)

// HandleView wraps a page component with the layout unless the request is an HTMX fragment.
func HandleView(c *fiber.Ctx, content templ.Component) error {
	// Check if this is an HTMX request but NOT a history restore request
	// History restore requests should get the full layout
	isHistoryRestore := c.Get("HX-History-Restore-Request") == "true"
	
	if IsHTMXRequest(c) && !isHistoryRestore {
		return renderComponent(c, content)
	}

	userRole, err := getUserRole(c)
	if err != nil {
		// Log the error, but continue with an empty user role
		// This allows the page to render for non-authenticated users
		log.Errorf("Error getting user role: %v", err)
	}

	// pass current request path so templates can mark active nav items
	base := views.Layout(content, userRole, c.Path())
	return renderComponent(c, base)
}

// HandleHome renders the landing page with recent manga activity and aggregate stats.
func HandleHome(c *fiber.Ctx) error {
	recentlyAdded, err := getRecentMangas("created_at")
	if err != nil {
		return handleError(c, err)
	}

	recentlyUpdated, err := getRecentMangas("updated_at")
	if err != nil {
		return handleError(c, err)
	}

	// Fetch basic stats for the homepage
	totalMangas, err := models.GetTotalMangas()
	if err != nil {
		return handleError(c, err)
	}
	totalChapters, err := models.GetTotalChapters()
	if err != nil {
		return handleError(c, err)
	}
	totalChaptersRead, err := models.GetTotalChaptersRead()
	if err != nil {
		return handleError(c, err)
	}

	// Fetch top 10 popular mangas
	topMangas, err := models.GetTopMangas(10)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.Home(recentlyAdded, recentlyUpdated, topMangas, totalMangas, totalChapters, totalChaptersRead))
}

// HandleNotFound renders the generic not-found page for unrouted paths.
func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
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
	mangas, _, err := models.SearchMangas("", 1, 20, sortBy, "desc", "", "")
	return mangas, err
}

func handleError(c *fiber.Ctx, err error) error {
	return HandleView(c, views.Error(err.Error()))
}
