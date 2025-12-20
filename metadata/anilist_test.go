package metadata

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAniListProvider_Name(t *testing.T) {
	provider := NewAniListProvider("")
	assert.Equal(t, "anilist", provider.Name())
}

func TestAniListProvider_RequiresAuth(t *testing.T) {
	provider := NewAniListProvider("")
	assert.False(t, provider.RequiresAuth())
}

func TestAniListProvider_SetAuthToken(t *testing.T) {
	provider := NewAniListProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can't easily test the internal state
	// but we can verify the method exists and doesn't panic
}

func TestAniListProvider_SetConfig(t *testing.T) {
	provider := NewAniListProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestAniListProvider_GetCoverImageURL(t *testing.T) {
	provider := NewAniListProvider("")

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

func TestAniListProvider_Search(t *testing.T) {
	// Create a test server that returns mock AniList response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock GraphQL response
		response := `{
			"data": {
				"Page": {
					"media": [
						{
							"id": 123,
							"title": {
								"romaji": "Test Manga",
								"english": "Test Manga English",
								"native": "テスト漫画"
							},
							"description": "A test manga",
							"startDate": {"year": 2020},
							"coverImage": {"large": "https://example.com/cover.jpg"},
							"genres": ["Action", "Adventure"],
							"tags": [{"name": "Shonen"}]
						}
					]
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &AniListProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	results, err := provider.Search("test")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "123", results[0].ID)
	assert.Equal(t, "Test Manga English", results[0].Title)
}

func TestAniListProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock AniList response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock GraphQL response
		response := `{
			"data": {
				"Media": {
					"id": 123,
					"title": {
						"romaji": "Test Manga",
						"english": "Test Manga English",
						"native": "テスト漫画"
					},
					"description": "A test manga description",
					"startDate": {"year": 2020},
					"endDate": {"year": 2022},
					"status": "FINISHED",
					"countryOfOrigin": "JP",
					"isAdult": false,
					"format": "MANGA",
					"genres": ["Action", "Adventure"],
					"tags": [{"name": "Shonen"}],
					"synonyms": ["Test Alias"],
					"coverImage": {"large": "https://example.com/cover.jpg", "extraLarge": "https://example.com/cover-xl.jpg"}
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &AniListProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	metadata, err := provider.GetMetadata("123")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga English", metadata.Title)
	assert.Equal(t, "A test manga description", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Equal(t, "completed", metadata.Status)
	assert.Equal(t, "manga", metadata.Type)
	assert.Contains(t, metadata.Tags, "Action")
}

func TestAniListProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock AniList responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response string

		// Check the request body to determine which query this is
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if strings.Contains(bodyStr, "search") || strings.Contains(bodyStr, "Page") {
			// Search query response
			response = `{
				"data": {
					"Page": {
						"media": [
							{
								"id": 123,
								"title": {
									"romaji": "Test Manga",
									"english": "Test Manga English"
								},
								"description": "A test manga description",
								"startDate": {"year": 2020},
								"coverImage": {"large": "https://example.com/cover.jpg"}
							}
						]
					}
				}
			}`
		} else if strings.Contains(bodyStr, "Media(id:") {
			// GetMetadata query response
			response = `{
				"data": {
					"Media": {
						"id": 123,
						"title": {
							"romaji": "Test Manga",
							"english": "Test Manga English",
							"native": "テスト漫画"
						},
						"description": "A test manga description",
						"startDate": {"year": 2020},
						"endDate": {"year": 2022},
						"status": "FINISHED",
						"countryOfOrigin": "JP",
						"isAdult": false,
						"format": "MANGA",
						"genres": ["Action", "Adventure"],
						"tags": [{"name": "Shonen"}],
						"synonyms": ["Test Alias"],
						"coverImage": {"large": "https://example.com/cover.jpg", "extraLarge": "https://example.com/cover-xl.jpg"}
					}
				}
			}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &AniListProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	metadata, err := provider.FindBestMatch("Test Manga")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga English", metadata.Title)
	assert.Equal(t, "A test manga description", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Equal(t, "completed", metadata.Status)
	assert.Equal(t, "manga", metadata.Type)
	assert.Contains(t, metadata.Tags, "Action")
}

func TestExtractAniListTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name:     "english title available",
			input:    map[string]interface{}{"english": "English Title", "romaji": "Romaji Title", "native": "Native Title"},
			expected: "English Title",
		},
		{
			name:     "english empty, romaji available",
			input:    map[string]interface{}{"english": "", "romaji": "Romaji Title", "native": "Native Title"},
			expected: "Romaji Title",
		},
		{
			name:     "only native available",
			input:    map[string]interface{}{"english": "", "romaji": "", "native": "Native Title"},
			expected: "Native Title",
		},
		{
			name:     "no titles available",
			input:    map[string]interface{}{"english": "", "romaji": "", "native": ""},
			expected: "",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAniListTitle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAniListStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"finished", "FINISHED", "completed"},
		{"releasing", "RELEASING", "ongoing"},
		{"not yet released", "NOT_YET_RELEASED", "upcoming"},
		{"cancelled", "CANCELLED", "cancelled"},
		{"hiatus", "HIATUS", "hiatus"},
		{"unknown", "UNKNOWN", "ongoing"},
		{"lowercase", "finished", "completed"},
		{"empty", "", "ongoing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAniListStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAniListContentRating(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{"adult content", true, "pornographic"},
		{"non-adult content", false, "safe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAniListContentRating(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAniListFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"manga", "MANGA", "manga"},
		{"novel", "NOVEL", "novel"},
		{"light novel", "LIGHT_NOVEL", "novel"},
		{"one shot", "ONE_SHOT", "oneshot"},
		{"unknown", "UNKNOWN", "manga"},
		{"empty", "", "manga"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAniListFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertCountryToLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"japan", "JP", "ja"},
		{"south korea", "KR", "ko"},
		{"china", "CN", "zh"},
		{"taiwan", "TW", "zh"},
		{"hong kong", "HK", "zh"},
		{"united states", "US", "en"},
		{"united kingdom", "GB", "en"},
		{"unknown", "XX", "ja"},
		{"empty", "", "ja"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCountryToLanguage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple html",
			input:    "<p>Hello <b>world</b></p>",
			expected: "Hello world",
		},
		{
			name:     "html with entities",
			input:    "Hello &amp; welcome",
			expected: "Hello &amp; welcome",
		},
		{
			name:     "no html",
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "br tags",
			input:    "Line 1<br>Line 2<br/>Line 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}