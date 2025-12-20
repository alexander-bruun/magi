package scheduler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexander-bruun/magi/metadata"
	"github.com/alexander-bruun/magi/models"
)

func TestIsSafeZipPath(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"safe/path/file.txt", true},
		{"another/safe/file.jpg", true},
		{"../unsafe/path", false},
		{"path/with/../traversal", false},
		{"/absolute/path", false},
		{"../absolute/with/traversal", false},
		{"", true}, // empty string is safe
		{"file.txt", true},
		{"dir/file.txt", true},
		{"../../../etc/passwd", false},
		{"C:\\Windows\\system32", true}, // Not absolute on Unix systems
		{"./safe/path", true},           // relative path with current dir
		{"safe/path/.", true},           // path ending with current dir
		{"safe/path/./file.txt", true},  // path with current dir in middle
		{"safe//path", true},            // path with double slashes
		{"safe/path//file.txt", true},   // path with double slashes
	}

	for _, test := range tests {
		result := isSafeZipPath(test.input)
		assert.Equal(t, test.expected, result, "isSafeZipPath(%q)", test.input)
	}
}

func TestContainsNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"no numbers", false},
		{"has 1 number", true},
		{"123", true},
		{"text with 456 numbers", true},
		{"", false},
		{"no1here", true},
		{"日本語", false},
		{"mix3d", true},
		{"end with 9", true},
	}

	for _, test := range tests {
		result := containsNumber(test.input)
		assert.Equal(t, test.expected, result, "containsNumber(%q)", test.input)
	}
}

func TestDetermineContentType(t *testing.T) {
	tests := []struct {
		path     string
		isDir    bool
		expected ContentType
	}{
		{"/some/dir", true, MediaDirectory},
		{"file.cbz", false, SingleMediaFile},
		{"file.cbr", false, SingleMediaFile},
		{"file.zip", false, SingleMediaFile},
		{"file.rar", false, SingleMediaFile},
		{"file.CBZ", false, SingleMediaFile}, // case insensitive
		{"file.txt", false, Skip},
		{"file.jpg", false, Skip},
		{"file.epub", false, Skip}, // epub is not considered SingleMediaFile
		{"", false, Skip},
	}

	for _, test := range tests {
		result := determineContentType(test.path, test.isDir)
		assert.Equal(t, test.expected, result, "determineContentType(%q, %v)", test.path, test.isDir)
	}
}

func TestContainsEPUBFiles(t *testing.T) {
	// Test with non-existent directory
	assert.False(t, ContainsEPUBFiles("/nonexistent"))

	// Test with a single epub file
	tempDir := t.TempDir()
	epubFile := tempDir + "/test.epub"
	err := os.WriteFile(epubFile, []byte("fake epub"), 0644)
	assert.NoError(t, err)
	assert.True(t, ContainsEPUBFiles(epubFile))

	// Test with a directory containing epub
	epubDir := t.TempDir()
	epubFile2 := epubDir + "/book.epub"
	err = os.WriteFile(epubFile2, []byte("fake epub"), 0644)
	assert.NoError(t, err)
	assert.True(t, ContainsEPUBFiles(epubDir))

	// Test with a directory not containing epub
	emptyDir := t.TempDir()
	assert.False(t, ContainsEPUBFiles(emptyDir))

	// Test with a regular file that's not epub
	txtFile := tempDir + "/test.txt"
	err = os.WriteFile(txtFile, []byte("text"), 0644)
	assert.NoError(t, err)
	assert.False(t, ContainsEPUBFiles(txtFile))
}

func TestCreateMediaFromMetadata(t *testing.T) {
	tests := []struct {
		name       string
		meta       *metadata.MediaMetadata
		mediaName  string
		slug       string
		librarySlug string
		path       string
		coverURL   string
		expected   models.Media
	}{
		{
			name: "with metadata",
			meta: &metadata.MediaMetadata{
				Description:      "Test description",
				Year:             2023,
				OriginalLanguage: "en",
				Type:             "manga",
				Status:           "ongoing",
				ContentRating:    "safe",
				Author:           "Test Author",
				Tags:             []string{"action", "adventure"},
			},
			mediaName:   "Test Manga",
			slug:        "test-manga",
			librarySlug: "test-library",
			path:        "/path/to/media",
			coverURL:    "http://example.com/cover.jpg",
			expected: models.Media{
				Name:             "Test Manga",
				Slug:             "test-manga",
				LibrarySlug:      "test-library",
				Path:             "/path/to/media",
				CoverArtURL:      "http://example.com/cover.jpg",
				Description:      "Test description",
				Year:             2023,
				OriginalLanguage: "en",
				Type:             "manga",
				Status:           "ongoing",
				ContentRating:    "safe",
				Author:           "Test Author",
				Tags:             []string{"action", "adventure"},
			},
		},
		{
			name:        "without metadata",
			meta:        nil,
			mediaName:   "Test Manga 2",
			slug:        "test-manga-2",
			librarySlug: "test-library",
			path:        "/path/to/media2",
			coverURL:    "http://example.com/cover2.jpg",
			expected: models.Media{
				Name:        "Test Manga 2",
				Slug:        "test-manga-2",
				LibrarySlug: "test-library",
				Path:        "/path/to/media2",
				CoverArtURL: "http://example.com/cover2.jpg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createMediaFromMetadata(tt.meta, tt.mediaName, tt.slug, tt.librarySlug, tt.path, tt.coverURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectWebtoonFromImages(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func()) // returns path and cleanup function
		expected  string
	}{
		{
			name: "non-existent path",
			setupFunc: func() (string, func()) {
				return "/non/existent/path", func() {}
			},
			expected: "",
		},
		{
			name: "empty directory",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				return tempDir, func() {}
			},
			expected: "",
		},
		{
			name: "directory with non-chapter files",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				// Create a file without numbers (not considered a chapter)
				err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("text"), 0644)
				assert.NoError(t, err)
				return tempDir, func() {}
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup := tt.setupFunc()
			defer cleanup()
			result := DetectWebtoonFromImages(path, "test-slug")
			assert.Equal(t, tt.expected, result)
		})
	}
}