package handlers

import (
	"sync"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// HandleHome renders the landing page with recent media activity and aggregate stats.
func HandleHome(c fiber.Ctx) error {
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

	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}

	// Get content rating limit
	contentRatingLimit := GetContentRatingLimit(userName)

	// For admins, allow access to all without library filter
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			accessibleLibraries = []string{} // Admins can see all
		}
	}

	// --- Run independent queries in parallel ---
	var (
		wg              sync.WaitGroup
		recentlyAdded   []models.Media
		latestUpdates   []models.MediaWithRecentChapters
		highlights      []models.HighlightWithMedia
		stats           models.HomePageStats
		notifications   []models.UserNotification
		readingActivity []models.ReadingActivityItem
		unreadCount     int
	)

	opts := models.SearchOptions{
		Filter:              "",
		Page:                1,
		PageSize:            10,
		SortBy:              "created_at",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  contentRatingLimit,
	}

	wg.Add(4) // recentlyAdded, latestUpdates, highlights, stats

	go func() {
		defer wg.Done()
		recentlyAdded, _, _ = models.SearchMediasWithOptions(opts)
	}()

	go func() {
		defer wg.Done()
		var err error
		latestUpdates, err = models.GetRecentSeriesWithChapters(24, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled, accessibleLibraries)
		if err != nil {
			log.Errorf("Failed to get latest updates: %v", err)
			latestUpdates = []models.MediaWithRecentChapters{}
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		highlights, err = models.GetHighlights()
		if err != nil {
			log.Errorf("Failed to get highlights: %v", err)
			highlights = []models.HighlightWithMedia{}
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		stats, err = models.GetHomePageStats()
		if err != nil {
			log.Errorf("Failed to get homepage statistics: %v", err)
			stats = models.HomePageStats{}
		}
	}()

	// User-specific queries (also parallel)
	if userName != "" {
		wg.Add(2) // notifications, readingActivity

		go func() {
			defer wg.Done()
			if notifs, err := models.GetUserNotifications(userName, true); err == nil {
				notifications = notifs
			}
		}()

		go func() {
			defer wg.Done()
			if activities, err := models.GetRecentReadingActivity(userName, 10); err == nil {
				readingActivity = activities
			}
		}()
	}

	wg.Wait()

	log.Debugf("GetRecentSeriesWithChapters returned %d items", len(latestUpdates))
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

	// Run enrichment + votes in parallel
	var (
		enrichmentData map[string]models.MediaEnrichmentData
		voteData       map[string][3]int
		wg2            sync.WaitGroup
	)
	wg2.Add(2)

	go func() {
		defer wg2.Done()
		var err error
		enrichmentData, err = models.BatchEnrichMediaData(allSlugs, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
		if err != nil {
			log.Errorf("Error enriching media data: %v", err)
			enrichmentData = make(map[string]models.MediaEnrichmentData)
		}
	}()

	go func() {
		defer wg2.Done()
		var err error
		voteData, err = models.BatchGetMediaVotes(allSlugs)
		if err != nil {
			log.Errorf("Error fetching vote data: %v", err)
			voteData = make(map[string][3]int)
		}
	}()

	wg2.Wait()

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
			VoteScore:         votes[0],
			Upvotes:           votes[1],
			Downvotes:         votes[2],
		}
	}

	// Mark chapters as read for logged-in users
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

	return handleView(c, views.Home(enrichedRecentlyAdded, enrichedRecentlyUpdated, cfg.PremiumEarlyAccessDuration, latestUpdates, highlights, stats, unreadCount, notifications, readingActivity), unreadCount, notifications)
}

// HandleTopReadFull renders the Most Read section with sub-navigation
func HandleTopReadFull(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/")
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
		topRead, err = models.GetTopReadMedias("today", 6, accessibleLibraries)
	case "week":
		topRead, err = models.GetTopReadMedias("week", 6, accessibleLibraries)
	case "month":
		topRead, err = models.GetTopReadMedias("month", 6, accessibleLibraries)
	case "year":
		topRead, err = models.GetTopReadMedias("year", 6, accessibleLibraries)
	case "all":
		topRead, err = models.GetTopReadMedias("all", 6, accessibleLibraries)
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

// HandleTopReadCard renders the full trending card with read content and correct active tab
func HandleTopReadCard(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the home page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/")
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

	log.Debugf("Got %d read media for period %s", len(topRead), period)

	return renderComponent(c, views.TrendingCardContent("read", period, nil, topRead))
}

// HandleNotFound renders the generic not-found page for unrouted paths.
func HandleNotFound(c fiber.Ctx) error {
	return handleView(c, views.NotFound())
}
