package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVariablesFromForm(t *testing.T) {
	tests := []struct {
		name     string
		formData ScraperFormData
		expected map[string]string
	}{
		{
			name: "single variable",
			formData: ScraperFormData{
				VariableName:  []string{"key1"},
				VariableValue: []string{"value1"},
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name: "multiple variables",
			formData: ScraperFormData{
				VariableName:  []string{"key1", "key2", "key3"},
				VariableValue: []string{"value1", "value2", "value3"},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "empty name skipped",
			formData: ScraperFormData{
				VariableName:  []string{"", "key1"},
				VariableValue: []string{"value0", "value1"},
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name: "mismatched lengths",
			formData: ScraperFormData{
				VariableName:  []string{"key1", "key2"},
				VariableValue: []string{"value1"},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "",
			},
		},
		{
			name: "empty form",
			formData: ScraperFormData{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVariablesFromForm(tt.formData)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPackagesFromForm(t *testing.T) {
	tests := []struct {
		name     string
		formData ScraperFormData
		expected []string
	}{
		{
			name: "single package",
			formData: ScraperFormData{
				Package: []string{"pkg1"},
			},
			expected: []string{"pkg1"},
		},
		{
			name: "multiple packages",
			formData: ScraperFormData{
				Package: []string{"pkg1", "pkg2", "pkg3"},
			},
			expected: []string{"pkg1", "pkg2", "pkg3"},
		},
		{
			name: "empty package skipped",
			formData: ScraperFormData{
				Package: []string{"", "pkg1", ""},
			},
			expected: []string{"pkg1"},
		},
		{
			name: "empty form",
			formData: ScraperFormData{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPackagesFromForm(tt.formData)
			assert.Equal(t, tt.expected, result)
		})
	}
}