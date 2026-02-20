package handlers

import (
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
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
	honeypotTriggerEntries = NewTTLStore[HoneypotTrigger](
		24*time.Hour, 10*time.Minute,
		func(e *HoneypotTrigger) time.Time { return e.TriggeredAt },
	)
	blockedByHoneypotEntries = NewTTLStore[time.Time](
		24*time.Hour, 10*time.Minute,
		func(e *time.Time) time.Time { return *e },
	)
)

// HoneypotMiddleware detects and blocks bots that access honeypot paths
func HoneypotMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
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

		// Check if IP is already blocked temporarily
		if isBlockedByHoneypot(ip) {
			log.Warnf("Honeypot: Blocked request from previously trapped IP %s", ip)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		// Check if IP is permanently banned
		if banned, err := models.IsIPBanned(ip); err != nil {
			log.Errorf("Error checking if IP %s is banned: %v", ip, err)
		} else if banned {
			log.Warnf("Honeypot: Blocked request from banned IP %s", ip)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		// Check if this is a honeypot path
		if isHoneypotPath(path) {
			log.Warnf("Honeypot: Trap triggered by IP %s on path %s", ip, path)

			// Record the trigger
			honeypotTriggerEntries.Set(ip, &HoneypotTrigger{
				IP:          ip,
				Path:        path,
				TriggeredAt: time.Now(),
				UserAgent:   c.Get("User-Agent"),
				Blocked:     cfg.HoneypotAutoBan || cfg.HoneypotAutoBlock,
			})

			// Auto-ban if enabled
			if cfg.HoneypotAutoBan {
				banDuration := cfg.HoneypotBlockDuration * 60 // convert minutes to seconds
				if err := models.BanIP(ip, "Triggered honeypot", banDuration); err != nil {
					log.Errorf("Failed to ban IP %s: %v", ip, err)
				} else {
					log.Warnf("Honeypot: IP %s banned for %d minutes for triggering honeypot", ip, cfg.HoneypotBlockDuration)
				}
			} else if cfg.HoneypotAutoBlock {
				// Auto-block temporarily if enabled
				blockExpiry := time.Now().Add(time.Duration(cfg.HoneypotBlockDuration) * time.Minute)
				blockedByHoneypotEntries.Set(ip, &blockExpiry)

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
	lowerPath := strings.ToLower(path)

	for _, hp := range honeypotPaths {
		hpLower := strings.ToLower(hp)
		if lowerPath == hpLower || (len(lowerPath) > len(hp) && lowerPath[:len(hp)] == hpLower) {
			return true
		}
	}

	return false
}

// isBlockedByHoneypot checks if an IP is currently blocked
func isBlockedByHoneypot(ip string) bool {
	expiry, ok := blockedByHoneypotEntries.Get(ip)
	if !ok {
		return false
	}
	if time.Now().After(*expiry) {
		blockedByHoneypotEntries.Delete(ip)
		return false
	}
	return true
}

// honeypotResponse returns a fake response to waste bot time
func honeypotResponse(c fiber.Ctx, path string) error {
	// Add a small delay to waste bot resources
	time.Sleep(2 * time.Second)

	// Return a convincing but useless response based on the path
	if strings.Contains(path, "admin") || strings.Contains(path, "login") {
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

	if strings.Contains(path, ".php") || strings.Contains(path, ".sql") || strings.Contains(path, ".env") {
		// Return 404 for sensitive file probes
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	// Default response
	return c.Status(fiber.StatusForbidden).SendString("Forbidden")
}

// GetHoneypotStats returns statistics about honeypot activity (for admin dashboard)
func GetHoneypotStats() map[string]interface{} {
	return map[string]interface{}{
		"triggers_24h":      honeypotTriggerEntries.Len(),
		"currently_blocked": blockedByHoneypotEntries.Len(),
		"honeypot_paths":    len(honeypotPaths),
	}
}

// GetRecentHoneypotTriggers returns recent honeypot triggers (for admin dashboard)
func GetRecentHoneypotTriggers() []HoneypotTrigger {
	triggers := make([]HoneypotTrigger, 0)
	honeypotTriggerEntries.Range(func(_ string, trigger *HoneypotTrigger) bool {
		triggers = append(triggers, *trigger)
		return true
	})
	return triggers
}
