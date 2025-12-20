package utils

import (
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveNonASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ASCII only",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with non-ASCII",
			input:    "héllo wörld",
			expected: "hllo wrld",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only non-ASCII",
			input:    "ñáéíóú",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeNonASCII(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveContentBetween(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		open     string
		close    string
		expected string
	}{
		{
			name:     "simple parentheses",
			input:    "hello (world) test",
			open:     "(",
			close:    ")",
			expected: "hello  test",
		},
		{
			name:     "nested parentheses",
			input:    "a (b (c) d) e",
			open:     "(",
			close:    ")",
			expected: "a  d) e",
		},
		{
			name:     "no closing",
			input:    "hello (world",
			open:     "(",
			close:    ")",
			expected: "hello (world",
		},
		{
			name:     "no opening",
			input:    "hello world)",
			open:     "(",
			close:    ")",
			expected: "hello world)",
		},
		{
			name:     "empty string",
			input:    "",
			open:     "(",
			close:    ")",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeContentBetween(tt.input, tt.open, tt.close)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveParenthesesContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple",
			input:    "hello (world) test",
			expected: "hello  test",
		},
		{
			name:     "nested",
			input:    "a (b (c) d) e",
			expected: "a  d) e",
		},
		{
			name:     "no parentheses",
			input:    "hello world",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeParenthesesContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveBracketsContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple",
			input:    "hello [world] test",
			expected: "hello  test",
		},
		{
			name:     "nested",
			input:    "a [b [c] d] e",
			expected: "a  d] e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeBracketsContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveBracesContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple",
			input:    "hello {world} test",
			expected: "hello  test",
		},
		{
			name:     "nested",
			input:    "a {b {c} d} e",
			expected: "a  d} e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeBracesContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveTrailingSuffixes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove archived",
			input:    "title - archived",
			expected: "title",
		},
		{
			name:     "remove RAR",
			input:    "title RAR",
			expected: "title",
		},
		{
			name:     "remove ZIP",
			input:    "title ZIP",
			expected: "title",
		},
		{
			name:     "remove rar lowercase",
			input:    "title rar",
			expected: "title",
		},
		{
			name:     "remove zip lowercase",
			input:    "title zip",
			expected: "title",
		},
		{
			name:     "remove +Plus",
			input:    "title +Plus",
			expected: "title",
		},
		{
			name:     "no suffix",
			input:    "title",
			expected: "title",
		},
		{
			name:     "multiple suffixes",
			input:    "title RAR ZIP",
			expected: "title RAR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeTrailingSuffixes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleSpecialCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "contains manga",
			input:    "One Piece Manga",
			expected: "one piece ",
		},
		{
			name:     "contains MANGA uppercase",
			input:    "One Piece MANGA",
			expected: "one piece ",
		},
		{
			name:     "no manga",
			input:    "One Piece",
			expected: "One Piece",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleSpecialCases(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessComplexPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "volume range v1 - v2",
			input:    "title v1 - v2",
			expected: "title",
		},
		{
			name:     "chapter range c1 - c2",
			input:    "title c1 - c2",
			expected: "title",
		},
		{
			name:     "mixed range v1 - 2",
			input:    "title v1 - 2",
			expected: "title",
		},
		{
			name:     "number range 12-34",
			input:    "title 12-34",
			expected: "title",
		},
		{
			name:     "large number range 000-305",
			input:    "title 000-305",
			expected: "title",
		},
		{
			name:     "volume plus Vol. 1 + Vol. 2",
			input:    "title Vol. 1 + Vol. 2",
			expected: "title",
		},
		{
			name:     "season S1",
			input:    "title S1",
			expected: "title",
		},
		{
			name:     "volume range with text Volume1-2+ABC",
			input:    "title Volume1-2+ABC",
			expected: "title",
		},
		{
			name:     "no patterns",
			input:    "simple title",
			expected: "simple title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processComplexPatterns(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSluggify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "with numbers",
			input:    "Test 123",
			expected: "test-123",
		},
		{
			name:     "with special chars",
			input:    "Hello, World!",
			expected: "hello-world",
		},
		{
			name:     "with dots and commas",
			input:    "v1.0, test",
			expected: "v1-0-test",
		},
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello-world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special chars",
			input:    "!@#$%",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sluggify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCollapseHyphens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple hyphens",
			input:    "hello--world",
			expected: "hello-world",
		},
		{
			name:     "leading hyphens",
			input:    "--hello",
			expected: "hello",
		},
		{
			name:     "trailing hyphens",
			input:    "hello--",
			expected: "hello",
		},
		{
			name:     "no hyphens",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single hyphens",
			input:    "hello-world",
			expected: "hello-world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collapseHyphens(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedNum int
		expectError bool
	}{
		{
			name:        "simple number",
			input:       "chapter 123",
			expectedNum: 123,
			expectError: false,
		},
		{
			name:        "number at start",
			input:       "123 test",
			expectedNum: 123,
			expectError: false,
		},
		{
			name:        "number in middle",
			input:       "abc123def",
			expectedNum: 123,
			expectError: false,
		},
		{
			name:        "no number",
			input:       "no numbers here",
			expectedNum: 0,
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectedNum: 0,
			expectError: true,
		},
		{
			name:        "number with letters after",
			input:       "123abc",
			expectedNum: 123,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractNumber(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNum, result)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "identical strings",
			s1:       "hello",
			s2:       "hello",
			expected: 0,
		},
		{
			name:     "empty s1",
			s1:       "",
			s2:       "hello",
			expected: 5,
		},
		{
			name:     "empty s2",
			s1:       "hello",
			s2:       "",
			expected: 5,
		},
		{
			name:     "both empty",
			s1:       "",
			s2:       "",
			expected: 0,
		},
		{
			name:     "single substitution",
			s1:       "kitten",
			s2:       "kitten",
			expected: 0,
		},
		{
			name:     "kitten to sitting",
			s1:       "kitten",
			s2:       "sitting",
			expected: 3,
		},
		{
			name:     "saturday to sunday",
			s1:       "saturday",
			s2:       "sunday",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LevenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSimilarityRatio(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected float64
	}{
		{
			name:     "identical",
			s1:       "hello",
			s2:       "hello",
			expected: 1.0,
		},
		{
			name:     "completely different",
			s1:       "abc",
			s2:       "xyz",
			expected: 0.0,
		},
		{
			name:     "one character different",
			s1:       "hello",
			s2:       "hella",
			expected: 0.8,
		},
		{
			name:     "case insensitive",
			s1:       "Hello",
			s2:       "hello",
			expected: 1.0,
		},
		{
			name:     "both empty",
			s1:       "",
			s2:       "",
			expected: 1.0,
		},
		{
			name:     "one empty",
			s1:       "",
			s2:       "hello",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SimilarityRatio(tt.s1, tt.s2)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestExtractChapterName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "volume v01",
			input:    "title v01.cbz",
			expected: "Volume 1",
		},
		{
			name:     "volume vol.1",
			input:    "title vol.1.cbz",
			expected: "Volume 1",
		},
		{
			name:     "volume volume 1",
			input:    "title volume 1.cbz",
			expected: "Volume 1",
		},
		{
			name:     "chapter c01",
			input:    "title c01.cbz",
			expected: "Chapter 1",
		},
		{
			name:     "chapter ch.1",
			input:    "title ch.1.cbz",
			expected: "Chapter 1",
		},
		{
			name:     "chapter chapter 1",
			input:    "title chapter 1.cbz",
			expected: "title chapter 1",
		},
		{
			name:     "no pattern, cleaned name",
			input:    "My Awesome Manga (2023).cbz",
			expected: "my awesome",
		},
		{
			name:     "digits only",
			input:    "001.cbz",
			expected: "Chapter 1",
		},
		{
			name:     "complex filename",
			input:    "One Piece v001 (2023) [Digital].cbz",
			expected: "Volume 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractChapterName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected template.HTML
	}{
		{
			name:     "empty string",
			input:    "",
			expected: template.HTML(""),
		},
		{
			name:     "simple text",
			input:    "hello world",
			expected: template.HTML("<p>hello world</p>\n"),
		},
		{
			name:     "bold text",
			input:    "**bold**",
			expected: template.HTML("<p><strong>bold</strong></p>\n"),
		},
		{
			name:     "italic text",
			input:    "*italic*",
			expected: template.HTML("<p><em>italic</em></p>\n"),
		},
		{
			name:     "link",
			input:    "[link](http://example.com)",
			expected: template.HTML("<p><a href=\"http://example.com\">link</a></p>\n"),
		},
		{
			name:     "code",
			input:    "`code`",
			expected: template.HTML("<p><code>code</code></p>\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkdownToHTML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemovePatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove parentheses",
			input:    "Title (2023)",
			expected: "Title",
		},
		{
			name:     "remove brackets",
			input:    "Title [Digital]",
			expected: "Title",
		},
		{
			name:     "remove braces",
			input:    "Title {Special}",
			expected: "Title",
		},
		{
			name:     "complex patterns",
			input:    "Title v1 - v2 (2023) [Digital]",
			expected: "Title",
		},
		{
			name:     "trailing suffixes",
			input:    "Title RAR",
			expected: "Title",
		},
		{
			name:     "special case manga",
			input:    "One Piece Manga",
			expected: "one piece",
		},
		{
			name:     "move the to front",
			input:    "Adventure, The",
			expected: "The Adventure",
		},
		{
			name:     "remove non-ASCII",
			input:    "Título Español",
			expected: "Ttulo Espaol",
		},
		{
			name:     "cached result",
			input:    "Test String",
			expected: "Test String",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemovePatterns(tt.input)
			assert.Equal(t, tt.expected, result)
			// Test caching by calling again
			result2 := RemovePatterns(tt.input)
			assert.Equal(t, result, result2)
		})
	}
}

func TestSimilarityRatioEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		minRatio float64
	}{
		{
			name:     "very long strings identical",
			s1:       strings.Repeat("a", 1000),
			s2:       strings.Repeat("a", 1000),
			minRatio: 0.99,
		},
		{
			name:     "unicode characters",
			s1:       "café",
			s2:       "cafe",
			minRatio: 0.5, // Should be somewhat different
		},
		{
			name:     "special characters",
			s1:       "hello@world!",
			s2:       "hello world",
			minRatio: 0.5,
		},
		{
			name:     "numbers and letters",
			s1:       "test123",
			s2:       "test456",
			minRatio: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SimilarityRatio(tt.s1, tt.s2)
			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}

func TestExtractNumberEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedNum int
		expectError bool
	}{
		{
			name:        "leading zeros",
			input:       "chapter 001",
			expectedNum: 1,
			expectError: false,
		},
		{
			name:        "very large number",
			input:       "chapter 999999999",
			expectedNum: 999999999,
			expectError: false,
		},
		{
			name:        "zero",
			input:       "chapter 0",
			expectedNum: 0,
			expectError: false,
		},
		{
			name:        "multiple numbers, extracts first",
			input:       "v1c2p3",
			expectedNum: 1,
			expectError: false,
		},
		{
			name:        "number with underscores before",
			input:       "___123",
			expectedNum: 123,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractNumber(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNum, result)
			}
		})
	}
}

func TestLevenshteinDistanceEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "single character strings",
			s1:       "a",
			s2:       "b",
			expected: 1,
		},
		{
			name:     "repeated characters",
			s1:       "aaaa",
			s2:       "aaaa",
			expected: 0,
		},
		{
			name:     "all different",
			s1:       "abcd",
			s2:       "efgh",
			expected: 4,
		},
		{
			name:     "substring",
			s1:       "abc",
			s2:       "abcdef",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LevenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlugggify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple title",
			input:    "My Awesome Manga",
			expected: "my-awesome-manga",
		},
		{
			name:     "with special chars",
			input:    "Title, With. Special",
			expected: "title-with-special",
		},
		{
			name:     "multiple spaces",
			input:    "Too    Many    Spaces",
			expected: "too-many-spaces",
		},
		{
			name:     "uppercase",
			input:    "UPPERCASE MANGA",
			expected: "uppercase-manga",
		},
		{
			name:     "numbers",
			input:    "Manga 123 Series",
			expected: "manga-123-series",
		},
		{
			name:     "leading trailing hyphens",
			input:    "--Leading Trailing--",
			expected: "leading-trailing",
		},
		{
			name:     "mixed punctuation",
			input:    "Title! With? Punctuation.",
			expected: "title-with-punctuation",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sluggify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}