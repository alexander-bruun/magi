package handlers

import (
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// TarpitEntry tracks suspicious activity for an IP
type TarpitEntry struct {
	SuspicionScore int       // Accumulated suspicion score
	LastSeen       time.Time // Last request time
	LastDelay      int       // Last delay applied in milliseconds
}

var tarpitEntries = NewTTLStore[TarpitEntry](
	time.Hour, 5*time.Minute,
	func(e *TarpitEntry) time.Time { return e.LastSeen },
)

// TarpitMiddleware progressively slows responses for suspected bots
// It tracks suspicious behavior and applies increasing delays to repeat offenders
func TarpitMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if tarpit is not enabled
		if !cfg.TarpitEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Skip for static assets
		path := c.Path()
		if isStaticAssetPath(path) {
			return c.Next()
		}

		ip := getRealIP(c)

		// Get or create tarpit entry
		entry := tarpitEntries.GetOrCreate(ip, func() *TarpitEntry {
			return &TarpitEntry{
				SuspicionScore: 0,
				LastSeen:       time.Now(),
				LastDelay:      0,
			}
		})

		// Calculate suspicion score increase based on request patterns
		scoreIncrease := calculateSuspicionScore(c, entry)
		entry.SuspicionScore += scoreIncrease
		entry.LastSeen = time.Now()

		// Apply decay for legitimate-looking traffic (reduces score over time)
		applyScoreDecay(entry)

		// Calculate delay based on suspicion score
		delay := calculateDelay(entry.SuspicionScore, cfg.TarpitMaxDelay)
		entry.LastDelay = delay

		// Apply the delay if significant
		if delay > 0 {
			log.Debugf("Tarpit: Applying %dms delay to IP %s (score: %d)", delay, ip, entry.SuspicionScore)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		return c.Next()
	}
}

// calculateSuspicionScore determines how suspicious a request is
func calculateSuspicionScore(c fiber.Ctx, entry *TarpitEntry) int {
	score := 0

	// Fast repeated requests (less than 100ms apart)
	if time.Since(entry.LastSeen) < 100*time.Millisecond {
		score += 5
	} else if time.Since(entry.LastSeen) < 500*time.Millisecond {
		score += 2
	}

	// Missing common browser headers
	if c.Get("Accept-Language") == "" {
		score += 1
	}
	if c.Get("Accept-Encoding") == "" {
		score += 1
	}

	// Suspicious User-Agent patterns
	ua := c.Get("User-Agent")
	if ua == "" {
		score += 3
	} else if isSuspiciousUserAgent(ua) {
		score += 2
	}

	// Marked as suspicious by referer validation
	if c.Locals("suspicious_referer") == true {
		score += 2
	}

	// HEAD requests are often used by bots/crawlers
	if c.Method() == "HEAD" {
		score += 1
	}

	return score
}

// applyScoreDecay reduces the suspicion score over time for legitimate traffic
func applyScoreDecay(entry *TarpitEntry) {
	// Decay 1 point per 10 seconds of inactivity
	secondsSinceLastSeen := int(time.Since(entry.LastSeen).Seconds())
	decay := secondsSinceLastSeen / 10

	if decay > 0 && entry.SuspicionScore > 0 {
		entry.SuspicionScore -= decay
		if entry.SuspicionScore < 0 {
			entry.SuspicionScore = 0
		}
	}
}

// calculateDelay returns the delay in milliseconds based on suspicion score
func calculateDelay(score, maxDelay int) int {
	if score < 10 {
		return 0 // No delay for low suspicion
	}

	// Exponential delay: starts at 100ms, doubles every 10 points
	delay := 100
	for s := 10; s < score; s += 10 {
		delay *= 2
	}

	// Cap at maximum delay
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}

	return delay
}


// isSuspiciousUserAgent checks for bot-like User-Agent patterns
func isSuspiciousUserAgent(ua string) bool {
	suspiciousPatterns := []string{
		"curl",
		"wget",
		"python",
		"scrapy",
		"httpclient",
		"java/",
		"libwww",
		"lwp-",
		"php/",
		"ruby",
		"perl",
		"go-http",
	}

	lowerUA := strings.ToLower(ua)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerUA, pattern) {
			return true
		}
	}
	return false
}

// GetTarpitStats returns statistics about the tarpit (for admin dashboard)
func GetTarpitStats() map[string]any {
	activeCount := 0
	highSuspicionCount := 0
	maxScore := 0

	tarpitEntries.Range(func(_ string, entry *TarpitEntry) bool {
		activeCount++
		if entry.SuspicionScore > 50 {
			highSuspicionCount++
		}
		if entry.SuspicionScore > maxScore {
			maxScore = entry.SuspicionScore
		}
		return true
	})

	return map[string]any{
		"active_entries":       activeCount,
		"high_suspicion_count": highSuspicionCount,
		"max_suspicion_score":  maxScore,
	}
}
