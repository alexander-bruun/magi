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

// HandleHome renders the landing page with recent media activity and aggregate stats.
func HandleHome(c *fiber.Ctx) error {
	// Fetch data for the home page
	recentlyAdded, _, _ := models.SearchMedias("", 1, 20, "created_at", "desc", "", "")
	recentlyUpdated, _, _ := models.SearchMedias("", 1, 20, "updated_at", "desc", "", "")
	topMedias, _ := models.GetTopMedias(10)
	topReadToday, _ := models.GetTopReadMedias("today", 10)
	topReadWeek, _ := models.GetTopReadMedias("week", 10)
	topReadMonth, _ := models.GetTopReadMedias("month", 10)
	topReadYear, _ := models.GetTopReadMedias("year", 10)
	topReadAll, _ := models.GetTopReadMedias("all", 10)

	// Stats
	totalMedias, _ := models.GetTotalMedias()
	totalChapters, _ := models.GetTotalChapters()
	totalChaptersRead, _ := models.GetTotalChaptersRead()
	mangasChange, _ := models.GetDailyChange("media")
	chaptersChange, _ := models.GetDailyChange("chapters")
	chaptersReadChange, _ := models.GetDailyChange("chapters_read")

	// Light Novel Stats
	totalLightNovels, _ := models.GetTotalMediasByType("novel")
	totalLightNovelChapters, _ := models.GetTotalChaptersByType("novel")
	totalLightNovelChaptersRead, _ := models.GetTotalChaptersReadByType("novel")
	lightNovelsChange, _ := models.GetDailyChangeByType("media", "novel")
	lightNovelChaptersChange, _ := models.GetDailyChangeByType("chapters", "novel")
	lightNovelChaptersReadChange, _ := models.GetDailyChangeByType("chapters_read", "novel")

	return HandleView(c, views.Home(recentlyAdded, recentlyUpdated, topMedias, topReadToday, topReadWeek, topReadMonth, topReadYear, topReadAll, totalMedias, totalChapters, totalChaptersRead, mangasChange, chaptersChange, chaptersReadChange, totalLightNovels, totalLightNovelChapters, totalLightNovelChaptersRead, lightNovelsChange, lightNovelChaptersChange, lightNovelChaptersReadChange))
}

// HandleTopReadPeriod renders the top read list for a specific period via HTMX
func HandleTopReadPeriod(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "today")
	log.Debugf("Top read request for period: %s", period)
	var topRead []models.Media
	var err error
	switch period {
	case "today":
		topRead, err = models.GetTopReadMedias("today", 10)
	case "week":
		topRead, err = models.GetTopReadMedias("week", 10)
	case "month":
		topRead, err = models.GetTopReadMedias("month", 10)
	case "year":
		topRead, err = models.GetTopReadMedias("year", 10)
	case "all":
		topRead, err = models.GetTopReadMedias("all", 10)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top read: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Debugf("Got %d media for period %s", len(topRead), period)

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

	return HandleView(c, views.TopReadFragment(topRead, emptyMessage, title))
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
