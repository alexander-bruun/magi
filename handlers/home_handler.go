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
	sessionTokenCookie = "session_token"
)

// HandleView wraps a page component with the layout unless the request is an HTMX fragment.
func HandleView(c *fiber.Ctx, content templ.Component) error {
	// Return partial content if HTMX target is specified
	if GetHTMXTarget(c) != "" {
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
	// Fetch data for the home page
	recentlyAdded, _, _ := models.SearchMangas("", 1, 20, "created_at", "desc", "", "")
	recentlyUpdated, _, _ := models.SearchMangas("", 1, 20, "updated_at", "desc", "", "")
	topMangas, _ := models.GetTopMangas(10)
	topReadToday, _ := models.GetTopReadMangas("today", 10)
	topReadWeek, _ := models.GetTopReadMangas("week", 10)
	topReadMonth, _ := models.GetTopReadMangas("month", 10)
	topReadYear, _ := models.GetTopReadMangas("year", 10)
	topReadAll, _ := models.GetTopReadMangas("all", 10)

	// Stats
	totalMangas, _ := models.GetTotalMangas()
	totalChapters, _ := models.GetTotalChapters()
	totalChaptersRead, _ := models.GetTotalChaptersRead()
	totalLightNovels, _ := models.GetTotalLightNovels()
	totalLightNovelChapters, _ := models.GetTotalLightNovelChapters()
	totalLightNovelChaptersRead, _ := models.GetTotalLightNovelChaptersRead()
	mangasChange, _ := models.GetDailyChange("mangas")
	chaptersChange, _ := models.GetDailyChange("chapters")
	chaptersReadChange, _ := models.GetDailyChange("chapters_read")
	lightNovelsChange, _ := models.GetDailyChange("light_novels")
	lightNovelChaptersChange, _ := models.GetDailyChange("light_novel_chapters")
	lightNovelChaptersReadChange, _ := models.GetDailyChange("light_novel_chapters_read")

	return HandleView(c, views.Home(recentlyAdded, recentlyUpdated, topMangas, topReadToday, topReadWeek, topReadMonth, topReadYear, topReadAll, totalMangas, totalChapters, totalChaptersRead, totalLightNovels, totalLightNovelChapters, totalLightNovelChaptersRead, mangasChange, chaptersChange, chaptersReadChange, lightNovelsChange, lightNovelChaptersChange, lightNovelChaptersReadChange))
}

// HandleTopReadPeriod renders the top read list for a specific period via HTMX
func HandleTopReadPeriod(c *fiber.Ctx) error {
	period := c.Query("period", "today")
	log.Infof("Top read request for period: %s", period)
	var topRead []models.Manga
	var err error
	switch period {
	case "today":
		topRead, err = models.GetTopReadMangas("today", 10)
	case "week":
		topRead, err = models.GetTopReadMangas("week", 10)
	case "month":
		topRead, err = models.GetTopReadMangas("month", 10)
	case "year":
		topRead, err = models.GetTopReadMangas("year", 10)
	case "all":
		topRead, err = models.GetTopReadMangas("all", 10)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top read: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Infof("Got %d mangas for period %s", len(topRead), period)

	var title string
	var emptyMessage string
	switch period {
	case "today":
		emptyMessage = "No reads today"
	case "week":
		emptyMessage = "No reads this week"
	case "month":
		emptyMessage = "No reads this month"
	case "year":
		emptyMessage = "No reads this year"
	case "all":
		emptyMessage = "No reads recorded"
	}

	return renderComponent(c, views.TopReadFragment(topRead, emptyMessage, title))
}

// HandleNotFound renders the generic not-found page for unrouted paths.
func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

// Helper functions

func renderComponent(c *fiber.Ctx, component templ.Component) error {
	// Preserve the status code if it was already set
	statusCode := c.Response().StatusCode()
	if statusCode == 0 {
		statusCode = fiber.StatusOK
	}
	
	handler := adaptor.HTTPHandler(templ.Handler(component))
	err := handler(c)
	
	// Restore the status code after rendering
	c.Status(statusCode)
	return err
}

func getUserRole(c *fiber.Ctx) (string, error) {
	sessionToken := c.Cookies(sessionTokenCookie)
	if sessionToken == "" {
		return "", nil
	}

	userName, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return "", fmt.Errorf("invalid session token")
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %s", userName)
	}
	if user == nil {
		return "", fmt.Errorf("user not found: %s", userName)
	}

	return user.Role, nil
}

// handleError renders an error view with an appropriate HTTP status code
func handleError(c *fiber.Ctx, err error) error {
	return handleErrorWithStatus(c, err, fiber.StatusInternalServerError)
}

// handleErrorWithStatus renders an error view with a custom HTTP status code
func handleErrorWithStatus(c *fiber.Ctx, err error, status int) error {
	c.Status(status)
	return HandleView(c, views.ErrorWithStatus(status, err.Error()))
}
