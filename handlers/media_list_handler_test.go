package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"text with &", "text with &amp;"},
		{"text with <", "text with &lt;"},
		{"text with >", "text with &gt;"},
		{"text with \"", "text with &quot;"},
		{"complex <>&\"", "complex &lt;&gt;&amp;&quot;"},
		{"", ""},
		{"no special chars", "no special chars"},
	}

	for _, tt := range tests {
		result := templEscape(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}