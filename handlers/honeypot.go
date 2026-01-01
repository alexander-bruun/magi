package handlers

import (
	"sync"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// HoneypotTrigger records when an IP accessed a honeypot
type HoneypotTrigger struct {
	IP          string
	Path        string
	TriggeredAt time.Time
	UserAgent   string
	Blocked     bool
}

// Honeypot paths that only bots would access (hidden from real users)
var honeypotPaths = []string{
	"/wp-admin",
	"/wp-login.php",
	"/administrator",
	"/phpmyadmin",
	"/login.php",
	"/config.php",
	"/xmlrpc.php",
	"/.env",
	"/.git/config",
	"/backup.sql",
	"/db.sql",
	"/dump.sql",
	"/api/v1/admin",
	"/api/debug",
	"/api/internal",
	"/actuator",
	"/actuator/health",
	"/console",
	"/shell",
	"/cmd",
	"/eval",
	"/phpinfo.php",
	"/info.php",
	"/test.php",
	"/setup.php",
	"/install.php",
	"/cgi-bin",
	"/.htaccess",
	"/.htpasswd",
	"/robots.txt.bak",
	"/sitemap.xml.bak",
	"/wp-content",
	"/wp-includes",
	"/wordpress",
	"/.svn",
	"/.git",
	"/admin.php",
	"/adminer.php",
}

var (
	honeypotTriggers    = make(map[string]*HoneypotTrigger)
	honeypotMu          sync.RWMutex
	blockedByHoneypot   = make(map[string]time.Time) // IP -> block expiry time
	blockedByHoneypotMu sync.RWMutex
)

// HoneypotMiddleware detects and blocks bots that access honeypot paths
func HoneypotMiddleware() fiber.Handler {
	// Start cleanup goroutine
	go honeypotCleanup()

	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if honeypot is not enabled
		if !cfg.HoneypotEnabled {
			return c.Next()
		}

		// Skip for privileged users (admins, moderators)
		if isPrivilegedUser(c) {
			return c.Next()
		}

		ip := getRealIP(c)
		path := c.Path()

		// Check if IP is already blocked
		if isBlockedByHoneypot(ip) {
			log.Warnf("Honeypot: Blocked request from previously trapped IP %s", ip)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		// Check if this is a honeypot path
		if isHoneypotPath(path) {
			log.Warnf("Honeypot: Trap triggered by IP %s on path %s", ip, path)

			// Record the trigger
			honeypotMu.Lock()
			honeypotTriggers[ip] = &HoneypotTrigger{
				IP:          ip,
				Path:        path,
				TriggeredAt: time.Now(),
				UserAgent:   c.Get("User-Agent"),
				Blocked:     cfg.HoneypotAutoBlock,
			}
			honeypotMu.Unlock()

			// Auto-block if enabled
			if cfg.HoneypotAutoBlock {
				blockDuration := time.Duration(cfg.HoneypotBlockDuration) * time.Minute
				blockedByHoneypotMu.Lock()
				blockedByHoneypot[ip] = time.Now().Add(blockDuration)
				blockedByHoneypotMu.Unlock()

				log.Warnf("Honeypot: IP %s blocked for %d minutes", ip, cfg.HoneypotBlockDuration)
			}

			// Return a fake response to waste bot's time
			return honeypotResponse(c, path)
		}

		return c.Next()
	}
}

// isHoneypotPath checks if the path matches a honeypot
func isHoneypotPath(path string) bool {
	lowerPath := toLower(path)

	for _, hp := range honeypotPaths {
		if lowerPath == toLower(hp) || (len(lowerPath) > len(hp) && lowerPath[:len(hp)] == toLower(hp)) {
			return true
		}
	}

	return false
}

// isBlockedByHoneypot checks if an IP is currently blocked
func isBlockedByHoneypot(ip string) bool {
	blockedByHoneypotMu.RLock()
	defer blockedByHoneypotMu.RUnlock()

	expiry, exists := blockedByHoneypot[ip]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Block expired, will be cleaned up later
		return false
	}

	return true
}

// honeypotResponse returns a fake response to waste bot time
func honeypotResponse(c *fiber.Ctx, path string) error {
	// Add a small delay to waste bot resources
	time.Sleep(2 * time.Second)

	// Return a convincing but useless response based on the path
	if containsString(path, "admin") || containsString(path, "login") {
		// Fake login page
		return c.Status(fiber.StatusOK).SendString(`<!DOCTYPE html>
<html>
<head><title>Login</title></head>
<body>
<form method="post">
<input type="text" name="username" placeholder="Username">
<input type="password" name="password" placeholder="Password">
<button type="submit">Login</button>
</form>
</body>
</html>`)
	}

	if containsString(path, ".php") || containsString(path, ".sql") || containsString(path, ".env") {
		// Return 404 for sensitive file probes
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	// Default response
	return c.Status(fiber.StatusForbidden).SendString("Forbidden")
}

// honeypotCleanup removes expired blocks and old triggers
func honeypotCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Clean up expired blocks
		blockedByHoneypotMu.Lock()
		for ip, expiry := range blockedByHoneypot {
			if now.After(expiry) {
				delete(blockedByHoneypot, ip)
			}
		}
		blockedByHoneypotMu.Unlock()

		// Clean up old triggers (keep for 24 hours for logging/analysis)
		honeypotMu.Lock()
		for ip, trigger := range honeypotTriggers {
			if now.Sub(trigger.TriggeredAt) > 24*time.Hour {
				delete(honeypotTriggers, ip)
			}
		}
		honeypotMu.Unlock()
	}
}

// GetHoneypotStats returns statistics about honeypot activity (for admin dashboard)
func GetHoneypotStats() map[string]interface{} {
	honeypotMu.RLock()
	triggersCount := len(honeypotTriggers)
	honeypotMu.RUnlock()

	blockedByHoneypotMu.RLock()
	blockedCount := len(blockedByHoneypot)
	blockedByHoneypotMu.RUnlock()

	return map[string]interface{}{
		"triggers_24h":      triggersCount,
		"currently_blocked": blockedCount,
		"honeypot_paths":    len(honeypotPaths),
	}
}

// GetRecentHoneypotTriggers returns recent honeypot triggers (for admin dashboard)
func GetRecentHoneypotTriggers() []HoneypotTrigger {
	honeypotMu.RLock()
	defer honeypotMu.RUnlock()

	triggers := make([]HoneypotTrigger, 0, len(honeypotTriggers))
	for _, trigger := range honeypotTriggers {
		triggers = append(triggers, *trigger)
	}

	return triggers
}
