package handlers

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// BehaviorEntry tracks behavioral signals for a session
type BehaviorEntry struct {
	SessionID     string
	IP            string
	MouseMoved    bool // Has mouse movement been detected
	Scrolled      bool // Has scrolling been detected
	KeyPressed    bool // Has key press been detected
	PageViews     int  // Number of page views
	ResourceLoads int  // Number of resource loads (images, css, js)
	HumanScore    int  // 0-100 score (higher = more likely human)
	LastActivity  time.Time
	CreatedAt     time.Time
}

// BehaviorSignal represents client-side behavior data sent via API
type BehaviorSignal struct {
	MouseMoved     bool    `json:"mouse_moved"`
	Scrolled       bool    `json:"scrolled"`
	KeyPressed     bool    `json:"key_pressed"`
	MousePositions int     `json:"mouse_positions"` // Number of unique positions
	ScrollDepth    float64 `json:"scroll_depth"`    // 0-1 percentage
	TimeOnPage     int     `json:"time_on_page"`    // milliseconds
	Clicks         int     `json:"clicks"`
	TouchEvents    int     `json:"touch_events"`
}

var (
	behaviorStore = make(map[string]*BehaviorEntry)
	behaviorMu    sync.RWMutex
)

// BehavioralAnalysisMiddleware tracks and analyzes user behavior patterns
func BehavioralAnalysisMiddleware() fiber.Handler {
	// Start cleanup goroutine
	go behaviorCleanup()

	return func(c *fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if behavioral analysis is not enabled
		if !cfg.BehavioralAnalysisEnabled {
			return c.Next()
		}

		// Skip for privileged users
		if isPrivilegedUser(c) {
			return c.Next()
		}

		ip := getRealIP(c)
		sessionID := c.Cookies("_sess")
		if sessionID == "" {
			sessionID = ip // Fallback to IP if no session
		}

		behaviorMu.Lock()
		entry, exists := behaviorStore[sessionID]
		if !exists {
			entry = &BehaviorEntry{
				SessionID:    sessionID,
				IP:           ip,
				HumanScore:   50, // Start neutral
				LastActivity: time.Now(),
				CreatedAt:    time.Now(),
			}
			behaviorStore[sessionID] = entry
		}

		// Track page view
		path := c.Path()
		if !isStaticAssetPath(path) && !isAPIPath(path) {
			entry.PageViews++
		} else if isStaticAssetPath(path) {
			entry.ResourceLoads++
		}

		entry.LastActivity = time.Now()

		// Calculate human score
		calculateHumanScore(entry)

		isSuspicious := entry.HumanScore < cfg.BehavioralScoreThreshold
		behaviorMu.Unlock()

		if isSuspicious {
			c.Locals("suspicious_behavior", true)
		}

		// Store entry reference for potential updates
		c.Locals("behavior_entry", sessionID)

		return c.Next()
	}
}

// HandleBehaviorSignal receives client-side behavior signals
func HandleBehaviorSignal(c *fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil || !cfg.BehavioralAnalysisEnabled {
		return c.JSON(fiber.Map{"status": "ok"})
	}

	var signal BehaviorSignal
	if err := json.Unmarshal(c.Body(), &signal); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid signal data",
		})
	}

	ip := getRealIP(c)
	sessionID := c.Cookies("_sess")
	if sessionID == "" {
		sessionID = ip
	}

	behaviorMu.Lock()
	defer behaviorMu.Unlock()

	entry, exists := behaviorStore[sessionID]
	if !exists {
		entry = &BehaviorEntry{
			SessionID:    sessionID,
			IP:           ip,
			HumanScore:   50,
			LastActivity: time.Now(),
			CreatedAt:    time.Now(),
		}
		behaviorStore[sessionID] = entry
	}

	// Update entry with signal data
	if signal.MouseMoved {
		entry.MouseMoved = true
	}
	if signal.Scrolled {
		entry.Scrolled = true
	}
	if signal.KeyPressed {
		entry.KeyPressed = true
	}

	// Recalculate human score with new data
	calculateHumanScoreWithSignal(entry, &signal)

	entry.LastActivity = time.Now()

	log.Debugf("Behavior signal received for session %s: score=%d", sessionID, entry.HumanScore)

	return c.JSON(fiber.Map{
		"status": "ok",
		"score":  entry.HumanScore,
	})
}

// calculateHumanScore calculates a score based on available behavior data
func calculateHumanScore(entry *BehaviorEntry) {
	score := 50 // Start neutral

	// Session age bonus (bots often have short sessions)
	sessionAge := time.Since(entry.CreatedAt)
	if sessionAge > 10*time.Second {
		score += 5
	}
	if sessionAge > 1*time.Minute {
		score += 10
	}

	// Mouse movement is a strong human indicator
	if entry.MouseMoved {
		score += 15
	}

	// Scrolling is a human indicator
	if entry.Scrolled {
		score += 10
	}

	// Key press is a human indicator
	if entry.KeyPressed {
		score += 10
	}

	// Resource loading ratio (humans load CSS, JS, images)
	if entry.PageViews > 0 && entry.ResourceLoads > 0 {
		ratio := float64(entry.ResourceLoads) / float64(entry.PageViews)
		if ratio > 5 {
			score += 10 // Good ratio of resources to pages
		}
	}

	// Penalty for too many rapid page views (potential scraping)
	if entry.PageViews > 20 && sessionAge < 2*time.Minute {
		score -= 20
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	entry.HumanScore = score
}

// calculateHumanScoreWithSignal includes client-side signal data
func calculateHumanScoreWithSignal(entry *BehaviorEntry, signal *BehaviorSignal) {
	score := entry.HumanScore

	// Mouse positions indicate real mouse movement
	if signal.MousePositions > 10 {
		score += 10
	}
	if signal.MousePositions > 50 {
		score += 10
	}

	// Scroll depth indicates reading behavior
	if signal.ScrollDepth > 0.3 {
		score += 5
	}
	if signal.ScrollDepth > 0.7 {
		score += 10
	}

	// Time on page indicates engagement
	if signal.TimeOnPage > 5000 { // 5 seconds
		score += 5
	}
	if signal.TimeOnPage > 30000 { // 30 seconds
		score += 10
	}

	// Clicks indicate interaction
	if signal.Clicks > 0 {
		score += 5
	}

	// Touch events (mobile users)
	if signal.TouchEvents > 0 {
		score += 10
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	entry.HumanScore = score
}

// isAPIPath checks if the path is an API endpoint
func isAPIPath(path string) bool {
	return len(path) >= 5 && path[:5] == "/api/"
}

// behaviorCleanup removes old behavior entries
func behaviorCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		behaviorMu.Lock()
		now := time.Now()
		for sessionID, entry := range behaviorStore {
			// Remove entries inactive for more than 2 hours
			if now.Sub(entry.LastActivity) > 2*time.Hour {
				delete(behaviorStore, sessionID)
			}
		}
		behaviorMu.Unlock()
	}
}

// GetBehaviorStats returns statistics about behavioral analysis (for admin dashboard)
func GetBehaviorStats() map[string]interface{} {
	behaviorMu.RLock()
	defer behaviorMu.RUnlock()

	totalCount := len(behaviorStore)
	humanCount := 0
	suspiciousCount := 0
	averageScore := 0

	for _, entry := range behaviorStore {
		averageScore += entry.HumanScore
		if entry.HumanScore >= 60 {
			humanCount++
		} else if entry.HumanScore < 40 {
			suspiciousCount++
		}
	}

	if totalCount > 0 {
		averageScore = averageScore / totalCount
	}

	return map[string]interface{}{
		"total_sessions":      totalCount,
		"likely_human":        humanCount,
		"suspicious_sessions": suspiciousCount,
		"average_score":       averageScore,
	}
}

// GetBehaviorJS returns JavaScript code for client-side behavior tracking
func GetBehaviorJS() string {
	return `
(function() {
	var data = {
		mouse_moved: false,
		scrolled: false,
		key_pressed: false,
		mouse_positions: 0,
		scroll_depth: 0,
		time_on_page: 0,
		clicks: 0,
		touch_events: 0
	};
	var start = Date.now();
	var positions = new Set();
	var maxScroll = 0;
	
	document.addEventListener('mousemove', function(e) {
		data.mouse_moved = true;
		positions.add(Math.floor(e.clientX/50) + ',' + Math.floor(e.clientY/50));
		data.mouse_positions = positions.size;
	});
	
	document.addEventListener('scroll', function() {
		data.scrolled = true;
		var scrolled = window.scrollY + window.innerHeight;
		var total = document.documentElement.scrollHeight;
		data.scroll_depth = Math.max(data.scroll_depth, scrolled / total);
	});
	
	document.addEventListener('keydown', function() {
		data.key_pressed = true;
	});
	
	document.addEventListener('click', function() {
		data.clicks++;
	});
	
	document.addEventListener('touchstart', function() {
		data.touch_events++;
	});
	
	function sendSignal() {
		data.time_on_page = Date.now() - start;
		fetch('/api/behavior-signal', {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(data),
			keepalive: true
		}).catch(function(){});
	}
	
	// Send signal periodically and before unload
	setInterval(sendSignal, 30000);
	window.addEventListener('beforeunload', sendSignal);
	setTimeout(sendSignal, 5000); // Initial signal after 5 seconds
})();
`
}
