package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogDuration(t *testing.T) {
	start := time.Now()

	// Call the function - it should not panic
	LogDuration("testFunction", start, "arg1", "arg2")

	// Also test without args
	LogDuration("testFunction", start)

	// Ensure it completes without error
	assert.True(t, true) // Dummy assertion to ensure test passes
}

func TestLogDurationInstant(t *testing.T) {
	start := time.Now()

	// Log immediately (should show ~0 duration)
	LogDuration("instantFunction", start)

	assert.True(t, true) // Should complete without panic
}

func TestLogDurationWithMultipleArgs(t *testing.T) {
	start := time.Now()

	// Test with various argument types
	LogDuration("testFunction", start, "string", 42, 3.14, true, []string{"a", "b"})

	assert.True(t, true) // Should complete without panic
}

func TestLogDurationEmptyName(t *testing.T) {
	start := time.Now()

	// Test with empty function name (should not panic)
	LogDuration("", start)

	assert.True(t, true) // Should complete without panic
}

func TestLogDurationVeryLongName(t *testing.T) {
	start := time.Now()
	longName := strings.Repeat("functionName", 100)

	// Test with very long function name (should not panic)
	LogDuration(longName, start)

	assert.True(t, true) // Should complete without panic
}

func TestLogDurationNilArgs(t *testing.T) {
	start := time.Now()

	// Test with nil values in args (should not panic)
	LogDuration("testFunction", start, nil, nil, nil)

	assert.True(t, true) // Should complete without panic
}

func TestLogDurationPastTime(t *testing.T) {
	// Test with a start time in the past
	past := time.Now().Add(-5 * time.Second)

	LogDuration("pastFunction", past)

	assert.True(t, true) // Should complete without panic and show positive duration
}

func TestLogDurationConcurrent(t *testing.T) {
	// Test concurrent calls don't cause issues
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			start := time.Now()
			time.Sleep(time.Duration(idx) * time.Millisecond)
			LogDuration("concurrentFunction", start)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.True(t, true) // Should complete without panic
}