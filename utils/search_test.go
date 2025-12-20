package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBigramSearch(t *testing.T) {
	items := []string{
		"One Piece",
		"Naruto",
		"Attack on Titan",
		"Death Note",
		"My Hero Academia",
		"Demon Slayer",
		"Fullmetal Alchemist",
	}

	tests := []struct {
		name     string
		keyword  string
		expected []string
	}{
		{
			name:     "exact match",
			keyword:  "One Piece",
			expected: []string{"One Piece"},
		},
		{
			name:     "partial match",
			keyword:  "Piece",
			expected: []string{"One Piece"},
		},
		{
			name:     "no match",
			keyword:  "Dragon Ball",
			expected: nil,
		},
		{
			name:     "case insensitive",
			keyword:  "one piece",
			expected: []string{"One Piece"},
		},
		{
			name:     "empty keyword",
			keyword:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BigramSearch(tt.keyword, items)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompareStrings(t *testing.T) {
	tests := []struct {
		name     string
		str1     string
		str2     string
		expected float64
	}{
		{
			name:     "identical strings",
			str1:     "hello",
			str2:     "hello",
			expected: 1.0,
		},
		{
			name:     "similar strings",
			str1:     "hello",
			str2:     "helo",
			expected: 0.857,
		},
		{
			name:     "different strings",
			str1:     "abc",
			str2:     "xyz",
			expected: 0.0,
		},
		{
			name:     "one is substring",
			str1:     "hello world",
			str2:     "world",
			expected: 0.771,
		},
		{
			name:     "short strings",
			str1:     "a",
			str2:     "b",
			expected: 0.0,
		},
		{
			name:     "empty strings",
			str1:     "",
			str2:     "",
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareStrings(tt.str1, tt.str2)
			if tt.expected == 0.0 || tt.expected == 1.0 {
				assert.Equal(t, tt.expected, result)
			} else {
				assert.InDelta(t, tt.expected, result, 0.1)
			}
		})
	}
}

func TestBuildBigramCounts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]int
	}{
		{
			name:  "simple string",
			input: "hello",
			expected: map[string]int{
				"he": 1,
				"el": 1,
				"ll": 1,
				"lo": 1,
			},
		},
		{
			name:  "repeated bigrams",
			input: "aaa",
			expected: map[string]int{
				"aa": 2,
			},
		},
		{
			name:     "short string",
			input:    "a",
			expected: map[string]int{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildBigramCounts(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountCommonBigrams(t *testing.T) {
	bigramCounts := map[string]int{
		"he": 1,
		"el": 1,
		"ll": 1,
		"lo": 1,
	}

	tests := []struct {
		name                 string
		str                  string
		bigramCounts         map[string]int
		expectedCommon       int
		expectedTotal        int
	}{
		{
			name:           "some common",
			str:            "hello",
			bigramCounts:   bigramCounts,
			expectedCommon: 4,
			expectedTotal:  4,
		},
		{
			name: "no common",
			str:  "xyz",
			bigramCounts: map[string]int{
				"he": 1,
			},
			expectedCommon: 0,
			expectedTotal:  2,
		},
		{
			name:           "short string",
			str:            "a",
			bigramCounts:   bigramCounts,
			expectedCommon: 0,
			expectedTotal:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common, total := countCommonBigrams(tt.str, tt.bigramCounts)
			assert.Equal(t, tt.expectedCommon, common)
			assert.Equal(t, tt.expectedTotal, total)
		})
	}
}