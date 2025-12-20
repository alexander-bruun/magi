package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJikanProvider_Name(t *testing.T) {
	provider := NewJikanProvider("")
	assert.Equal(t, "jikan", provider.Name())
}

func TestJikanProvider_RequiresAuth(t *testing.T) {
	provider := NewJikanProvider("")
	assert.False(t, provider.RequiresAuth())
}

func TestJikanProvider_SetAuthToken(t *testing.T) {
	provider := NewJikanProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can't easily test the internal state
	// but we can verify the method exists and doesn't panic
}

func TestJikanProvider_SetConfig(t *testing.T) {
	provider := NewJikanProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestJikanProvider_GetCoverImageURL(t *testing.T) {
	provider := NewJikanProvider("")

	tests := []struct {
		name     string
		metadata *MediaMetadata
		expected string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: "",
		},
		{
			name: "empty cover URL",
			metadata: &MediaMetadata{
				CoverArtURL: "",
			},
			expected: "",
		},
		{
			name: "valid cover URL",
			metadata: &MediaMetadata{
				CoverArtURL: "https://example.com/cover.jpg",
			},
			expected: "https://example.com/cover.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetCoverImageURL(tt.metadata)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJikanProvider_Search(t *testing.T) {
	// Create a test server that returns mock Jikan response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Jikan search response
		response := `{
			"data": [
				{
					"mal_id": 123,
					"title": "Test Manga",
					"title_english": "Test Manga English",
					"title_japanese": "テスト漫画",
					"title_synonyms": ["Test Alias"],
					"synopsis": "A test manga description",
					"type": "Manga",
					"status": "Finished",
					"published": {"from": "2020-01-01"},
					"images": {
						"jpg": {
							"image_url": "https://example.com/image.jpg",
							"small_image_url": "https://example.com/small.jpg",
							"large_image_url": "https://example.com/large.jpg"
						}
					},
					"genres": [{"name": "Action"}, {"name": "Adventure"}],
					"themes": [{"name": "Shonen"}],
					"demographics": [{"name": "Shonen"}]
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &JikanProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	results, err := provider.Search("Test Manga")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "123", results[0].ID)
	assert.Equal(t, "Test Manga", results[0].Title)
	assert.Equal(t, "A test manga description", results[0].Description)
	assert.Equal(t, 2020, results[0].Year)
	assert.Contains(t, results[0].Tags, "Action")
}

func TestJikanProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock Jikan response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Jikan get metadata response
		response := `{
			"data": {
				"mal_id": 123,
				"title": "Test Manga",
				"title_english": "Test Manga English",
				"title_japanese": "テスト漫画",
				"title_synonyms": ["Test Alias"],
				"synopsis": "A test manga description",
				"type": "Manga",
				"status": "Finished",
				"published": {"from": "2020-01-01", "to": "2022-01-01"},
				"images": {
					"jpg": {
						"image_url": "https://example.com/image.jpg",
						"small_image_url": "https://example.com/small.jpg",
						"large_image_url": "https://example.com/large.jpg"
					}
				},
				"genres": [{"name": "Action"}, {"name": "Adventure"}],
				"themes": [{"name": "Shonen"}],
				"demographics": [{"name": "Shonen"}]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &JikanProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	metadata, err := provider.GetMetadata("123")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga", metadata.Title)
	assert.Equal(t, "A test manga description", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Equal(t, "completed", metadata.Status)
	assert.Equal(t, "manga", metadata.Type)
	assert.Contains(t, metadata.Tags, "Action")
}

func TestJikanProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock Jikan responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response string

		// Check the request URL to determine which endpoint this is
		if r.URL.Path == "/series" && r.URL.RawQuery != "" {
			// Search endpoint
			response = `{
				"data": [
					{
						"mal_id": 123,
						"title": "Test Manga",
						"title_english": "Test Manga English",
						"synopsis": "A test manga description",
						"published": {"from": "2020-01-01"},
						"images": {
							"jpg": {
								"large_image_url": "https://example.com/large.jpg"
							}
						}
					}
				]
			}`
		} else if r.URL.Path == "/series/123/full" {
			// GetMetadata endpoint
			response = `{
				"data": {
					"mal_id": 123,
					"title": "Test Manga",
					"title_english": "Test Manga English",
					"title_japanese": "テスト漫画",
					"title_synonyms": ["Test Alias"],
					"synopsis": "A test manga description",
					"type": "Manga",
					"status": "Finished",
					"published": {"from": "2020-01-01", "to": "2022-01-01"},
					"images": {
						"jpg": {
							"image_url": "https://example.com/image.jpg",
							"large_image_url": "https://example.com/large.jpg"
						}
					},
					"genres": [{"name": "Action"}, {"name": "Adventure"}],
					"themes": [{"name": "Shonen"}],
					"demographics": [{"name": "Shonen"}]
				}
			}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &JikanProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	metadata, err := provider.FindBestMatch("Test Manga")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga", metadata.Title)
	assert.Equal(t, "A test manga description", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Equal(t, "completed", metadata.Status)
	assert.Equal(t, "manga", metadata.Type)
	assert.Contains(t, metadata.Tags, "Action")
}

func TestConvertJikanStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"finished", "finished", "completed"},
		{"publishing", "publishing", "ongoing"},
		{"on hiatus", "on hiatus", "hiatus"},
		{"discontinued", "discontinued", "cancelled"},
		{"unknown", "unknown", "ongoing"},
		{"empty", "", "ongoing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertJikanStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertJikanRating(t *testing.T) {
	tests := []struct {
		name     string
		input    []struct {
			MalID int    `json:"mal_id"`
			Type  string `json:"type"`
			Name  string `json:"name"`
		}
		expected string
	}{
		{
			name: "safe rating",
			input: []struct {
				MalID int    `json:"mal_id"`
				Type  string `json:"type"`
				Name  string `json:"name"`
			}{
				{MalID: 1, Type: "genre", Name: "Shounen"},
				{MalID: 2, Type: "genre", Name: "Comedy"},
			},
			expected: "safe",
		},
		{
			name: "suggestive rating",
			input: []struct {
				MalID int    `json:"mal_id"`
				Type  string `json:"type"`
				Name  string `json:"name"`
			}{
				{MalID: 3, Type: "genre", Name: "Ecchi"},
			},
			expected: "suggestive",
		},
		{
			name: "pornographic rating",
			input: []struct {
				MalID int    `json:"mal_id"`
				Type  string `json:"type"`
				Name  string `json:"name"`
			}{
				{MalID: 4, Type: "genre", Name: "Hentai"},
			},
			expected: "pornographic",
		},
		{
			name: "empty demographics",
			input: []struct {
				MalID int    `json:"mal_id"`
				Type  string `json:"type"`
				Name  string `json:"name"`
			}{},
			expected: "safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertJikanRating(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertJikanType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"manga", "manga", "manga"},
		{"novel", "novel", "novel"},
		{"one shot", "one-shot", "oneshot"},
		{"manhwa", "manhwa", "manhwa"},
		{"manhua", "manhua", "manhua"},
		{"doujinshi", "doujinshi", "doujinshi"},
		{"light novel", "light_novel", "novel"},
		{"unknown", "unknown", "manga"},
		{"empty", "", "manga"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertJikanType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}