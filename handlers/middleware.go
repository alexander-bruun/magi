package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

const (
	sessionTokenDuration = 30 * 24 * time.Hour // 1 month
)

var roleHierarchy = map[string]int{
	"reader":    1,
	"moderator": 2,
	"admin":     3,
}

// Bot detection constants
const (
	maxSeriesAccesses   = 5  // Max series accesses per time window
	maxChapterAccesses  = 10  // Max chapter accesses per time window
	accessTimeWindow    = 60  // Time window in seconds
	cleanupInterval     = 300 // Cleanup old entries every 5 minutes
)

// Rate limiting tracking
var (
	requestCounts = make(map[string][]time.Time)
	requestsMu    sync.RWMutex
)

// IPTracker tracks access patterns for an IP
type IPTracker struct {
	SeriesAccesses  []time.Time
	ChapterAccesses []time.Time
	LastCleanup     time.Time
}

var (
	ipTrackers = make(map[string]*IPTracker)
	trackersMu sync.RWMutex
)

// AuthMiddleware handles session token validation
func AuthMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Cookies("session_token")

		if sessionToken != "" {
			if err := validateSessionToken(c, sessionToken, requiredRole); err == nil {
				return c.Next()
			}
		}

		originalURL := c.OriginalURL()
		return c.Redirect("/auth/login?target="+url.QueryEscape(originalURL), fiber.StatusSeeOther)
	}
}

func validateSessionToken(c *fiber.Ctx, sessionToken, requiredRole string) error {
	username, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return fiber.ErrUnauthorized
	}

	return validateUserRole(c, username, requiredRole)
}

func validateUserRole(c *fiber.Ctx, userName, requiredRole string) error {
	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return fiber.ErrUnauthorized
	}

	if roleHierarchy[user.Role] < roleHierarchy[requiredRole] {
		return fiber.ErrForbidden
	}

	if user.Banned {
		return fiber.ErrForbidden
	}

	c.Locals("user_name", userName)
	return nil
}

func clearSessionCookie(c *fiber.Ctx) {
	expiredTime := time.Now().Add(-time.Hour)
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  expiredTime,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
}

func setSessionCookie(c *fiber.Ctx, sessionToken string) {
	secure := isSecureRequest(c)
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(sessionTokenDuration),
		MaxAge:   int(sessionTokenDuration.Seconds()),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})
}

// isPrivilegedUser checks if the current request is from a moderator or admin user
func isPrivilegedUser(c *fiber.Ctx) bool {
	sessionToken := c.Cookies("session_token")
	if sessionToken == "" {
		return false
	}

	username, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return false
	}

	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return false
	}

	return user.Role == "moderator" || user.Role == "admin"
}

// RateLimitingMiddleware limits the number of requests per IP
func RateLimitingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for rate limiting: %v", err)
			return c.Next() // Continue without rate limiting on error
		}
		
		if !cfg.RateLimitEnabled {
			return c.Next()
		}

		// Bypass rate limiting for moderators and admins
		if isPrivilegedUser(c) {
			return c.Next()
		}
		
		ip := getRealIP(c)
		now := time.Now()
		
		requestsMu.Lock()
		times, exists := requestCounts[ip]
		if !exists {
			times = []time.Time{}
		}
		
		// Clean old requests outside the window
		windowStart := now.Add(-time.Duration(cfg.RateLimitWindow) * time.Second)
		var validTimes []time.Time
		for _, t := range times {
			if t.After(windowStart) {
				validTimes = append(validTimes, t)
			}
		}
		
		// Check if limit exceeded
		if len(validTimes) >= cfg.RateLimitRequests {
			requestsMu.Unlock()
			log.Infof("Rate limit exceeded for IP: %s", ip)
			
			// Calculate time until reset
			var resetTime time.Time
			if len(validTimes) > 0 {
				// Find the oldest request
				oldest := validTimes[0]
				for _, t := range validTimes {
					if t.Before(oldest) {
						oldest = t
					}
				}
				resetTime = oldest.Add(time.Duration(cfg.RateLimitWindow) * time.Second)
			} else {
				resetTime = now.Add(time.Duration(cfg.RateLimitWindow) * time.Second)
			}
			timeRemaining := resetTime.Sub(now)
			seconds := int(timeRemaining.Seconds())
			if seconds < 0 {
				seconds = 0
			}
			
			// Return HTML with error page
			html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dim">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Rate Limited</title>
    <link rel="stylesheet" href="/assets/css/vendor/core.min.css">
    <link rel="stylesheet" href="/assets/css/vendor/utilities.min.css">
    <link rel="stylesheet" href="/assets/css/styles.css">
</head>
<body class="bg-background text-foreground">
    <section class="flex items-center justify-center min-h-screen">
        <div class="py-8 px-4 mx-auto max-w-screen-xl lg:py-16 lg:px-6">
            <div class="mx-auto max-w-screen-sm text-center">
                <h1 class="mb-4 text-7xl tracking-tight font-extrabold lg:text-9xl text-primary-600 dark:text-primary-500">429</h1>
                <p class="mb-4 text-3xl tracking-tight font-bold text-gray-900 md:text-4xl dark:text-white">Too Many Requests</p>
                <p class="mb-4 text-lg font-light text-gray-500 dark:text-gray-400">Whoa buddy, you are a bit too fast! Try again in <strong>%d seconds</strong>.</p>
                <p class="mb-4 text-lg font-light text-gray-500 dark:text-gray-400">This helps protect the server from abuse.</p>
                <a href="/" class="inline-flex text-white bg-primary-600 hover:bg-primary-800 focus:ring-4 focus:outline-none focus:ring-primary-300 font-medium rounded-lg text-sm px-5 py-2.5 text-center dark:focus:ring-primary-900 my-4">Back to Homepage</a>
            </div>
        </div>
    </section>
</body>
</html>`, seconds)
			
			return c.Type("html").SendString(html)
		}
		
		// Add current request
		validTimes = append(validTimes, now)
		requestCounts[ip] = validTimes
		requestsMu.Unlock()
		
		return c.Next()
	}
}

// isSecureRequest returns true if the request is using HTTPS or forwarded as HTTPS.
func isSecureRequest(c *fiber.Ctx) bool {
	if c.Secure() || c.Protocol() == "https" {
		return true
	}
	// Respect common proxy headers
	if proto := c.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}
	if https := c.Get("X-Forwarded-SSL"); https == "on" || https == "1" {
		return true
	}
	return false
}

// getRealIP extracts the real client IP from the request, considering proxies
func getRealIP(c *fiber.Ctx) string {
	// Check X-Forwarded-For header (common with proxies/load balancers)
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := c.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteIP
	return c.IP()
}

func BotDetectionMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for bot detection: %v", err)
			return c.Next() // Continue without bot detection on error
		}
		
		if !cfg.BotDetectionEnabled {
			return c.Next()
		}

		// Bypass bot detection for moderators and admins
		if isPrivilegedUser(c) {
			return c.Next()
		}
		
		ip := getRealIP(c)
		log.Debugf("Bot detection for IP: %s, Path: %s", ip, c.Path())

		// Check if IP is already banned
		banned, err := models.IsIPBanned(ip)
		if err != nil {
			log.Errorf("Error checking if IP %s is banned: %v", ip, err)
			c.Locals("bot_check_error", err)
		} else if banned {
			log.Infof("Blocking banned IP: %s", ip)
			return c.Status(fiber.StatusForbidden).SendString("Access denied")
		}

		// Track access if it's a media or chapter request
		path := c.Path()
		if strings.HasPrefix(path, "/series/") {
			trackersMu.Lock()
			tracker := getOrCreateTracker(ip)
			now := time.Now()

			if strings.Contains(path, "/chapters/") {
				// Chapter access
				tracker.ChapterAccesses = append(tracker.ChapterAccesses, now)
				if isBotBehavior(tracker.ChapterAccesses, cfg.BotChapterThreshold, cfg.BotDetectionWindow) {
					log.Infof("Banning IP %s for excessive chapter accesses", ip)
					models.BanIP(ip, "Excessive chapter accesses")
					// Continue processing the request - ban is for future requests
				}
			} else if !strings.Contains(path, "/search") && !strings.Contains(path, "/tags") {
				// Series access (not search or tags)
				tracker.SeriesAccesses = append(tracker.SeriesAccesses, now)
				if isBotBehavior(tracker.SeriesAccesses, cfg.BotSeriesThreshold, cfg.BotDetectionWindow) {
					log.Infof("Banning IP %s for excessive series accesses", ip)
					models.BanIP(ip, "Excessive series accesses")
					// Continue processing the request - ban is for future requests
				}
			}

			// Cleanup old entries periodically
			if now.Sub(tracker.LastCleanup) > time.Duration(cleanupInterval)*time.Second {
				cleanupTracker(tracker, now)
				tracker.LastCleanup = now
			}

			trackersMu.Unlock()
		}

		return c.Next()
	}
}

// getOrCreateTracker gets or creates an IPTracker for the given IP
func getOrCreateTracker(ip string) *IPTracker {
	if tracker, exists := ipTrackers[ip]; exists {
		return tracker
	}
	tracker := &IPTracker{
		SeriesAccesses:  []time.Time{},
		ChapterAccesses: []time.Time{},
		LastCleanup:     time.Now(),
	}
	ipTrackers[ip] = tracker
	return tracker
}

// isBotBehavior checks if the access pattern indicates bot behavior
func isBotBehavior(accesses []time.Time, maxAccesses int, windowSeconds int) bool {
	if len(accesses) < maxAccesses {
		return false
	}
	now := time.Now()
	windowStart := now.Add(-time.Duration(windowSeconds) * time.Second)
	count := 0
	for _, t := range accesses {
		if t.After(windowStart) {
			count++
			if count >= maxAccesses {
				return true
			}
		}
	}
	return false
}

// cleanupTracker removes old access times from the tracker
func cleanupTracker(tracker *IPTracker, now time.Time) {
	windowStart := now.Add(-time.Duration(accessTimeWindow) * time.Second)
	tracker.SeriesAccesses = filterTimesAfter(tracker.SeriesAccesses, windowStart)
	tracker.ChapterAccesses = filterTimesAfter(tracker.ChapterAccesses, windowStart)
}

// filterTimesAfter filters a slice of times to only include those after the given time
func filterTimesAfter(times []time.Time, after time.Time) []time.Time {
	var filtered []time.Time
	for _, t := range times {
		if t.After(after) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
// but does not enforce authentication. It sets c.Locals("user_name") when a valid
// token is found so handlers can optionally adapt views for logged-in users.
func OptionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Cookies("session_token")

		if sessionToken != "" {
			// Try to validate; ignore errors for optional auth
			_ = validateSessionToken(c, sessionToken, "reader")
		}

		return c.Next()
	}
}

// ConditionalAuthMiddleware attempts to authenticate a user if session cookie is present,
// and falls back to anonymous role permissions for unauthenticated users.
// If anonymous users have no permissions, it enforces authentication.
func ConditionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to authenticate if session cookie is present
		sessionToken := c.Cookies("session_token")
		if sessionToken != "" {
			if err := validateSessionToken(c, sessionToken, "reader"); err == nil {
				return c.Next()
			}
		}

		// No authenticated user - check if anonymous users have any permissions
		// If they do, allow access; otherwise redirect to login
		libraries, err := models.GetAccessibleLibrariesForAnonymous()
		if err != nil {
			// If we can't get anonymous permissions, fail open
			return c.Next()
		}

		// If anonymous has no permissions, require authentication
		if len(libraries) == 0 {
			originalURL := c.OriginalURL()
			return c.Redirect("/auth/login?target="+url.QueryEscape(originalURL), fiber.StatusSeeOther)
		}

		// Anonymous has permissions, allow access
		return c.Next()
	}
}

// GetCurrentUsername retrieves the username from the fiber context
func GetCurrentUsername(c *fiber.Ctx) string {
	username, ok := c.Locals("user_name").(string)
	if !ok {
		return ""
	}
	return username
}

// GetUserAccessibleLibraries returns the library slugs accessible to the current user
// Returns libraries based on role permissions for authenticated users or anonymous permissions for unauthenticated users
func GetUserAccessibleLibraries(c *fiber.Ctx) ([]string, error) {
	username := GetCurrentUsername(c)
	
	// If no user is authenticated, return anonymous role permissions
	if username == "" {
		return models.GetAccessibleLibrariesForAnonymous()
	}
	
	// Check user role
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return []string{}, err
	}
	
	// Admins and moderators have access to all libraries
	if user.Role == "admin" || user.Role == "moderator" {
		libraries, err := models.GetLibraries()
		if err != nil {
			return nil, err
		}
		
		slugs := make([]string, len(libraries))
		for i, lib := range libraries {
			slugs[i] = lib.Slug
		}
		return slugs, nil
	}
	
	// Regular users - get accessible libraries based on permissions
	return models.GetAccessibleLibrariesForUser(username)
}

// UserHasLibraryAccess checks if the current user has access to a specific library
func UserHasLibraryAccess(c *fiber.Ctx, librarySlug string) (bool, error) {
	username := GetCurrentUsername(c)
	
	// If no user is authenticated, check anonymous role permissions
	if username == "" {
		return models.AnonymousHasLibraryAccess(librarySlug)
	}
	
	// Check user role
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return false, err
	}
	
	// Admins and moderators have access to all libraries
	if user.Role == "admin" || user.Role == "moderator" {
		return true, nil
	}
	
	// Regular users - check permissions
	return models.UserHasLibraryAccess(username, librarySlug)
}
