package handlers

import (
	"regexp"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// HeaderAnalysisMiddleware checks for common browser headers and suspicious patterns
func HeaderAnalysisMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if header analysis is not enabled
		if !cfg.HeaderAnalysisEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Skip for API endpoints and static assets
		path := c.Path()
		if isStaticAssetPath(path) || isAPIPath(path) {
			return c.Next()
		}

		suspicionScore := 0
		ip := getRealIP(c)

		// Check for missing common browser headers
		if c.Get("Accept") == "" {
			suspicionScore += 2
		}
		if c.Get("Accept-Language") == "" {
			suspicionScore += 3
		}
		if c.Get("Accept-Encoding") == "" {
			suspicionScore += 2
		}

		// Check User-Agent
		ua := c.Get("User-Agent")
		if ua == "" {
			suspicionScore += 5
		} else {
			// Check for bot-like User-Agent patterns
			uaScore := analyzeUserAgent(ua)
			suspicionScore += uaScore
		}

		// Check Sec-Fetch-* headers (modern browsers)
		// These are hard to fake as they're automatically set by browsers
		if c.Get("Sec-Fetch-Mode") == "" && c.Get("Sec-Fetch-Site") == "" {
			// Older browsers might not have these, so only minor penalty
			suspicionScore += 1
		}

		// Check for header consistency
		consistencyScore := checkHeaderConsistency(c)
		suspicionScore += consistencyScore

		// Check for suspicious header values
		valueScore := checkSuspiciousHeaderValues(c)
		suspicionScore += valueScore

		// If suspicion score is high, flag or block
		if suspicionScore >= cfg.HeaderAnalysisThreshold {
			log.Warnf("Header analysis: Suspicious headers from IP %s (score: %d)", ip, suspicionScore)
			c.Locals("suspicious_headers", true)

			// If strict mode, block the request
			if cfg.HeaderAnalysisStrict {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Request blocked",
				})
			}
		}

		return c.Next()
	}
}

// analyzeUserAgent checks for bot-like User-Agent patterns
func analyzeUserAgent(ua string) int {
	score := 0
	lowerUA := strings.ToLower(ua)

	// Known bot/scraper patterns
	botPatterns := []string{
		"curl", "wget", "python", "scrapy", "httpclient",
		"java/", "libwww", "lwp-", "php/", "perl", "ruby",
		"go-http", "axios", "node-fetch", "undici", "got/",
		"postman", "insomnia", "httpie", "aiohttp",
	}

	for _, pattern := range botPatterns {
		if strings.Contains(lowerUA, pattern) {
			score += 5
			break
		}
	}

	// Generic bot patterns
	genericBotPatterns := []string{
		"bot", "crawler", "spider", "scraper", "headless",
	}

	for _, pattern := range genericBotPatterns {
		if strings.Contains(lowerUA, pattern) {
			// Some are legitimate (Googlebot, etc.) so lower penalty
			score += 2
			break
		}
	}

	// Check for fake browser UA (claims to be browser but has anomalies)
	if strings.Contains(lowerUA, "mozilla") {
		// Check for missing expected components
		if !strings.Contains(lowerUA, "applewebkit") && !strings.Contains(lowerUA, "gecko") && !strings.Contains(lowerUA, "trident") {
			score += 2
		}
	}

	// Very short User-Agent is suspicious
	if len(ua) < 20 {
		score += 2
	}

	// Very long User-Agent is suspicious
	if len(ua) > 500 {
		score += 2
	}

	return score
}

// checkHeaderConsistency checks for inconsistencies between headers
func checkHeaderConsistency(c fiber.Ctx) int {
	score := 0
	ua := strings.ToLower(c.Get("User-Agent"))
	acceptLanguage := c.Get("Accept-Language")

	// If UA claims to be Chrome but Accept-Language format is unusual
	if strings.Contains(ua, "chrome") {
		// Chrome typically sends Accept-Language with quality values
		if acceptLanguage != "" && !strings.Contains(acceptLanguage, ",") && !strings.Contains(acceptLanguage, ";") {
			score += 1
		}
	}

	// If UA claims to be from Windows but has Linux-style headers
	if strings.Contains(ua, "windows") {
		// Some HTTP libraries leave platform-specific traces
		// This is a heuristic check
	}

	// Check Accept header format
	accept := c.Get("Accept")
	if accept != "" && !strings.Contains(accept, "/") {
		// Invalid Accept header format
		score += 3
	}

	return score
}

// checkSuspiciousHeaderValues checks for suspicious header values
func checkSuspiciousHeaderValues(c fiber.Ctx) int {
	score := 0

	// Check for localhost/internal references in headers (proxy bypass attempts)
	headersToCheck := []string{"Host", "X-Forwarded-Host", "X-Real-IP"}
	suspiciousPatterns := regexp.MustCompile(`(?i)(localhost|127\.0\.0\.1|192\.168\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])\.)`)

	for _, headerName := range headersToCheck {
		value := c.Get(headerName)
		if value != "" && suspiciousPatterns.MatchString(value) {
			// Only flag if it's trying to impersonate internal traffic
			if headerName != "Host" { // Host can be localhost legitimately
				score += 2
			}
		}
	}

	// Check for header injection attempts
	allHeaders := ""
	c.Request().Header.VisitAll(func(key, value []byte) {
		allHeaders += string(value)
	})

	// Check for newlines in header values (header injection)
	if strings.Contains(allHeaders, "\n") || strings.Contains(allHeaders, "\r") {
		score += 5
	}

	return score
}

// GetHeaderAnalysisStats returns statistics about header analysis (for admin dashboard)
func GetHeaderAnalysisStats() map[string]interface{} {
	// Header analysis is stateless, so we return general info
	return map[string]interface{}{
		"status": "active",
	}
}
