package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteMedia(t *testing.T) {
	tests := []struct {
		name        string
		mediaSlug   string
		expectError bool
	}{
		{
			name:        "delete existing media",
			mediaSlug:   "test-media",
			expectError: false, // Function handles non-existent media gracefully (returns nil)
		},
		{
			name:        "delete non-existent media",
			mediaSlug:   "nonexistent",
			expectError: false, // Function handles non-existent media gracefully (returns nil)
		},
		{
			name:        "empty media slug",
			mediaSlug:   "",
			expectError: false, // Function handles empty slug gracefully (returns nil)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DeleteMedia(tt.mediaSlug)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}