package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKitsuProvider_Name(t *testing.T) {
	provider := NewKitsuProvider("")
	assert.Equal(t, "kitsu", provider.Name())
}

func TestKitsuProvider_RequiresAuth(t *testing.T) {
	provider := NewKitsuProvider("")
	assert.False(t, provider.RequiresAuth())
}

func TestKitsuProvider_SetAuthToken(t *testing.T) {
	provider := NewKitsuProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can verify the method exists and doesn't panic
}

func TestKitsuProvider_SetConfig(t *testing.T) {
	provider := NewKitsuProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestKitsuProvider_GetCoverImageURL(t *testing.T) {
	provider := NewKitsuProvider("")

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

func TestKitsuProvider_Search(t *testing.T) {
	// Create a test server that returns mock Kitsu response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Kitsu Search response
		response := `{
			"data": [
				{
					"id": "manga-123",
					"type": "manga",
					"attributes": {
						"titles": {
							"en": "Test Manga",
							"en_jp": "テスト漫画"
						},
						"slug": "test-manga",
						"synopsis": "A test manga",
						"description": "",
						"startDate": "2020-01-01",
						"endDate": "2022-12-31",
						"posterImage": {
							"original": "https://example.com/cover.jpg",
							"small": "https://example.com/cover-small.jpg"
						},
						"genreIdMap": {
							"action": "Action",
							"adventure": "Adventure"
						}
					}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &KitsuProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("test")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "manga-123", results[0].ID)
	assert.Equal(t, "Test Manga", results[0].Title)
	assert.Equal(t, "A test manga", results[0].Description)
	assert.Equal(t, 2020, results[0].Year)
}

func TestKitsuProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock Kitsu response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Kitsu GetMetadata response
		response := `{
			"data": {
				"id": "manga-123",
				"type": "manga",
				"attributes": {
					"titles": {
						"en": "Test Manga",
						"en_jp": "テスト漫画"
					},
					"slug": "test-manga",
					"synopsis": "A detailed test manga",
					"description": "",
					"startDate": "2020-01-01",
					"endDate": "2022-12-31",
					"posterImage": {
						"original": "https://example.com/cover.jpg",
						"small": "https://example.com/cover-small.jpg"
					},
					"genreIdMap": {
						"action": "Action",
						"adventure": "Adventure"
					}
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &KitsuProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.GetMetadata("manga-123")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga", metadata.Title)
	assert.Equal(t, "A detailed test manga", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Equal(t, "manga", metadata.Type)
}

func TestKitsuProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock Kitsu responses
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var response string

		if callCount == 1 {
			// First call - Search
			response = `{
				"data": [
					{
						"id": "manga-456",
						"type": "manga",
						"attributes": {
							"titles": {
								"en": "Test Manga",
								"en_jp": "テスト漫画"
							},
							"slug": "test-manga",
							"synopsis": "A test manga",
							"description": "",
							"startDate": "2020-01-01",
							"endDate": "2022-12-31",
							"posterImage": {
								"original": "https://example.com/cover.jpg",
								"small": "https://example.com/cover-small.jpg"
							},
							"genreIdMap": {
								"action": "Action"
							}
						}
					}
				]
			}`
		} else {
			// Second call - GetMetadata
			response = `{
				"data": {
					"id": "manga-456",
					"type": "manga",
					"attributes": {
						"titles": {
							"en": "Test Manga",
							"en_jp": "テスト漫画"
						},
						"slug": "test-manga",
						"synopsis": "A detailed test manga",
						"description": "",
						"startDate": "2020-01-01",
						"endDate": "2022-12-31",
						"posterImage": {
							"original": "https://example.com/cover.jpg",
							"small": "https://example.com/cover-small.jpg"
						},
						"genreIdMap": {
							"action": "Action"
						}
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
	provider := &KitsuProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.FindBestMatch("test manga")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga", metadata.Title)
	assert.Equal(t, "A detailed test manga", metadata.Description)
}

func TestKitsuProvider_Search_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": []
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	provider := &KitsuProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestKitsuProvider_GetMetadata_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := &KitsuProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.GetMetadata("manga-123")
	assert.Error(t, err)
	assert.Nil(t, metadata)
}

func TestExtractKitsuTitle(t *testing.T) {
	tests := []struct {
		name     string
		titles   map[string]string
		expected string
	}{
		{
			name:     "empty map",
			titles:   map[string]string{},
			expected: "",
		},
		{
			name: "with en title",
			titles: map[string]string{
				"en": "English Title",
			},
			expected: "English Title",
		},
		{
			name: "with en and ja titles",
			titles: map[string]string{
				"en": "English Title",
				"ja": "Japanese Title",
			},
			expected: "English Title",
		},
		{
			name: "without en title",
			titles: map[string]string{
				"ja": "Japanese Title",
			},
			expected: "Japanese Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKitsuTitle(tt.titles)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapKitsuAgeRating(t *testing.T) {
	tests := []struct {
		name           string
		ageRating      *string
		ageRatingGuide *string
		expected       string
	}{
		{
			name:     "nil age rating",
			expected: "safe",
		},
		{
			name:      "G rating",
			ageRating: stringPtr("G"),
			expected:  "safe",
		},
		{
			name:      "PG rating",
			ageRating: stringPtr("PG"),
			expected:  "suggestive",
		},
		{
			name:      "R rating",
			ageRating: stringPtr("R"),
			expected:  "suggestive",
		},
		{
			name:           "R with guide",
			ageRating:      stringPtr("R"),
			ageRatingGuide: stringPtr("Mild Nudity"),
			expected:       "suggestive",
		},
		{
			name:      "unknown rating",
			ageRating: stringPtr("X"),
			expected:  "safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapKitsuAgeRating(tt.ageRating, tt.ageRatingGuide)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractKitsuCoverURL(t *testing.T) {
	tests := []struct {
		name     string
		image    kitsuImage
		expected string
	}{
		{
			name: "large image available",
			image: kitsuImage{
				Large: "large.jpg",
				Medium: "medium.jpg",
				Small: "small.jpg",
				Original: "original.jpg",
				Tiny: "tiny.jpg",
			},
			expected: "large.jpg",
		},
		{
			name: "medium image available",
			image: kitsuImage{
				Medium: "medium.jpg",
				Small: "small.jpg",
				Original: "original.jpg",
				Tiny: "tiny.jpg",
			},
			expected: "medium.jpg",
		},
		{
			name: "small image available",
			image: kitsuImage{
				Small: "small.jpg",
				Original: "original.jpg",
				Tiny: "tiny.jpg",
			},
			expected: "small.jpg",
		},
		{
			name: "original image available",
			image: kitsuImage{
				Original: "original.jpg",
				Tiny: "tiny.jpg",
			},
			expected: "original.jpg",
		},
		{
			name: "only tiny image available",
			image: kitsuImage{
				Tiny: "tiny.jpg",
			},
			expected: "tiny.jpg",
		},
		{
			name: "empty image",
			image: kitsuImage{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKitsuCoverURL(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}