package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "h1 title",
			html:     `<html><body><h1>Chapter 1</h1><p>Content</p></body></html>`,
			expected: "Chapter 1",
		},
		{
			name:     "title tag",
			html:     `<html><head><title>Test Title</title></head><body></body></html>`,
			expected: "Test Title",
		},
		{
			name:     "h1 with attributes",
			html:     `<h1 class="title" id="main">Main Title</h1>`,
			expected: "Main Title",
		},
		{
			name:     "case insensitive",
			html:     `<H1>Uppercase</H1>`,
			expected: "Uppercase",
		},
		{
			name:     "no title",
			html:     `<html><body><p>No title here</p></body></html>`,
			expected: "Untitled",
		},
		{
			name:     "empty html",
			html:     "",
			expected: "Untitled",
		},
		{
			name:     "h1 takes precedence",
			html:     `<html><head><title>Page Title</title></head><body><h1>H1 Title</h1></body></html>`,
			expected: "H1 Title",
		},
		{
			name:     "whitespace trimmed",
			html:     `<h1>  Spaced Title  </h1>`,
			expected: "Spaced Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTitle(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanHTMLContentWithValidity(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "remove doctype and html tags",
			html:     `<!DOCTYPE html><html><head></head><body>content</body></html>`,
			expected: `content`,
		},
		{
			name:     "remove script tags",
			html:     `<p>before</p><script>alert('test');</script><p>after</p>`,
			expected: `<p>before</p><p>after</p>`,
		},
		{
			name:     "remove style tags",
			html:     `<p>before</p><style>body { color: red; }</style><p>after</p>`,
			expected: `<p>before</p><p>after</p>`,
		},
		{
			name:     "remove link tags",
			html:     `<link rel="stylesheet" href="style.css"><p>content</p>`,
			expected: `<p>content</p>`,
		},
		{
			name:     "remove meta tags",
			html:     `<meta charset="utf-8"><p>content</p>`,
			expected: `<p>content</p>`,
		},
		{
			name:     "complex html cleaning",
			html:     `<!DOCTYPE html><html><head><meta charset="utf-8"><link rel="stylesheet" href="style.css"></head><body><script>console.log('test');</script><p>Hello world</p></body></html>`,
			expected: `<p>Hello world</p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTMLContentWithValidity(tt.html, "manga", "chapter", "path", "opf", 5)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetChapters(t *testing.T) {
	// Test with non-existent file
	chapters, err := GetChapters("/non/existent/file.epub")
	assert.Error(t, err)
	assert.Nil(t, chapters)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestGetOPFDir(t *testing.T) {
	// Test with non-existent file
	opfDir, err := GetOPFDir("/non/existent/file.epub")
	assert.Error(t, err)
	assert.Empty(t, opfDir)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestGetTitlesFromNCX(t *testing.T) {
	// Test with non-existent file
	titles, err := GetTitlesFromNCX("/non/existent/file.epub")
	assert.Error(t, err)
	assert.Nil(t, titles)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestGetTOC(t *testing.T) {
	// Test with non-existent file
	toc := GetTOC("/non/existent/file.epub")
	assert.Contains(t, toc, "Error opening EPUB")
	assert.Contains(t, toc, "no such file or directory")
}

func TestGetBookContent(t *testing.T) {
	// Test with non-existent file
	content := GetBookContent("/non/existent/file.epub", "manga", "chapter")
	assert.Contains(t, content, "Error opening EPUB")
	assert.Contains(t, content, "no such file or directory")
}