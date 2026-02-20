package handlers

import (
	"math"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// TimingEntry tracks request timing for an IP
type TimingEntry struct {
	Timestamps    []time.Time // Recent request timestamps
	Intervals     []int64     // Intervals between requests in milliseconds
	SuspicionFlag bool        // Whether this IP has been flagged for suspicious timing
	LastAnalyzed  time.Time   // When the timing was last analyzed
}

const (
	maxTimestamps     = 100 // Max timestamps to keep per IP
	timingWindowHours = 1   // How long to keep timing data
)

var timingEntries = NewTTLStore[TimingEntry](
	time.Duration(timingWindowHours)*time.Hour, 10*time.Minute,
	func(e *TimingEntry) time.Time {
		if len(e.Timestamps) > 0 {
			return e.Timestamps[len(e.Timestamps)-1]
		}
		return time.Time{}
	},
)

// RequestTimingMiddleware analyzes request timing patterns to detect bots
func RequestTimingMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := models.GetAppConfig()
		if err != nil {
			return c.Next()
		}

		// Skip if timing analysis is not enabled
		if !cfg.TimingAnalysisEnabled {
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
		now := time.Now()

		entry := timingEntries.GetOrCreate(ip, func() *TimingEntry {
			return &TimingEntry{
				Timestamps:   make([]time.Time, 0, maxTimestamps),
				Intervals:    make([]int64, 0, maxTimestamps-1),
				LastAnalyzed: now,
			}
		})

		// Calculate interval from last request
		if len(entry.Timestamps) > 0 {
			lastTimestamp := entry.Timestamps[len(entry.Timestamps)-1]
			interval := now.Sub(lastTimestamp).Milliseconds()
			entry.Intervals = append(entry.Intervals, interval)

			// Keep only recent intervals
			if len(entry.Intervals) > maxTimestamps-1 {
				entry.Intervals = entry.Intervals[1:]
			}
		}

		// Add current timestamp
		entry.Timestamps = append(entry.Timestamps, now)
		if len(entry.Timestamps) > maxTimestamps {
			entry.Timestamps = entry.Timestamps[1:]
		}

		// Analyze timing patterns periodically (not every request)
		shouldAnalyze := len(entry.Timestamps) >= 10 && time.Since(entry.LastAnalyzed) > 30*time.Second
		if shouldAnalyze {
			entry.SuspicionFlag = analyzeTimingPatterns(entry.Intervals, cfg.TimingVarianceThreshold)
			entry.LastAnalyzed = now

			if entry.SuspicionFlag {
				log.Warnf("Timing analysis: Suspicious pattern detected for IP %s", ip)
				// Flag for tarpit to pick up
				c.Locals("suspicious_timing", true)
			}
		}

		isSuspicious := entry.SuspicionFlag

		// If flagged, we could block or just let tarpit handle it
		if isSuspicious {
			c.Locals("suspicious_timing", true)
		}

		return c.Next()
	}
}

// analyzeTimingPatterns checks for bot-like timing characteristics
func analyzeTimingPatterns(intervals []int64, varianceThreshold float64) bool {
	if len(intervals) < 5 {
		return false
	}

	// Calculate mean and standard deviation of intervals
	var sum int64
	for _, interval := range intervals {
		sum += interval
	}
	mean := float64(sum) / float64(len(intervals))

	// Calculate variance
	var varianceSum float64
	for _, interval := range intervals {
		diff := float64(interval) - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(intervals))
	stdDev := math.Sqrt(variance)

	// Calculate coefficient of variation (CV)
	// Low CV = consistent timing = likely bot
	cv := stdDev / mean
	if cv < varianceThreshold {
		// Very consistent timing - suspicious
		return true
	}

	// Check for unnaturally fast requests (average < 100ms between requests)
	if mean < 100 {
		return true
	}

	// Check for machine-like precision (many identical intervals)
	identicalCount := countIdenticalIntervals(intervals)
	if identicalCount > len(intervals)/2 {
		return true
	}

	return false
}

// countIdenticalIntervals counts how many intervals are within 10ms of each other
func countIdenticalIntervals(intervals []int64) int {
	if len(intervals) < 2 {
		return 0
	}

	// Group intervals by similarity
	groups := make(map[int64]int) // interval bucket -> count

	for _, interval := range intervals {
		// Bucket to nearest 10ms
		bucket := (interval / 10) * 10
		groups[bucket]++
	}

	// Find largest group
	maxCount := 0
	for _, count := range groups {
		if count > maxCount {
			maxCount = count
		}
	}

	return maxCount
}

// GetTimingStats returns statistics about timing analysis (for admin dashboard)
func GetTimingStats() map[string]interface{} {
	activeCount := 0
	suspiciousCount := 0

	timingEntries.Range(func(_ string, entry *TimingEntry) bool {
		activeCount++
		if entry.SuspicionFlag {
			suspiciousCount++
		}
		return true
	})

	return map[string]interface{}{
		"active_entries":   activeCount,
		"suspicious_count": suspiciousCount,
	}
}
