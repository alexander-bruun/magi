package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

const (
	// Challenge cookie names
	browserChallengeCookie = "bc_verified"
	browserChallengeNonce  = "bc_nonce"
)

// ChallengeData represents the data embedded in a challenge
type ChallengeData struct {
	Nonce      string `json:"n"`
	Timestamp  int64  `json:"t"`
	IP         string `json:"i"`
	Difficulty int    `json:"d"` // Number of leading zeros required in hash
}

// ChallengeResponse represents the client's solution to the challenge
type ChallengeResponse struct {
	Nonce       string `json:"nonce"`
	Solution    string `json:"solution"`
	Answer      int64  `json:"answer"`
	Fingerprint string `json:"fingerprint"` // Browser fingerprint to bind the token
}

// generateChallengeSecret generates or retrieves a secret for challenge signing
func getChallengeSecret() string {
	cfg, err := models.GetAppConfig()
	if err != nil || cfg.ImageAccessSecret == "" {
		// Fallback to a generated secret if not configured
		return "magi-browser-challenge-default-secret"
	}
	return cfg.ImageAccessSecret + "-browser-challenge"
}

// generateNonce creates a cryptographically secure random nonce
func generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// signChallenge creates an HMAC signature for challenge data
func signChallenge(data string) string {
	h := hmac.New(sha256.New, []byte(getChallengeSecret()))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// verifySignature checks if a signature is valid for the given data
func verifySignature(data, signature string) bool {
	expected := signChallenge(data)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// HandleBrowserChallengeInit returns challenge data for invisible JS challenge
// This endpoint is called by the browser to get a new challenge to solve
func HandleBrowserChallengeInit(c fiber.Ctx) error {
	// Check if already verified
	if IsBrowserChallengeValid(c) {
		return c.JSON(fiber.Map{
			"verified": true,
		})
	}

	// Generate a new challenge
	nonce, err := generateNonce()
	if err != nil {
		log.Errorf("Failed to generate challenge nonce: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate challenge",
		})
	}

	ip := getRealIP(c)
	now := time.Now().Unix()

	// Get difficulty from config (default to 3 - relatively easy for invisible challenge)
	difficulty := 3
	cfg, err := models.GetAppConfig()
	if err == nil && cfg.BrowserChallengeDifficulty > 0 {
		difficulty = cfg.BrowserChallengeDifficulty
	}

	challengeData := ChallengeData{
		Nonce:      nonce,
		Timestamp:  now,
		IP:         ip,
		Difficulty: difficulty,
	}

	// Encode and sign the challenge
	dataBytes, _ := json.Marshal(challengeData)
	dataStr := base64.StdEncoding.EncodeToString(dataBytes)
	signature := signChallenge(dataStr)

	// Set the nonce cookie (short-lived, used for verification)
	c.Cookie(&fiber.Cookie{
		Name:     browserChallengeNonce,
		Value:    nonce,
		MaxAge:   300, // 5 minutes to solve
		HTTPOnly: true,
		Secure:   isSecureRequest(c),
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"verified":   false,
		"challenge":  dataStr,
		"signature":  signature,
		"difficulty": difficulty,
	})
}

// HandleBrowserChallengeVerify verifies the browser challenge solution
func HandleBrowserChallengeVerify(c fiber.Ctx) error {
	var response ChallengeResponse
	if err := c.Bind().Body(&response); err != nil {
		log.Debugf("Failed to parse challenge response: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Verify the nonce matches the cookie
	cookieNonce := c.Cookies(browserChallengeNonce)
	if cookieNonce == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Challenge expired or missing",
		})
	}

	// Decode the challenge data
	dataBytes, err := base64.StdEncoding.DecodeString(response.Nonce)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid challenge data",
		})
	}

	var challengeData ChallengeData
	if err := json.Unmarshal(dataBytes, &challengeData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid challenge format",
		})
	}

	// Verify the signature
	if !verifySignature(response.Nonce, response.Solution) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid challenge signature",
		})
	}

	// Verify the nonce matches
	if challengeData.Nonce != cookieNonce {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Nonce mismatch",
		})
	}

	// Verify the challenge hasn't expired (5 minutes)
	if time.Now().Unix()-challengeData.Timestamp > 300 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Challenge expired",
		})
	}

	// Verify the IP matches (optional, can be disabled for mobile users)
	cfg, err := models.GetAppConfig()
	if err == nil && cfg.BrowserChallengeIPBound {
		if challengeData.IP != getRealIP(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "IP address mismatch",
			})
		}
	}

	// Verify the proof-of-work solution
	// The client must find a number that when appended to the challenge data
	// produces a hash with the required number of leading zeros
	proofData := fmt.Sprintf("%s:%d", response.Nonce, response.Answer)
	hash := sha256.Sum256([]byte(proofData))
	hashHex := hex.EncodeToString(hash[:])

	requiredPrefix := strings.Repeat("0", challengeData.Difficulty)
	if !strings.HasPrefix(hashHex, requiredPrefix) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid proof-of-work solution",
		})
	}

	// Challenge solved! Set the verification cookie
	validityHours := 24
	if cfg.BrowserChallengeValidityHours > 0 {
		validityHours = cfg.BrowserChallengeValidityHours
	}

	// Create a hash of the User-Agent for binding (not storing full UA for privacy)
	uaHash := sha256.Sum256([]byte(c.Get("User-Agent")))
	uaHashStr := hex.EncodeToString(uaHash[:8]) // First 8 bytes = 16 hex chars

	// Create a hash of the browser fingerprint if provided
	fpHash := ""
	if response.Fingerprint != "" {
		fp := sha256.Sum256([]byte(response.Fingerprint))
		fpHash = hex.EncodeToString(fp[:8])
	}

	// Create a signed verification token with IP, timestamp, UA hash, and fingerprint hash
	// Format: ip:timestamp:nonce:uahash:fphash
	verificationData := fmt.Sprintf("%s:%d:%s:%s:%s", getRealIP(c), time.Now().Unix(), cookieNonce, uaHashStr, fpHash)
	encodedData := base64.StdEncoding.EncodeToString([]byte(verificationData))
	verificationToken := encodedData + "." + signChallenge(encodedData)

	c.Cookie(&fiber.Cookie{
		Name:     browserChallengeCookie,
		Value:    verificationToken,
		MaxAge:   validityHours * 3600,
		HTTPOnly: true,
		Secure:   isSecureRequest(c),
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})

	// Clear the nonce cookie
	c.Cookie(&fiber.Cookie{
		Name:     browserChallengeNonce,
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   isSecureRequest(c),
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
	})

	log.Debugf("Browser challenge solved by IP: %s", getRealIP(c))

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// IsBrowserChallengeValid checks if the current request has a valid browser challenge cookie
func IsBrowserChallengeValid(c fiber.Ctx) bool {
	token := c.Cookies(browserChallengeCookie)
	if token == "" {
		return false
	}

	// Parse the token
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	data, signature := parts[0], parts[1]

	// Verify signature
	if !verifySignature(data, signature) {
		return false
	}

	// Decode and verify the data
	dataBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return false
	}

	dataParts := strings.Split(string(dataBytes), ":")
	// Format: ip:timestamp:nonce:uahash:fphash (5 parts)
	// Legacy format: ip:timestamp:nonce (3 parts) - still supported
	if len(dataParts) < 3 {
		return false
	}

	// Verify timestamp hasn't expired
	timestamp, err := strconv.ParseInt(dataParts[1], 10, 64)
	if err != nil {
		return false
	}

	cfg, _ := models.GetAppConfig()
	validityHours := 24
	if cfg.BrowserChallengeValidityHours > 0 {
		validityHours = cfg.BrowserChallengeValidityHours
	}

	if time.Now().Unix()-timestamp > int64(validityHours*3600) {
		return false
	}

	// Optionally verify IP binding
	if cfg.BrowserChallengeIPBound {
		if dataParts[0] != getRealIP(c) {
			return false
		}
	}

	// Always verify User-Agent binding if present in token (new format)
	if len(dataParts) >= 4 && dataParts[3] != "" {
		uaHash := sha256.Sum256([]byte(c.Get("User-Agent")))
		uaHashStr := hex.EncodeToString(uaHash[:8])
		if dataParts[3] != uaHashStr {
			return false
		}
	}

	return true
}

// BrowserChallengeMiddleware enforces browser challenge verification on protected API endpoints
// This middleware is designed to be invisible - it only blocks API requests that require verification
// The actual challenge is solved by JavaScript running in the browser
func BrowserChallengeMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for browser challenge: %v", err)
			return c.Next() // Continue without challenge on error
		}

		// Skip if browser challenge is not enabled
		if !cfg.BrowserChallengeEnabled {
			return c.Next()
		}

		// Skip for privileged users (moderators/admins)
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Check if challenge is already verified
		if IsBrowserChallengeValid(c) {
			return c.Next()
		}

		// Block the request - the browser should solve the challenge first
		// Return 403 with a specific error code that the frontend can detect
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":         "Browser verification required",
			"code":          "BROWSER_CHALLENGE_REQUIRED",
			"challenge_url": "/api/browser-challenge/init",
		})
	}
}

// BrowserChallengePageMiddleware serves a challenge page for HTML requests without a valid cookie
// This middleware intercepts page requests and serves a lightweight challenge page that
// auto-redirects once solved. This prevents curl/scripts from getting real page content.
func BrowserChallengePageMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			log.Errorf("Failed to get app config for browser challenge: %v", err)
			return c.Next()
		}

		// Skip if browser challenge is not enabled
		if !cfg.BrowserChallengeEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		// Skip for API requests, static assets, and challenge endpoints
		path := c.Path()
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/captcha") ||
			path == "/robots.txt" ||
			path == "/health" ||
			path == "/ready" {
			return c.Next()
		}

		// Skip if not requesting HTML
		accept := c.Get("Accept")
		if !strings.Contains(accept, "text/html") && accept != "*/*" && accept != "" {
			return c.Next()
		}

		// Check if challenge is already verified
		if IsBrowserChallengeValid(c) {
			return c.Next()
		}

		// Generate challenge data for the inline page
		nonce, err := generateNonce()
		if err != nil {
			log.Errorf("Failed to generate challenge nonce: %v", err)
			return c.Next()
		}

		ip := getRealIP(c)
		now := time.Now().Unix()
		difficulty := 3
		if cfg.BrowserChallengeDifficulty > 0 {
			difficulty = cfg.BrowserChallengeDifficulty
		}

		challengeData := ChallengeData{
			Nonce:      nonce,
			Timestamp:  now,
			IP:         ip,
			Difficulty: difficulty,
		}

		dataBytes, _ := json.Marshal(challengeData)
		dataStr := base64.StdEncoding.EncodeToString(dataBytes)
		signature := signChallenge(dataStr)

		// Set the nonce cookie
		c.Cookie(&fiber.Cookie{
			Name:     browserChallengeNonce,
			Value:    nonce,
			MaxAge:   300,
			HTTPOnly: true,
			Secure:   isSecureRequest(c),
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
		})

		// Serve the inline challenge page
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
		return adaptor.HTTPHandler(templ.Handler(views.BrowserChallengePage(dataStr, signature, difficulty)))(c)
	}
}
