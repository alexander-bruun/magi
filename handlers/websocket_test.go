package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetScraperKey(t *testing.T) {
	tests := []struct {
		scriptID int64
		expected string
	}{
		{1, "scraper_1"},
		{123, "scraper_123"},
		{0, "scraper_0"},
		{-1, "scraper_-1"},
	}

	for _, tt := range tests {
		result := getScraperKey(tt.scriptID)
		assert.Equal(t, tt.expected, result)
	}
}

func TestGetIndexerKey(t *testing.T) {
	tests := []struct {
		librarySlug string
		expected    string
	}{
		{"manga-lib", "indexer_manga-lib"},
		{"test", "indexer_test"},
		{"", "indexer_"},
		{"special_chars!@#", "indexer_special_chars!@#"},
	}

	for _, tt := range tests {
		result := getIndexerKey(tt.librarySlug)
		assert.Equal(t, tt.expected, result)
	}
}