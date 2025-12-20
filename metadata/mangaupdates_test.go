package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMangaUpdatesProvider_Name(t *testing.T) {
	provider := NewMangaUpdatesProvider("")
	assert.Equal(t, "mangaupdates", provider.Name())
}

func TestMangaUpdatesProvider_RequiresAuth(t *testing.T) {
	provider := NewMangaUpdatesProvider("")
	assert.False(t, provider.RequiresAuth())
}

func TestMangaUpdatesProvider_SetAuthToken(t *testing.T) {
	provider := NewMangaUpdatesProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can verify the method exists and doesn't panic
}

func TestMangaUpdatesProvider_SetConfig(t *testing.T) {
	provider := NewMangaUpdatesProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestMangaUpdatesProvider_GetCoverImageURL(t *testing.T) {
	provider := NewMangaUpdatesProvider("")

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

func TestMangaUpdatesProvider_Search(t *testing.T) {
	// Create a test server that returns mock MangaUpdates response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock MangaUpdates Search response
		response := `{
			"results": [
				{
					"hit": 100,
					"record": {
						"series_id": 123,
						"title": "Test Manga",
						"image": {
							"url": {
								"original": "https://example.com/cover.jpg"
							}
						},
						"type": "Manga",
						"year": "2020",
						"description": "A test manga",
						"genres": [
							{"genre": "Action"},
							{"genre": "Adventure"}
						],
						"categories": [
							{"category": "Shounen"}
						]
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
	provider := &MangaUpdatesProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("test")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "123", results[0].ID)
	assert.Equal(t, "Test Manga", results[0].Title)
	assert.Equal(t, "A test manga", results[0].Description)
	assert.Equal(t, 2020, results[0].Year)
	assert.Contains(t, results[0].Tags, "Action")
}

func TestMangaUpdatesProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock MangaUpdates response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock MangaUpdates GetMetadata response
		response := `{
			"series_id": 123,
			"title": "Test Manga",
			"image": {
				"url": {
					"original": "https://example.com/cover.jpg"
				}
			},
			"type": "Manga",
			"year": "2020",
			"description": "A detailed test manga",
			"status": "Complete",
			"genres": [
				{"genre": "Action"},
				{"genre": "Adventure"}
			],
			"categories": [
				{"category": "Shounen"}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &MangaUpdatesProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.GetMetadata("123")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "Test Manga", metadata.Title)
	assert.Equal(t, "A detailed test manga", metadata.Description)
	assert.Equal(t, 2020, metadata.Year)
	assert.Contains(t, metadata.Tags, "Action")
}

func TestMangaUpdatesProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock MangaUpdates responses
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var response string

		if callCount == 1 {
			// First call - Search
			response = `{
				"results": [
					{
						"hit": 100,
						"record": {
							"series_id": 456,
							"title": "Test Manga",
							"image": {
								"url": {
									"original": "https://example.com/cover.jpg"
								}
							},
							"type": "Manga",
							"year": "2020",
							"description": "A test manga",
							"genres": [
								{"genre": "Action"}
							]
						}
					}
				]
			}`
		} else {
			// Second call - GetMetadata
			response = `{
				"series_id": 456,
				"title": "Test Manga",
				"image": {
					"url": {
						"original": "https://example.com/cover.jpg"
					}
				},
				"type": "Manga",
				"year": "2020",
				"description": "A detailed test manga",
				"status": "Complete",
				"genres": [
					{"genre": "Action"}
				]
			}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &MangaUpdatesProvider{
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

func TestMangaUpdatesProvider_Search_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"results": []
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	provider := &MangaUpdatesProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestMangaUpdatesProvider_GetMetadata_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := &MangaUpdatesProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.GetMetadata("123")
	assert.Error(t, err)
	assert.Nil(t, metadata)
}

func TestExtractMangaUpdatesTitle(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "",
		},
		{
			name: "with title",
			series: mangaupdatesSeriesDetail{
				Title: "Test Title",
			},
			expected: "Test Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesTitle(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesDescription(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "",
		},
		{
			name: "with description",
			series: mangaupdatesSeriesDetail{
				Description: "Test description",
			},
			expected: "Test description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesDescription(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesCoverURL(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "",
		},
		{
			name: "with image URL",
			series: mangaupdatesSeriesDetail{
				Image: struct {
					URL      struct {
						Original string `json:"original"`
						Thumb    string `json:"thumb"`
					} `json:"url"`
					Height   int `json:"height"`
					Width    int `json:"width"`
				}{
					URL: struct {
						Original string `json:"original"`
						Thumb    string `json:"thumb"`
					}{Original: "https://example.com/cover.jpg"},
				},
			},
			expected: "https://example.com/cover.jpg",
		},
		{
			name: "nil image URL",
			series: mangaupdatesSeriesDetail{
				Image: struct {
					URL      struct {
						Original string `json:"original"`
						Thumb    string `json:"thumb"`
					} `json:"url"`
					Height   int `json:"height"`
					Width    int `json:"width"`
				}{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesCoverURL(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesYear(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected int
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: 0,
		},
		{
			name: "with year",
			series: mangaupdatesSeriesDetail{
				Year: "2023",
			},
			expected: 2023,
		},
		{
			name: "invalid year",
			series: mangaupdatesSeriesDetail{
				Year: "invalid",
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesYear(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesLanguage(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "ja",
		},
		{
			name: "with Japanese language",
			series: mangaupdatesSeriesDetail{
				Publications: []struct {
					PublicationID int    `json:"publication_id"`
					Name          string `json:"name"`
					PublisherName string `json:"publisher_name"`
				}{
					{Name: "JP"},
				},
			},
			expected: "ja",
		},
		{
			name: "with Korean language",
			series: mangaupdatesSeriesDetail{
				Type: "Korean Manhwa",
			},
			expected: "ko",
		},
		{
			name: "with Chinese language",
			series: mangaupdatesSeriesDetail{
				Type: "Chinese Manhua",
			},
			expected: "zh",
		},
		{
			name: "unknown language",
			series: mangaupdatesSeriesDetail{
				Publications: []struct {
					PublicationID int    `json:"publication_id"`
					Name          string `json:"name"`
					PublisherName string `json:"publisher_name"`
				}{
					{Name: "XX"},
				},
			},
			expected: "ja",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesLanguage(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesStatus(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "ongoing",
		},
		{
			name: "completed status",
			series: mangaupdatesSeriesDetail{
				Status: "Complete",
			},
			expected: "completed",
		},
		{
			name: "ongoing status",
			series: mangaupdatesSeriesDetail{
				Status: "Ongoing",
			},
			expected: "ongoing",
		},
		{
			name: "hiatus status",
			series: mangaupdatesSeriesDetail{
				Status: "hiatus",
			},
			expected: "hiatus",
		},
		{
			name: "cancelled status",
			series: mangaupdatesSeriesDetail{
				Status: "Cancelled",
			},
			expected: "cancelled",
		},
		{
			name: "unknown status",
			series: mangaupdatesSeriesDetail{
				Status: "Unknown",
			},
			expected: "ongoing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesStatus(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesContentRating(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "safe",
		},
		{
			name: "with genres containing adult",
			series: mangaupdatesSeriesDetail{
				Genres: []struct {
					Genre string `json:"genre"`
				}{
					{Genre: "Action"},
					{Genre: "Adult"},
				},
			},
			expected: "pornographic",
		},
		{
			name: "with genres containing hentai",
			series: mangaupdatesSeriesDetail{
				Genres: []struct {
					Genre string `json:"genre"`
				}{
					{Genre: "Action"},
					{Genre: "Hentai"},
				},
			},
			expected: "pornographic",
		},
		{
			name: "with genres containing mature",
			series: mangaupdatesSeriesDetail{
				Genres: []struct {
					Genre string `json:"genre"`
				}{
					{Genre: "Action"},
					{Genre: "Mature"},
				},
			},
			expected: "erotica",
		},
		{
			name: "safe genres",
			series: mangaupdatesSeriesDetail{
				Genres: []struct {
					Genre string `json:"genre"`
				}{
					{Genre: "Action"},
					{Genre: "Comedy"},
				},
			},
			expected: "safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesContentRating(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMangaUpdatesType(t *testing.T) {
	tests := []struct {
		name     string
		series   mangaupdatesSeriesDetail
		expected string
	}{
		{
			name: "empty series",
			series: mangaupdatesSeriesDetail{},
			expected: "manga",
		},
		{
			name: "manhwa type",
			series: mangaupdatesSeriesDetail{
				Type: "Manhwa",
			},
			expected: "manhwa",
		},
		{
			name: "manhua type",
			series: mangaupdatesSeriesDetail{
				Type: "Manhua",
			},
			expected: "manhua",
		},
		{
			name: "unknown type",
			series: mangaupdatesSeriesDetail{
				Type: "Unknown",
			},
			expected: "manga",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMangaUpdatesType(tt.series)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateMangaUpdatesMatchScore(t *testing.T) {
	tests := []struct {
		name          string
		result        SearchResult
		originalTitle string
		expected      float64
	}{
		{
			name: "basic score without adjustments",
			result: SearchResult{
				SimilarityScore: 0.8,
				Year:            0,
				Tags:            []string{"action", "adventure"},
			},
			originalTitle: "Test Manga",
			expected:      0.8,
		},
		{
			name: "score with year bonus",
			result: SearchResult{
				SimilarityScore: 0.7,
				Year:            2020,
				Tags:            []string{"action"},
			},
			originalTitle: "Test Manga",
			expected:      0.8, // 0.7 + 0.1
		},
		{
			name: "score with doujinshi penalty",
			result: SearchResult{
				SimilarityScore: 0.9,
				Year:            0,
				Tags:            []string{"doujinshi", "romance"},
			},
			originalTitle: "Test Manga",
			expected:      0.6, // 0.9 - 0.3
		},
		{
			name: "score with both year bonus and doujinshi penalty",
			result: SearchResult{
				SimilarityScore: 0.8,
				Year:            2015,
				Tags:            []string{"doujinshi", "comedy"},
			},
			originalTitle: "Test Manga",
			expected:      0.6, // 0.8 - 0.3 + 0.1 = 0.6
		},
		{
			name: "score with case insensitive doujinshi penalty",
			result: SearchResult{
				SimilarityScore: 0.75,
				Year:            0,
				Tags:            []string{"Doujinshi"},
			},
			originalTitle: "Test Manga",
			expected:      0.45, // 0.75 - 0.3
		},
		{
			name: "score with multiple tags including doujinshi",
			result: SearchResult{
				SimilarityScore: 0.85,
				Year:            2022,
				Tags:            []string{"action", "doujinshi", "fantasy"},
			},
			originalTitle: "Test Manga",
			expected:      0.65, // 0.85 - 0.3 + 0.1 = 0.65
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMangaUpdatesMatchScore(tt.result, tt.originalTitle)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}