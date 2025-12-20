package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAccountListConfig(t *testing.T) {
	tests := []struct {
		listType        string
		expectedTitle   string
		expectedBread   string
		expectedEmpty   string
		expectedPath    string
	}{
		{
			"favorites",
			"Favorites",
			"Favorites",
			"You have no favorites yet.",
			"/account/favorites",
		},
		{
			"upvoted",
			"Upvoted",
			"Upvoted",
			"You have not upvoted any media yet.",
			"/account/upvoted",
		},
		{
			"downvoted",
			"Downvoted",
			"Downvoted",
			"You have not downvoted any media yet.",
			"/account/downvoted",
		},
		{
			"reading",
			"Reading",
			"Reading",
			"You are not reading any media right now.",
			"/account/reading",
		},
		{
			"unknown",
			"Unknown",
			"Unknown",
			"No items found.",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.listType, func(t *testing.T) {
			title, bread, empty, path := GetAccountListConfig(tt.listType)
			assert.Equal(t, tt.expectedTitle, title)
			assert.Equal(t, tt.expectedBread, bread)
			assert.Equal(t, tt.expectedEmpty, empty)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}