package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

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

	// For admins, allow access to all without library filter
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			accessibleLibraries = []string{} // Admins can see all
		}
	}

	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}

	// Fetch data for the home page
	opts := models.SearchOptions{
		Filter:              "",
		Page:                1,
		PageSize:            12, // Reduced from 20
		SortBy:              "created_at",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  cfg.ContentRatingLimit,
	}

	recentlyAdded, _, _ := models.SearchMediasWithOptions(opts)

	// Fetch latest updates data
	latestUpdates, err := models.GetRecentSeriesWithChapters(12, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled, accessibleLibraries) // Reduced from 18
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

	// Batch fetch vote data
	voteData, err := models.BatchGetMediaVotes(allSlugs)
	if err != nil {
		log.Errorf("Error fetching vote data: %v", err)
		voteData = make(map[string][3]int) // Empty map if error
	}

	// Enrich recently added media
	enrichedRecentlyAdded := make([]models.EnrichedMedia, len(recentlyAdded))
	for i, m := range recentlyAdded {
		enrichData := enrichmentData[m.Slug]
		votes := voteData[m.Slug]
		enrichedRecentlyAdded[i] = models.EnrichedMedia{
			Media:             m,
			PremiumCountdown:  enrichData.PremiumCountdown,
			LatestChapterSlug: enrichData.LatestChapterSlug,
			LatestChapterName: enrichData.LatestChapterName,
			AverageRating:     enrichData.AverageRating,
			ReviewCount:       enrichData.ReviewCount,
			VoteScore:         votes[0],
			Upvotes:           votes[1],
			Downvotes:         votes[2],
		}
	}

	// Enrich recently updated media
	enrichedRecentlyUpdated := make([]models.EnrichedMedia, len(recentlyUpdated))
	for i, m := range recentlyUpdated {
		enrichData := enrichmentData[m.Slug]
		votes := voteData[m.Slug]
		enrichedRecentlyUpdated[i] = models.EnrichedMedia{
			Media:             m,
			PremiumCountdown:  enrichData.PremiumCountdown,
			LatestChapterSlug: enrichData.LatestChapterSlug,
			LatestChapterName: enrichData.LatestChapterName,
			AverageRating:     enrichData.AverageRating,
			ReviewCount:       enrichData.ReviewCount,
			VoteScore:         votes[0],
			Upvotes:           votes[1],
			Downvotes:         votes[2],
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

	// Fetch notifications for logged-in users
	var notifications []models.UserNotification
	if userName != "" {
		if notifs, err := models.GetUserNotifications(userName, true); err == nil {
			notifications = notifs
		}
	}

	// Fetch homepage statistics
	stats, err := models.GetHomePageStats()
	if err != nil {
		log.Errorf("Failed to get homepage statistics: %v", err)
		// Continue with empty stats if error
		stats = models.HomePageStats{}
	}

	return handleView(c, views.Home(enrichedRecentlyAdded, enrichedRecentlyUpdated, cfg.PremiumEarlyAccessDuration, topMedias, topReadToday, topReadWeek, topReadAll, latestUpdates, highlights, stats, unreadCount, notifications), unreadCount, notifications)
}

// HandleTopPopularFull renders the Popular section with sub-navigation
func HandleTopPopularFull(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !isHTMXRequest(c) {
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

// HandleTopReadFull renders the Most Read section with sub-navigation
func HandleTopReadFull(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !isHTMXRequest(c) {
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
	if !isHTMXRequest(c) {
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
	if !isHTMXRequest(c) {
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

// HandleNotFound renders the generic not-found page for unrouted paths.
func HandleNotFound(c *fiber.Ctx) error {
	return handleView(c, views.NotFound())
}
