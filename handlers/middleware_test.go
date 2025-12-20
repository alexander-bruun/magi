package handlers

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestIsSecureRequest(t *testing.T) {
	app := fiber.New()

	tests := []struct {
		name     string
		setup    func(*fiber.Ctx)
		expected bool
	}{
		{
			name: "HTTPS protocol",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-Proto", "https")
			},
			expected: true,
		},
		{
			name: "X-Forwarded-SSL on",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-SSL", "on")
			},
			expected: true,
		},
		{
			name: "X-Forwarded-SSL 1",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-SSL", "1")
			},
			expected: true,
		},
		{
			name: "HTTP protocol",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-Proto", "http")
			},
			expected: false,
		},
		{
			name: "No headers",
			setup: func(c *fiber.Ctx) {
				// No setup
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			test.setup(c)
			result := isSecureRequest(c)
			assert.Equal(t, test.expected, result)
			app.ReleaseCtx(c)
		})
	}
}

func TestGetRealIP(t *testing.T) {
	app := fiber.New()

	tests := []struct {
		name     string
		setup    func(*fiber.Ctx)
		expected string
	}{
		{
			name: "X-Forwarded-For single IP",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-For", "192.168.1.100")
			},
			expected: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For multiple IPs",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
			},
			expected: "192.168.1.100",
		},
		{
			name: "X-Real-IP",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Real-IP", "10.0.0.1")
			},
			expected: "10.0.0.1",
		},
		{
			name: "X-Forwarded-For takes precedence",
			setup: func(c *fiber.Ctx) {
				c.Request().Header.Set("X-Forwarded-For", "192.168.1.100")
				c.Request().Header.Set("X-Real-IP", "10.0.0.1")
			},
			expected: "192.168.1.100",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			test.setup(c)
			result := getRealIP(c)
			assert.Equal(t, test.expected, result)
			app.ReleaseCtx(c)
		})
	}
}

func TestIsBotBehavior(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		accesses      []time.Time
		maxAccesses   int
		windowSeconds int
		expected      bool
	}{
		{
			name:          "below threshold",
			accesses:      []time.Time{now.Add(-10 * time.Second)},
			maxAccesses:   5,
			windowSeconds: 60,
			expected:      false,
		},
		{
			name: "old accesses ignored",
			accesses: []time.Time{
				now.Add(-120 * time.Second), // Too old
				now.Add(-10 * time.Second),
				now.Add(-20 * time.Second),
				now.Add(-30 * time.Second),
				now.Add(-40 * time.Second),
			},
			maxAccesses:   5,
			windowSeconds: 60,
			expected:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isBotBehavior(test.accesses, test.maxAccesses, test.windowSeconds)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestFilterTimesAfter(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	times := []time.Time{
		baseTime.Add(-10 * time.Minute), // Before
		baseTime.Add(-5 * time.Minute),  // Before
		baseTime.Add(-1 * time.Minute),  // After
		baseTime,                        // After
		baseTime.Add(5 * time.Minute),   // After
	}

	filtered := filterTimesAfter(times, baseTime.Add(-2*time.Minute))
	expected := []time.Time{
		baseTime.Add(-1 * time.Minute),
		baseTime,
		baseTime.Add(5 * time.Minute),
	}

	assert.Equal(t, expected, filtered)
}

func TestGetCurrentUsername(t *testing.T) {
	app := fiber.New()

	// Test with username set
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Locals("user_name", "testuser")
	result := GetCurrentUsername(c)
	assert.Equal(t, "testuser", result)
	app.ReleaseCtx(c)

	// Test without username
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	result = GetCurrentUsername(c)
	assert.Equal(t, "", result)
	app.ReleaseCtx(c)

	// Test with wrong type
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Locals("user_name", 123)
	result = GetCurrentUsername(c)
	assert.Equal(t, "", result)
	app.ReleaseCtx(c)
}

func TestCleanupTracker(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker := &IPTracker{
		SeriesAccesses: []time.Time{
			baseTime.Add(-120 * time.Second), // 2 minutes ago - should be removed
			baseTime.Add(-30 * time.Second),  // 30 seconds ago - should be kept
			baseTime.Add(-10 * time.Second),  // 10 seconds ago - should be kept
		},
		ChapterAccesses: []time.Time{
			baseTime.Add(-90 * time.Second),  // 1.5 minutes ago - should be removed
			baseTime.Add(-20 * time.Second),  // 20 seconds ago - should be kept
		},
	}

	cleanupTracker(tracker, baseTime)

	// Check SeriesAccesses (window is 60 seconds)
	expectedSeries := []time.Time{
		baseTime.Add(-30 * time.Second),
		baseTime.Add(-10 * time.Second),
	}
	assert.Equal(t, expectedSeries, tracker.SeriesAccesses)

	// Check ChapterAccesses
	expectedChapters := []time.Time{
		baseTime.Add(-20 * time.Second),
	}
	assert.Equal(t, expectedChapters, tracker.ChapterAccesses)
}

func TestClearSessionCookie(t *testing.T) {
	app := fiber.New()

	tests := []struct {
		name         string
		isSecure     bool
		expectedSecure bool
	}{
		{
			name:         "secure request",
			isSecure:     true,
			expectedSecure: true,
		},
		{
			name:         "insecure request",
			isSecure:     false,
			expectedSecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			if tt.isSecure {
				c.Request().Header.Set("X-Forwarded-Proto", "https")
			}

			clearSessionCookie(c)

			cookies := c.Response().Header.PeekCookie("session_token")
			assert.NotEmpty(t, cookies, "Cookie should be set")

			// Parse the cookie to verify properties
			cookieStr := string(cookies)
			assert.Contains(t, cookieStr, "session_token=")
			assert.Contains(t, cookieStr, "HttpOnly")
			assert.Contains(t, cookieStr, "SameSite=Lax")
			assert.Contains(t, cookieStr, "path=/")

			if tt.expectedSecure {
				assert.Contains(t, cookieStr, "secure")
			} else {
				assert.NotContains(t, cookieStr, "secure")
			}
		})
	}
}

func TestSetSessionCookie(t *testing.T) {
	app := fiber.New()

	tests := []struct {
		name         string
		isSecure     bool
		sessionToken string
		expectedSecure bool
	}{
		{
			name:         "secure request",
			isSecure:     true,
			sessionToken: "test-token-123",
			expectedSecure: true,
		},
		{
			name:         "insecure request",
			isSecure:     false,
			sessionToken: "test-token-456",
			expectedSecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			if tt.isSecure {
				c.Request().Header.Set("X-Forwarded-Proto", "https")
			}

			setSessionCookie(c, tt.sessionToken)

			cookies := c.Response().Header.PeekCookie("session_token")
			assert.NotEmpty(t, cookies, "Cookie should be set")

			// Parse the cookie to verify properties
			cookieStr := string(cookies)
			assert.Contains(t, cookieStr, "session_token="+tt.sessionToken)
			assert.Contains(t, cookieStr, "HttpOnly")
			assert.Contains(t, cookieStr, "SameSite=Lax")
			assert.Contains(t, cookieStr, "path=/")
			assert.Contains(t, cookieStr, "max-age=")

			if tt.expectedSecure {
				assert.Contains(t, cookieStr, "secure")
			} else {
				assert.NotContains(t, cookieStr, "secure")
			}
		})
	}
}
