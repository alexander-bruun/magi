package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMALProvider_Name(t *testing.T) {
	provider := NewMALProvider("")
	assert.Equal(t, "mal", provider.Name())
}

func TestMALProvider_RequiresAuth(t *testing.T) {
	provider := NewMALProvider("")
	assert.True(t, provider.RequiresAuth())
}

func TestMALProvider_SetAuthToken(t *testing.T) {
	provider := NewMALProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can't easily test the internal state
	// but we can verify the method exists and doesn't panic
}

func TestMALProvider_SetConfig(t *testing.T) {
	provider := NewMALProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestMALProvider_GetCoverImageURL(t *testing.T) {
	provider := NewMALProvider("")

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

func TestMALProvider_Search(t *testing.T) {
	// Create a test server that returns mock MAL response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for API token header
		if r.Header.Get("X-MAL-CLIENT-ID") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Mock MAL search response
		response := `{
			"data": [
				{
					"node": {
						"id": 123,
						"title": "Test Manga",
						"synopsis": "A test manga description",
						"start_date": "2020-01-01",
						"media_type": "manga",
						"main_picture": {
							"medium": "https://example.com/medium.jpg",
							"large": "https://example.com/large.jpg"
						},
						"alternative_titles": {
							"en": "Test Manga English",
							"ja": "テスト漫画",
							"synonyms": ["Test Alias"]
						},
						"genres": [{"id": 1, "name": "Action"}, {"id": 2, "name": "Adventure"}]
					}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client, baseURL, and token
	provider := &MALProvider{
		apiToken: "test-token",
		client:   server.Client(),
		baseURL:  server.URL,
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

func TestMALProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock MAL response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for API token header
		if r.Header.Get("X-MAL-CLIENT-ID") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Mock MAL get metadata response
		response := `{
			"id": 123,
			"title": "Test Manga",
			"synopsis": "A test manga description",
			"start_date": "2020-01-01",
			"end_date": "2022-01-01",
			"status": "finished",
			"media_type": "manga",
			"nsfw": "white",
			"num_chapters": 50,
			"main_picture": {
				"medium": "https://example.com/medium.jpg",
				"large": "https://example.com/large.jpg"
			},
			"alternative_titles": {
				"en": "Test Manga English",
				"ja": "テスト漫画",
				"synonyms": ["Test Alias"]
			},
			"genres": [{"id": 1, "name": "Action"}, {"id": 2, "name": "Adventure"}]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client, baseURL, and token
	provider := &MALProvider{
		apiToken: "test-token",
		client:   server.Client(),
		baseURL:  server.URL,
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

func TestMALProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock MAL responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for API token header
		if r.Header.Get("X-MAL-CLIENT-ID") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var response string

		// Check the request URL to determine which endpoint this is
		if r.URL.RawQuery != "" && r.URL.Path == "/series" {
			// Search endpoint
			response = `{
				"data": [
					{
						"node": {
							"id": 123,
							"title": "Test Manga",
							"synopsis": "A test manga description",
							"start_date": "2020-01-01",
							"media_type": "manga",
							"main_picture": {
								"large": "https://example.com/large.jpg"
							}
						}
					}
				]
			}`
		} else if r.URL.Path == "/series/123" {
			// GetMetadata endpoint
			response = `{
				"id": 123,
				"title": "Test Manga",
				"synopsis": "A test manga description",
				"start_date": "2020-01-01",
				"end_date": "2022-01-01",
				"status": "finished",
				"media_type": "manga",
				"nsfw": "white",
				"num_chapters": 50,
				"main_picture": {
					"medium": "https://example.com/medium.jpg",
					"large": "https://example.com/large.jpg"
				},
				"alternative_titles": {
					"en": "Test Manga English",
					"ja": "テスト漫画",
					"synonyms": ["Test Alias"]
				},
				"genres": [{"id": 1, "name": "Action"}, {"id": 2, "name": "Adventure"}]
			}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client, baseURL, and token
	provider := &MALProvider{
		apiToken: "test-token",
		client:   server.Client(),
		baseURL:  server.URL,
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

func TestConvertMALStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"finished", "completed"},
		{"currently_publishing", "ongoing"},
		{"on_hiatus", "hiatus"},
		{"discontinued", "cancelled"},
		{"unknown", "ongoing"},
		{"", "ongoing"},
		{"FINISHED", "completed"}, // test case insensitive
	}

	for _, test := range tests {
		result := convertMALStatus(test.input)
		assert.Equal(t, test.expected, result, "convertMALStatus(%q)", test.input)
	}
}

func TestConvertMALContentRating(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"white", "safe"},
		{"gray", "suggestive"},
		{"black", "pornographic"},
		{"unknown", "safe"},
		{"", "safe"},
		{"WHITE", "safe"}, // test case insensitive
	}

	for _, test := range tests {
		result := convertMALContentRating(test.input)
		assert.Equal(t, test.expected, result, "convertMALContentRating(%q)", test.input)
	}
}

func TestConvertMALMediaType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"manga", "manga"},
		{"manhwa", "manhwa"},
		{"manhua", "manhua"},
		{"one_shot", "oneshot"},
		{"doujinshi", "doujinshi"},
		{"light_novel", "novel"},
		{"novel", "novel"},
		{"unknown", "manga"},
		{"", "manga"},
		{"MANGA", "manga"}, // test case insensitive
	}

	for _, test := range tests {
		result := convertMALMediaType(test.input)
		assert.Equal(t, test.expected, result, "convertMALMediaType(%q)", test.input)
	}
}