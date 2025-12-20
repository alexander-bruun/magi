package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineMangaTypeByLanguage(t *testing.T) {
	tests := []struct {
		lang     string
		expected string
	}{
		{"ja", "manga"},
		{"jp", "manga"},
		{"JA", "manga"}, // case insensitive
		{"  ja  ", "manga"}, // trimmed
		{"ko", "manhwa"},
		{"zh", "manhua"},
		{"cn", "manhua"},
		{"zh-cn", "manhua"},
		{"zh-hk", "manhua"},
		{"zh-tw", "manhua"},
		{"fr", "manfra"},
		{"en", "oel"},
		{"unknown", "manga"}, // default
		{"", "manga"},        // default
		{"  ", "manga"},      // default after trim
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			result := DetermineMangaTypeByLanguage(tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}