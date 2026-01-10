package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// TLSFingerprintEntry stores information about a TLS fingerprint
type TLSFingerprintEntry struct {
	Fingerprint  string
	UserAgent    string
	FirstSeen    time.Time
	LastSeen     time.Time
	RequestCount int
	IsTrusted    bool
}

// Known browser TLS fingerprints (simplified - real implementation would have many more)
// These are common JA3 hashes for popular browsers
var knownBrowserFingerprints = map[string]string{
	// Chrome fingerprints
	"769,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0": "Chrome",
	"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0":    "Chrome",
	// Firefox fingerprints
	"771,49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0": "Firefox",
	// Safari fingerprints
	"771,4865-4866-4867-49196-49195-52393-49200-49199-52392-49162-49161-49172-49171-157-156-53-47-49160-49170-10,0-23-65281-10-11-16-5-13-18-51-45-43-27,29-23-24,0": "Safari",
}

// Known bot/script TLS fingerprints
var knownBotFingerprints = map[string]string{
	"769,47-53-5-10-49161-49162-49171-49172-50-56-19-4,0-10-11,23-24-25,0":                                                    "Python requests",
	"769,49162-49161-52393-49200-49199-49172-49171-52392-47-53,0-23-65281-10-11-35-16-13,29-23-24,0":                          "Go HTTP client",
	"769,49195-49199-49196-49200-52393-52392-52244-52243-49171-49172-156-157-47-53,65281-0-23-35-13-5-16-11-10,29-23-24-25,0": "curl",
}

var (
	fingerprintStore = make(map[string]*TLSFingerprintEntry)
	fingerprintMu    sync.RWMutex
)

// TLSFingerprintMiddleware analyzes TLS fingerprints to detect non-browser clients
// Note: This requires TLS termination to happen at the Go application level,
// or the fingerprint to be passed via a header from a reverse proxy
func TLSFingerprintMiddleware() fiber.Handler {
	// Start cleanup goroutine
	go fingerprintCleanup()

	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if TLS fingerprinting is not enabled
		if !cfg.TLSFingerprintEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Get TLS fingerprint from header (set by reverse proxy like nginx/cloudflare)
		// Common header names: X-JA3-Fingerprint, CF-JA3-Hash, X-TLS-Fingerprint
		fingerprint := c.Get("X-JA3-Fingerprint")
		if fingerprint == "" {
			fingerprint = c.Get("CF-JA3-Hash")
		}
		if fingerprint == "" {
			fingerprint = c.Get("X-TLS-Fingerprint")
		}

		// If no fingerprint available, generate a pseudo-fingerprint from headers
		if fingerprint == "" {
			fingerprint = generatePseudoFingerprint(c)
		}

		if fingerprint == "" {
			return c.Next()
		}

		ip := getRealIP(c)
		ua := c.Get("User-Agent")

		// Check against known fingerprints
		isSuspicious := checkFingerprint(fingerprint, ua, ip)

		if isSuspicious {
			log.Warnf("TLS fingerprint: Suspicious fingerprint detected for IP %s: %s", ip, fingerprint)
			c.Locals("suspicious_tls", true)

			// If strict mode, block the request
			if cfg.TLSFingerprintStrict {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Request blocked",
				})
			}
		}

		return c.Next()
	}
}

// generatePseudoFingerprint creates a fingerprint based on available HTTP/2 and header characteristics
func generatePseudoFingerprint(c *fiber.Ctx) string {
	// Build a fingerprint from available request characteristics
	var components []string

	// Protocol version
	components = append(components, c.Protocol())

	// Header order fingerprint (browsers have consistent header ordering)
	headerOrder := getHeaderOrder(c)
	components = append(components, headerOrder)

	// Accept headers fingerprint
	components = append(components, c.Get("Accept"))
	components = append(components, c.Get("Accept-Language"))
	components = append(components, c.Get("Accept-Encoding"))

	// Connection header
	components = append(components, c.Get("Connection"))

	// Sec-* headers (modern browsers)
	components = append(components, c.Get("Sec-Fetch-Dest"))
	components = append(components, c.Get("Sec-Fetch-Mode"))
	components = append(components, c.Get("Sec-Fetch-Site"))

	// Create hash
	combined := strings.Join(components, "|")
	hash := md5.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// getHeaderOrder returns a string representing the order of headers
func getHeaderOrder(c *fiber.Ctx) string {
	// Get all request headers in order
	var headers []string
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers = append(headers, string(key))
	})
	return strings.Join(headers, ",")
}

// checkFingerprint checks if a fingerprint is suspicious
func checkFingerprint(fingerprint, userAgent, ip string) bool {
	fingerprintMu.Lock()
	defer fingerprintMu.Unlock()

	entry, exists := fingerprintStore[fingerprint]
	if !exists {
		entry = &TLSFingerprintEntry{
			Fingerprint:  fingerprint,
			UserAgent:    userAgent,
			FirstSeen:    time.Now(),
			LastSeen:     time.Now(),
			RequestCount: 1,
			IsTrusted:    false,
		}
		fingerprintStore[fingerprint] = entry

		// Check against known fingerprints
		if browser, ok := knownBrowserFingerprints[fingerprint]; ok {
			entry.IsTrusted = true
			log.Debugf("TLS fingerprint: Known browser fingerprint (%s) from IP %s", browser, ip)
			return false
		}

		if bot, ok := knownBotFingerprints[fingerprint]; ok {
			log.Warnf("TLS fingerprint: Known bot fingerprint (%s) from IP %s", bot, ip)
			return true
		}

		// Unknown fingerprint - not suspicious on first sight
		// Without real JA3 fingerprints, we can't reliably distinguish browsers
		// Mark as potentially trusted since browsers have varied fingerprints
		entry.IsTrusted = true
		return false
	}

	// Update existing entry
	entry.LastSeen = time.Now()
	entry.RequestCount++

	// Check for User-Agent inconsistency (same fingerprint, different UA)
	// Only flag if same fingerprint appears with very different UAs
	if entry.RequestCount > 10 && entry.UserAgent != userAgent {
		// Check if both are browser UAs - that's normal (UA can have minor variations)
		if !isBrowserUserAgent(entry.UserAgent) || !isBrowserUserAgent(userAgent) {
			// Suspicious: fingerprint switching between browser and non-browser UA
			log.Warnf("TLS fingerprint: UA type mismatch for fingerprint %s from IP %s", fingerprint, ip)
			return true
		}
	}

	return false
}

// isBrowserUserAgent checks if the UA claims to be a common browser
func isBrowserUserAgent(ua string) bool {
	browserPatterns := []string{
		"Mozilla/5.0",
		"Chrome/",
		"Firefox/",
		"Safari/",
		"Edge/",
		"Opera/",
	}

	lowerUA := strings.ToLower(ua)
	for _, pattern := range browserPatterns {
		if strings.Contains(lowerUA, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// fingerprintCleanup removes old fingerprint entries
func fingerprintCleanup() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fingerprintMu.Lock()
		now := time.Now()
		for fp, entry := range fingerprintStore {
			// Remove entries not seen in the last 24 hours
			if now.Sub(entry.LastSeen) > 24*time.Hour {
				delete(fingerprintStore, fp)
			}
		}
		fingerprintMu.Unlock()
	}
}

// GetTLSFingerprintStats returns statistics about TLS fingerprinting (for admin dashboard)
func GetTLSFingerprintStats() map[string]any {
	fingerprintMu.RLock()
	defer fingerprintMu.RUnlock()

	totalCount := len(fingerprintStore)
	trustedCount := 0
	suspiciousCount := 0

	for _, entry := range fingerprintStore {
		if entry.IsTrusted {
			trustedCount++
		} else {
			suspiciousCount++
		}
	}

	return map[string]any{
		"total_fingerprints":      totalCount,
		"trusted_fingerprints":    trustedCount,
		"suspicious_fingerprints": suspiciousCount,
	}
}
