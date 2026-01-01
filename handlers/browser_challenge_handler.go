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

	"github.com/alexander-bruun/magi/models"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
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
func HandleBrowserChallengeInit(c *fiber.Ctx) error {
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
func HandleBrowserChallengeVerify(c *fiber.Ctx) error {
	var response ChallengeResponse
	if err := c.BodyParser(&response); err != nil {
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

	log.Infof("Browser challenge solved by IP: %s", getRealIP(c))

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// IsBrowserChallengeValid checks if the current request has a valid browser challenge cookie
func IsBrowserChallengeValid(c *fiber.Ctx) bool {
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
	return func(c *fiber.Ctx) error {
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
	return func(c *fiber.Ctx) error {
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
		return c.Status(fiber.StatusOK).SendString(generateChallengePage(dataStr, signature, difficulty))
	}
}

// generateChallengePage creates a minimal HTML page that solves the challenge and reloads
func generateChallengePage(challenge, signature string, difficulty int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Checking your browser...</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0a0a0a;color:#fafafa;min-height:100vh;display:flex;align-items:center;justify-content:center}
.container{text-align:center;padding:2rem}
.spinner{width:40px;height:40px;border:3px solid #333;border-top-color:#3b82f6;border-radius:50%%;animation:spin 1s linear infinite;margin:0 auto 1rem}
@keyframes spin{to{transform:rotate(360deg)}}
h1{font-size:1.25rem;font-weight:500;margin-bottom:.5rem}
p{color:#a1a1aa;font-size:.875rem}
.error{color:#ef4444;margin-top:1rem;display:none}
</style>
</head>
<body>
<div class="container">
<div class="spinner" id="spinner"></div>
<h1>Checking your browser</h1>
<p>This will only take a moment...</p>
<p class="error" id="error">JavaScript is required to view this site.</p>
</div>
<script>
(async function(){
const challenge='%s';
const signature='%s';
const difficulty=%d;

async function sha256(m){
const b=new TextEncoder().encode(m);
const h=await crypto.subtle.digest('SHA-256',b);
return Array.from(new Uint8Array(h)).map(x=>x.toString(16).padStart(2,'0')).join('');
}

function fingerprint(){
const f=[];
f.push(screen.width+'x'+screen.height);
f.push(screen.colorDepth);
f.push(window.devicePixelRatio||1);
try{f.push(Intl.DateTimeFormat().resolvedOptions().timeZone)}catch(e){f.push('?')}
f.push(navigator.language);
f.push(navigator.platform);
f.push(navigator.hardwareConcurrency||0);
f.push(navigator.deviceMemory||0);
f.push(navigator.maxTouchPoints||0);
try{
const c=document.createElement('canvas');
const g=c.getContext('webgl')||c.getContext('experimental-webgl');
if(g){const d=g.getExtension('WEBGL_debug_renderer_info');if(d)f.push(g.getParameter(d.UNMASKED_RENDERER_WEBGL))}
}catch(e){}
try{
const c=document.createElement('canvas');c.width=200;c.height=50;
const x=c.getContext('2d');x.textBaseline='top';x.font='14px Arial';
x.fillStyle='#f60';x.fillRect(125,1,62,20);
x.fillStyle='#069';x.fillText('Cwm fjordbank',2,15);
f.push(c.toDataURL().slice(-50));
}catch(e){}
return f.join('|');
}

const target='0'.repeat(difficulty);
let answer=0;
let solved=false;
while(!solved){
for(let i=0;i<5000;i++){
const h=await sha256(challenge+':'+answer);
if(h.substring(0,difficulty)===target){
const fp=fingerprint();
const r=await fetch('/api/browser-challenge/verify',{
method:'POST',credentials:'same-origin',
headers:{'Content-Type':'application/json'},
body:JSON.stringify({nonce:challenge,solution:signature,answer:answer,fingerprint:fp})
});
if(r.ok){
solved=true;
await new Promise(r=>setTimeout(r,100));
const chk=await fetch('/api/browser-challenge/init',{method:'GET',credentials:'same-origin'});
if(chk.ok){const d=await chk.json();if(d.verified){location.reload();return}}
location.reload();return;
}
}
answer++;
}
await new Promise(r=>setTimeout(r,0));
}
})();
setTimeout(function(){document.getElementById('error').style.display='block';document.getElementById('spinner').style.display='none'},10000);
</script>
<noscript><style>.spinner{display:none}</style><p class="error" style="display:block">JavaScript is required to view this site.</p></noscript>
</body>
</html>`, challenge, signature, difficulty)
}

// GenerateChallengeScript returns JavaScript code that can be embedded in pages
// to automatically solve the browser challenge in the background
func GenerateChallengeScript() string {
	return `
(function() {
	'use strict';
	
	// Check if we already have a valid verification cookie
	// The actual cookie is httpOnly, so we check via API
	var bcInitialized = false;
	var bcSolving = false;
	
	async function sha256(message) {
		const msgBuffer = new TextEncoder().encode(message);
		const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
		const hashArray = Array.from(new Uint8Array(hashBuffer));
		return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
	}
	
	// Collect browser fingerprint - characteristics that scripts can't easily fake
	function collectFingerprint() {
		const fp = [];
		
		// Screen properties
		fp.push(screen.width + 'x' + screen.height);
		fp.push(screen.colorDepth);
		fp.push(window.devicePixelRatio || 1);
		
		// Timezone
		fp.push(Intl.DateTimeFormat().resolvedOptions().timeZone);
		
		// Language
		fp.push(navigator.language);
		
		// Platform
		fp.push(navigator.platform);
		
		// Hardware concurrency (CPU cores)
		fp.push(navigator.hardwareConcurrency || 0);
		
		// Device memory (if available)
		fp.push(navigator.deviceMemory || 0);
		
		// Touch support
		fp.push(navigator.maxTouchPoints || 0);
		
		// WebGL renderer (hard to fake)
		try {
			const canvas = document.createElement('canvas');
			const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
			if (gl) {
				const debugInfo = gl.getExtension('WEBGL_debug_renderer_info');
				if (debugInfo) {
					fp.push(gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL));
				}
			}
		} catch (e) {}
		
		// Canvas fingerprint (rendering differences between systems)
		try {
			const canvas = document.createElement('canvas');
			const ctx = canvas.getContext('2d');
			ctx.textBaseline = 'top';
			ctx.font = '14px Arial';
			ctx.fillText('Browser fingerprint test', 2, 2);
			fp.push(canvas.toDataURL().slice(-50)); // Last 50 chars of data URL
		} catch (e) {}
		
		return fp.join('|');
	}
	
	async function solveChallenge(challenge, difficulty) {
		const target = '0'.repeat(difficulty);
		let answer = 0;
		
		// Solve in batches to avoid blocking the main thread
		const batchSize = 10000;
		
		while (true) {
			for (let i = 0; i < batchSize; i++) {
				const hash = await sha256(challenge + ':' + answer);
				if (hash.startsWith(target)) {
					return answer;
				}
				answer++;
			}
			// Yield to the main thread
			await new Promise(resolve => setTimeout(resolve, 0));
		}
	}
	
	async function initBrowserChallenge() {
		if (bcInitialized || bcSolving) return;
		bcSolving = true;
		
		try {
			// Check if we need to solve a challenge
			const initResp = await fetch('/api/browser-challenge/init', {
				method: 'GET',
				credentials: 'same-origin'
			});
			
			if (!initResp.ok) {
				console.debug('Browser challenge init failed');
				bcSolving = false;
				return;
			}
			
			const data = await initResp.json();
			
			if (data.verified) {
				// Already verified
				bcInitialized = true;
				bcSolving = false;
				return;
			}
			
			// Collect browser fingerprint
			const fingerprint = collectFingerprint();
			
			// Solve the challenge
			const answer = await solveChallenge(data.challenge, data.difficulty);
			
			// Submit the solution with fingerprint
			const verifyResp = await fetch('/api/browser-challenge/verify', {
				method: 'POST',
				credentials: 'same-origin',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					nonce: data.challenge,
					solution: data.signature,
					answer: answer,
					fingerprint: fingerprint
				})
			});
			
			if (verifyResp.ok) {
				bcInitialized = true;
				console.debug('Browser challenge solved');
			}
		} catch (e) {
			console.debug('Browser challenge error:', e);
		}
		
		bcSolving = false;
	}
	
	// Start solving immediately
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', initBrowserChallenge);
	} else {
		initBrowserChallenge();
	}
	
	// Also solve on visibility change (in case tab was in background)
	document.addEventListener('visibilitychange', function() {
		if (document.visibilityState === 'visible' && !bcInitialized) {
			initBrowserChallenge();
		}
	});
	
	// Expose for manual retry
	window.__magiBC = { init: initBrowserChallenge, isVerified: function() { return bcInitialized; } };
})();
`
}
