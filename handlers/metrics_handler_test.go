package handlers

import (
	"testing"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/stretchr/testify/assert"
)

func TestMarshallIfOk(t *testing.T) {
	tests := []struct {
		name     string
		dataName string
		data     interface{}
		expected string
	}{
		{
			name:     "valid data",
			dataName: "test",
			data:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "empty map",
			dataName: "empty",
			data:     map[string]string{},
			expected: "{}",
		},
		{
			name:     "nil data",
			dataName: "nil",
			data:     nil,
			expected: "null",
		},
		{
			name:     "complex data",
			dataName: "complex",
			data:     []int{1, 2, 3},
			expected: "[1,2,3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := marshallIfOk(tt.dataName, tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSeriesToMap(t *testing.T) {
	series := []models.SeriesData{
		{Name: "Series A", Count: 10},
		{Name: "Series B", Count: 25},
		{Name: "Series C", Count: 5},
	}

	result := seriesToMap(series)
	expected := map[string]int{
		"Series A": 10,
		"Series B": 25,
		"Series C": 5,
	}

	assert.Equal(t, expected, result)
}

func TestSeriesToMapEmpty(t *testing.T) {
	series := []models.SeriesData{}
	result := seriesToMap(series)
	assert.Empty(t, result)
}

func TestGroupUsersByCreationDate(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	users := []models.User{
		{CreatedAt: baseTime},
		{CreatedAt: baseTime.AddDate(0, 0, 1)}, // Next day
		{CreatedAt: baseTime.AddDate(0, 0, 1)}, // Same day as above
		{CreatedAt: baseTime.AddDate(0, 0, 2)}, // Another day
	}

	result := groupUsersByCreationDate(users)
	expected := map[string]int{
		"2023-01-01": 1,
		"2023-01-02": 2,
		"2023-01-03": 1,
	}

	assert.Equal(t, expected, result)
}

func TestGroupUsersByCreationDateEmpty(t *testing.T) {
	users := []models.User{}
	result := groupUsersByCreationDate(users)
	assert.Empty(t, result)
}