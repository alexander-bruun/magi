package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTimestamps(t *testing.T) {
	before := time.Now()
	timestamp := NewTimestamps()
	after := time.Now()

	// Check that timestamps are set to current time (within reasonable bounds)
	assert.True(t, timestamp.CreatedAt.After(before) || timestamp.CreatedAt.Equal(before))
	assert.True(t, timestamp.CreatedAt.Before(after) || timestamp.CreatedAt.Equal(after))
	assert.True(t, timestamp.UpdatedAt.After(before) || timestamp.UpdatedAt.Equal(before))
	assert.True(t, timestamp.UpdatedAt.Before(after) || timestamp.UpdatedAt.Equal(after))

	// Check that CreatedAt and UpdatedAt are equal (set at the same time)
	assert.Equal(t, timestamp.CreatedAt, timestamp.UpdatedAt)
}

func TestTimestampPair_UpdateTimestamp(t *testing.T) {
	// Create initial timestamp
	initial := NewTimestamps()
	initialCreatedAt := initial.CreatedAt

	// Wait a bit to ensure time difference
	time.Sleep(1 * time.Millisecond)

	// Update timestamp
	initial.UpdateTimestamp()

	// Check that CreatedAt remains unchanged
	assert.Equal(t, initialCreatedAt, initial.CreatedAt)

	// Check that UpdatedAt is updated (should be after CreatedAt)
	assert.True(t, initial.UpdatedAt.After(initial.CreatedAt))
}

func TestTimestampPair_UnixTimestamps(t *testing.T) {
	// Create timestamp with known values
	createdTime := time.Unix(1640995200, 0) // 2022-01-01 00:00:00 UTC
	updatedTime := time.Unix(1641081600, 0) // 2022-01-02 00:00:00 UTC

	timestamp := TimestampPair{
		CreatedAt: createdTime,
		UpdatedAt: updatedTime,
	}

	createdUnix, updatedUnix := timestamp.UnixTimestamps()

	assert.Equal(t, int64(1640995200), createdUnix)
	assert.Equal(t, int64(1641081600), updatedUnix)
}

func TestTimestampPair_FromUnixTimestamps(t *testing.T) {
	var timestamp TimestampPair

	createdUnix := int64(1640995200) // 2022-01-01 00:00:00 UTC
	updatedUnix := int64(1641081600) // 2022-01-02 00:00:00 UTC

	timestamp.FromUnixTimestamps(createdUnix, updatedUnix)

	expectedCreated := time.Unix(1640995200, 0)
	expectedUpdated := time.Unix(1641081600, 0)

	assert.Equal(t, expectedCreated, timestamp.CreatedAt)
	assert.Equal(t, expectedUpdated, timestamp.UpdatedAt)
}

func TestTimestampPair_RoundTrip(t *testing.T) {
	// Test round-trip conversion
	original := NewTimestamps()

	// Convert to Unix timestamps
	createdUnix, updatedUnix := original.UnixTimestamps()

	// Create new timestamp pair and populate from Unix
	var roundTrip TimestampPair
	roundTrip.FromUnixTimestamps(createdUnix, updatedUnix)

	// Should be identical (Unix timestamps have second precision)
	assert.Equal(t, original.CreatedAt.Unix(), roundTrip.CreatedAt.Unix())
	assert.Equal(t, original.UpdatedAt.Unix(), roundTrip.UpdatedAt.Unix())
}