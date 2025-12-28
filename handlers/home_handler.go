package handlers

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

const (
	sessionTokenCookie = "session_token"
)

// HandleView wraps a page component with the layout unless the request is an HTMX fragment.
func HandleView(c *fiber.Ctx, content templ.Component, unreadCountAndNotifications ...interface{}) error {
	// Return partial content if HTMX request
	if IsHTMXRequest(c) {
		return renderComponent(c, content)
	}

	userRole, err := getUserRole(c)
	if err != nil {
		// Log the error, but continue with an empty user role
		// This allows the page to render for non-authenticated users
		log.Errorf("Error getting user role: %v", err)
	}

	unreadCount := 0
	notifications := []models.UserNotification{}
	if len(unreadCountAndNotifications) > 0 {
		unreadCount = unreadCountAndNotifications[0].(int)
	}
	if len(unreadCountAndNotifications) > 1 {
		notifications = unreadCountAndNotifications[1].([]models.UserNotification)
	}

	// pass current request path so templates can mark active nav items
	base := views.Layout(content, userRole, c.Path(), unreadCount, notifications)
	return renderComponent(c, base)
}

// HandleHome renders the landing page with recent media activity and aggregate stats.
func HandleHome(c *fiber.Ctx) error {
	userName := GetUserContext(c)

	// Get accessible libraries for the current user
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	log.Debugf("User %s has access to libraries: %v", userName, accessibleLibraries)

	// Fetch data for the home page
	opts := models.SearchOptions{
		Filter:              "",
		Page:                1,
		PageSize:            20,
		SortBy:              "created_at",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
	}
	recentlyAdded, _, _ := models.SearchMediasWithOptions(opts)
	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}

	// Fetch latest updates data
	latestUpdates, err := models.GetRecentSeriesWithChapters(21, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled, accessibleLibraries)
	if err != nil {
		log.Errorf("Failed to get latest updates: %v", err)
		latestUpdates = []models.MediaWithRecentChapters{} // Empty slice if error
	}
	log.Debugf("GetRecentSeriesWithChapters returned %d items", len(latestUpdates))

	// No need to filter latest updates anymore - already filtered at database level
	log.Debugf("LatestUpdates count for user %s: %d", userName, len(latestUpdates))

	// Use the same media for recently updated, ordered by latest chapter
	var recentlyUpdated []models.Media
	for _, mwc := range latestUpdates {
		recentlyUpdated = append(recentlyUpdated, mwc.Media)
	}

	// Batch enrich recently added and updated media (reduces 60 queries to 2)
	allSlugs := make([]string, 0, len(recentlyAdded)+len(recentlyUpdated))
	for _, m := range recentlyAdded {
		allSlugs = append(allSlugs, m.Slug)
	}
	for _, m := range recentlyUpdated {
		allSlugs = append(allSlugs, m.Slug)
	}

	enrichmentData, err := models.BatchEnrichMediaData(allSlugs, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
	if err != nil {
		log.Errorf("Error enriching media data: %v", err)
		enrichmentData = make(map[string]models.MediaEnrichmentData) // Empty map if error
	}

	// Enrich recently added media
	enrichedRecentlyAdded := make([]models.EnrichedMedia, len(recentlyAdded))
	for i, m := range recentlyAdded {
		enrichData := enrichmentData[m.Slug]
		enrichedRecentlyAdded[i] = models.EnrichedMedia{
			Media:             m,
			PremiumCountdown:  enrichData.PremiumCountdown,
			LatestChapterSlug: enrichData.LatestChapterSlug,
			LatestChapterName: enrichData.LatestChapterName,
			AverageRating:     enrichData.AverageRating,
			ReviewCount:       enrichData.ReviewCount,
		}
	}

	// Enrich recently updated media
	enrichedRecentlyUpdated := make([]models.EnrichedMedia, len(recentlyUpdated))
	for i, m := range recentlyUpdated {
		enrichData := enrichmentData[m.Slug]
		enrichedRecentlyUpdated[i] = models.EnrichedMedia{
			Media:             m,
			PremiumCountdown:  enrichData.PremiumCountdown,
			LatestChapterSlug: enrichData.LatestChapterSlug,
			LatestChapterName: enrichData.LatestChapterName,
			AverageRating:     enrichData.AverageRating,
			ReviewCount:       enrichData.ReviewCount,
		}
	}

	topMedias, err := models.GetTopMedias(10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top medias: %v", err)
	}
	log.Debugf("Got %d top medias for libraries %v", len(topMedias), accessibleLibraries)

	topReadToday, err := models.GetTopReadMedias("today", 10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top read today: %v", err)
	}
	log.Debugf("Got %d top read today for libraries %v", len(topReadToday), accessibleLibraries)
	topReadWeek, _ := models.GetTopReadMedias("week", 10, accessibleLibraries)
	topReadMonth, _ := models.GetTopReadMedias("month", 10, accessibleLibraries)
	topReadYear, _ := models.GetTopReadMedias("year", 10, accessibleLibraries)
	topReadAll, _ := models.GetTopReadMedias("all", 10, accessibleLibraries)

	// No need to filter top media lists anymore - already filtered at database level

	// Mark chapters as read for logged-in users
	unreadCount := 0
	if userName != "" {
		for i := range latestUpdates {
			readMap, err := models.GetReadChaptersForUser(userName, latestUpdates[i].Media.Slug)
			if err == nil {
				for j := range latestUpdates[i].Chapters {
					latestUpdates[i].Chapters[j].Read = readMap[latestUpdates[i].Chapters[j].Slug]
				}
			}
		}

		// Fetch unread notification count for logged-in users
		if count, err := models.GetUnreadNotificationCount(userName); err == nil {
			unreadCount = count
		}
	}

	// Fetch highlights for the banner
	highlights, err := models.GetHighlights()
	if err != nil {
		log.Errorf("Failed to get highlights: %v", err)
		highlights = []models.HighlightWithMedia{} // Empty slice if error
	}

	// Filter highlights to only include accessible libraries
	if len(accessibleLibraries) > 0 {
		// Create a set for O(1) lookup
		librarySet := make(map[string]struct{}, len(accessibleLibraries))
		for _, slug := range accessibleLibraries {
			librarySet[slug] = struct{}{}
		}

		filteredHighlights := make([]models.HighlightWithMedia, 0, len(highlights))
		for _, highlight := range highlights {
			if _, ok := librarySet[highlight.Media.LibrarySlug]; ok {
				filteredHighlights = append(filteredHighlights, highlight)
			}
		}
		highlights = filteredHighlights
	} else if len(accessibleLibraries) == 0 && userName != "" {
		// If user has no accessible libraries, clear highlights
		highlights = []models.HighlightWithMedia{}
	}

	// Fetch notifications for logged-in users
	var notifications []models.UserNotification
	if userName != "" {
		if notifs, err := models.GetUserNotifications(userName, true); err == nil {
			notifications = notifs
		}
	}

	return HandleView(c, views.Home(enrichedRecentlyAdded, enrichedRecentlyUpdated, cfg.PremiumEarlyAccessDuration, topMedias, topReadToday, topReadWeek, topReadMonth, topReadYear, topReadAll, latestUpdates, highlights, unreadCount, notifications), unreadCount, notifications)
}

// HandleTopReadPeriod renders the top read list for a specific period via HTMX
func HandleTopReadPeriod(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "today")
	log.Debugf("Top read request for period: %s", period)

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	var topRead []models.Media
	var err error
	switch period {
	case "today":
		topRead, err = models.GetTopReadMedias("today", 10, accessibleLibraries)
	case "week":
		topRead, err = models.GetTopReadMedias("week", 10, accessibleLibraries)
	case "month":
		topRead, err = models.GetTopReadMedias("month", 10, accessibleLibraries)
	case "year":
		topRead, err = models.GetTopReadMedias("year", 10, accessibleLibraries)
	case "all":
		topRead, err = models.GetTopReadMedias("all", 10, accessibleLibraries)
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

	return renderComponent(c, views.TopReadList(topRead, emptyMessage, title))
}

// HandleTopPopular renders the top popular media list via HTMX
func HandleTopPopular(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	topMedias, err := models.GetTopMedias(10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top medias: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Debugf("Got %d top medias", len(topMedias))

	return renderComponent(c, views.TopPopularFragment(topMedias))
}

// HandleTopPopularFull renders the Popular section with sub-navigation
func HandleTopPopularFull(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "all")
	log.Debugf("Top popular full request for period: %s", period)

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	var topMedias []models.Media
	var err error
	switch period {
	case "today":
		topMedias, err = models.GetTopMediasByPeriod("today", 10, accessibleLibraries)
	case "week":
		topMedias, err = models.GetTopMediasByPeriod("week", 10, accessibleLibraries)
	case "month":
		topMedias, err = models.GetTopMediasByPeriod("month", 10, accessibleLibraries)
	case "year":
		topMedias, err = models.GetTopMediasByPeriod("year", 10, accessibleLibraries)
	case "all":
		topMedias, err = models.GetTopMediasByPeriod("all", 10, accessibleLibraries)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top popular: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Debugf("Got %d media for period %s", len(topMedias), period)

	return renderComponent(c, views.TopPopularFullFragment(topMedias, period))
}

// HandleTop10 renders the entire Top 10 card with HTMX tabs
func HandleTop10(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	topMedias, err := models.GetTopMedias(10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top medias: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	topReadAll, err := models.GetTopReadMedias("all", 10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top read all: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Debugf("Got %d top medias and %d top read", len(topMedias), len(topReadAll))

	return renderComponent(c, views.Top10Card(topMedias, topReadAll))
}

// HandleTopReadFull renders the Most Read section with sub-navigation
func HandleTopReadFull(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "today")
	log.Debugf("Top read full request for period: %s", period)

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	var topRead []models.Media
	var err error
	switch period {
	case "today":
		topRead, err = models.GetTopReadMedias("today", 10, accessibleLibraries)
	case "week":
		topRead, err = models.GetTopReadMedias("week", 10, accessibleLibraries)
	case "month":
		topRead, err = models.GetTopReadMedias("month", 10, accessibleLibraries)
	case "year":
		topRead, err = models.GetTopReadMedias("year", 10, accessibleLibraries)
	case "all":
		topRead, err = models.GetTopReadMedias("all", 10, accessibleLibraries)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top read: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	log.Debugf("Got %d media for period %s", len(topRead), period)

	return renderComponent(c, views.TopReadFullWrapper(topRead, period))
}

// HandleTopPopularCard renders the full top 10 card with popular content and correct active tab
func HandleTopPopularCard(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "all")

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	var topPopular []models.Media
	var err error
	switch period {
	case "today":
		topPopular, err = models.GetTopMediasByPeriod("today", 10, accessibleLibraries)
	case "week":
		topPopular, err = models.GetTopMediasByPeriod("week", 10, accessibleLibraries)
	case "month":
		topPopular, err = models.GetTopMediasByPeriod("month", 10, accessibleLibraries)
	case "year":
		topPopular, err = models.GetTopMediasByPeriod("year", 10, accessibleLibraries)
	case "all":
		topPopular, err = models.GetTopMediasByPeriod("all", 10, accessibleLibraries)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top popular: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	// Get top read for all time (for the initial state)
	var topReadAll []models.Media
	topReadAll, err = models.GetTopReadMedias("all", 10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top read all: %v", err)
		topReadAll = []models.Media{}
	}

	log.Debugf("Got %d popular media for period %s", len(topPopular), period)

	return renderComponent(c, views.Top10CardContent("popular", period, topPopular, topReadAll))
}

// HandleTopReadCard renders the full top 10 card with read content and correct active tab
func HandleTopReadCard(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	period := c.Query("period", "all")

	// Get accessible libraries for the current user
	userName := GetUserContext(c)
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
			accessibleLibraries = []string{} // Empty if error
		}
	} else {
		var err error
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
		if err != nil {
			log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
			accessibleLibraries = []string{} // Empty if error
		}
	}

	var topRead []models.Media
	var err error
	switch period {
	case "today":
		topRead, err = models.GetTopReadMedias("today", 10, accessibleLibraries)
	case "week":
		topRead, err = models.GetTopReadMedias("week", 10, accessibleLibraries)
	case "month":
		topRead, err = models.GetTopReadMedias("month", 10, accessibleLibraries)
	case "year":
		topRead, err = models.GetTopReadMedias("year", 10, accessibleLibraries)
	case "all":
		topRead, err = models.GetTopReadMedias("all", 10, accessibleLibraries)
	default:
		return c.Status(400).SendString("Invalid period")
	}
	if err != nil {
		log.Errorf("Error getting top read: %v", err)
		return c.Status(500).SendString(err.Error())
	}

	// Get top popular for all time (for the initial state)
	var topPopular []models.Media
	topPopular, err = models.GetTopMediasByPeriod("all", 10, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting top popular: %v", err)
		topPopular = []models.Media{}
	}

	log.Debugf("Got %d read media for period %s", len(topRead), period)

	return renderComponent(c, views.Top10CardContent("read", period, topPopular, topRead))
}

// HandleStatistics renders the statistics section via HTMX
func HandleStatistics(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !IsHTMXRequest(c) {
		return c.Redirect("/")
	}

	return renderComponent(c, views.StatisticsFragment())
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

// filterMediaByAccessibleLibraries filters a slice of media to only include those from accessible libraries
func filterMediaByAccessibleLibraries(media []models.Media, librarySet map[string]struct{}) []models.Media {
	if len(librarySet) == 0 {
		return []models.Media{}
	}

	filtered := make([]models.Media, 0, len(media))
	for _, m := range media {
		if _, ok := librarySet[m.LibrarySlug]; ok {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
