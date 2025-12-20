package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMangaDexProvider_Name(t *testing.T) {
	provider := NewMangaDexProvider("")
	assert.Equal(t, "mangadex", provider.Name())
}

func TestMangaDexProvider_RequiresAuth(t *testing.T) {
	provider := NewMangaDexProvider("")
	assert.False(t, provider.RequiresAuth())
}

func TestMangaDexProvider_SetAuthToken(t *testing.T) {
	provider := NewMangaDexProvider("")
	provider.SetAuthToken("test-token")
	// Since SetAuthToken is a setter, we can verify the method exists and doesn't panic
}

func TestMangaDexProvider_SetConfig(t *testing.T) {
	provider := NewMangaDexProvider("")
	config := &mockConfigProvider{}
	provider.SetConfig(config)
	// Since SetConfig is a setter, we can verify the method exists and doesn't panic
}

func TestMangaDexProvider_GetCoverImageURL(t *testing.T) {
	provider := NewMangaDexProvider("")

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

func TestMangaDexProvider_Search(t *testing.T) {
	// Create a test server that returns mock MangaDex response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock MangaDex Search response
		response := `{
			"result": "ok",
			"data": [
				{
					"id": "manga-123",
					"type": "manga",
					"attributes": {
						"title": {"en": "Test Manga"},
						"altTitles": [{"ja": "テスト漫画"}],
						"description": {"en": "A test manga"},
						"year": 2020,
						"originalLanguage": "ja",
						"status": "ongoing",
						"contentRating": "safe",
						"tags": [
							{
								"id": "tag-1",
								"attributes": {"name": {"en": "Action"}}
							}
						]
					},
					"relationships": [
						{
							"id": "cover-456",
							"type": "cover_art",
							"attributes": {
								"fileName": "cover.jpg"
							}
						}
					]
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &MangaDexProvider{
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
	assert.Contains(t, results[0].Tags, "Action")
}

func TestMangaDexProvider_GetMetadata(t *testing.T) {
	// Create a test server that returns mock MangaDex response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock MangaDex GetMetadata response
		response := `{
			"result": "ok",
			"data": {
				"id": "manga-123",
				"type": "manga",
				"attributes": {
					"title": {"en": "Test Manga"},
					"altTitles": [{"ja": "テスト漫画"}],
					"description": {"en": "A detailed test manga"},
					"year": 2020,
					"originalLanguage": "ja",
					"status": "completed",
					"contentRating": "safe",
					"tags": [
						{
							"id": "tag-1",
							"attributes": {"name": {"en": "Action"}}
						},
						{
							"id": "tag-2",
							"attributes": {"name": {"en": "Adventure"}}
						}
					]
				},
				"relationships": [
					{
						"id": "cover-456",
						"type": "cover_art",
						"attributes": {
							"fileName": "cover.jpg"
						}
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &MangaDexProvider{
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
	assert.Equal(t, "completed", metadata.Status)
	assert.Equal(t, "manga", metadata.Type)
	assert.Contains(t, metadata.Tags, "Action")
	assert.Contains(t, metadata.Tags, "Adventure")
}

func TestMangaDexProvider_FindBestMatch(t *testing.T) {
	// Create a test server that returns mock MangaDex responses
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var response string

		if callCount == 1 {
			// First call - Search
			response = `{
				"result": "ok",
				"data": [
					{
						"id": "manga-456",
						"type": "manga",
						"attributes": {
							"title": {"en": "Test Manga"},
							"altTitles": [],
							"description": {"en": "A test manga"},
							"year": 2020,
							"originalLanguage": "ja",
							"status": "ongoing",
							"contentRating": "safe",
							"tags": [
								{
									"id": "tag-1",
									"attributes": {"name": {"en": "Action"}}
								}
							]
						},
						"relationships": [
							{
								"id": "cover-456",
								"type": "cover_art",
								"attributes": {
									"fileName": "cover.jpg"
								}
							}
						]
					}
				]
			}`
		} else {
			// Second call - GetMetadata
			response = `{
				"result": "ok",
				"data": {
					"id": "manga-456",
					"type": "manga",
					"attributes": {
						"title": {"en": "Test Manga"},
						"altTitles": [],
						"description": {"en": "A detailed test manga"},
						"year": 2020,
						"originalLanguage": "ja",
						"status": "completed",
						"contentRating": "safe",
						"tags": [
							{
								"id": "tag-1",
								"attributes": {"name": {"en": "Action"}}
							}
						]
					},
					"relationships": [
						{
							"id": "cover-456",
							"type": "cover_art",
							"attributes": {
								"fileName": "cover.jpg"
							}
						}
					]
				}
			}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with test client and baseURL
	provider := &MangaDexProvider{
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

func TestMangaDexProvider_Search_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"result": "ok",
			"data": []
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	provider := &MangaDexProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestMangaDexProvider_Search_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"result": "error",
			"data": []
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	provider := &MangaDexProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	results, err := provider.Search("test")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestMangaDexProvider_GetMetadata_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := &MangaDexProvider{
		client:  server.Client(),
		baseURL: server.URL,
		config:  &mockConfigProvider{},
	}

	metadata, err := provider.GetMetadata("manga-123")
	assert.Error(t, err)
	assert.Nil(t, metadata)
}

func TestExtractBestTitle(t *testing.T) {
	tests := []struct {
		name      string
		titles    map[string]string
		altTitles []map[string]string
		expected  string
	}{
		{
			name:     "empty titles and alt titles",
			titles:   map[string]string{},
			altTitles: []map[string]string{},
			expected: "",
		},
		{
			name: "title in en",
			titles: map[string]string{
				"en": "English Title",
			},
			altTitles: []map[string]string{},
			expected:  "English Title",
		},
		{
			name: "title in ja, alt in en",
			titles: map[string]string{
				"ja": "Japanese Title",
			},
			altTitles: []map[string]string{
				{"en": "English Alt Title"},
			},
			expected: "English Alt Title",
		},
		{
			name: "multiple alt titles",
			titles: map[string]string{
				"ja": "Japanese Title",
			},
			altTitles: []map[string]string{
				{"ko": "Korean Title"},
				{"en": "English Title"},
			},
			expected: "English Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBestTitle(tt.titles, tt.altTitles)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name        string
		descriptions map[string]string
		expected    string
	}{
		{
			name:        "empty descriptions",
			descriptions: map[string]string{},
			expected:    "",
		},
		{
			name: "description in en",
			descriptions: map[string]string{
				"en": "English description",
			},
			expected: "English description",
		},
		{
			name: "multiple descriptions",
			descriptions: map[string]string{
				"ja": "Japanese description",
				"en": "English description",
			},
			expected: "English description",
		},
		{
			name: "no en description",
			descriptions: map[string]string{
				"ja": "Japanese description",
			},
			expected: "Japanese description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDescription(tt.descriptions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineMediaType(t *testing.T) {
	tests := []struct {
		name     string
		detail   *mangadexMediaDetail
		expected string
	}{
		{
			name: "empty tags",
			detail: &mangadexMediaDetail{
				Attributes: mangadexAttributes{
					Tags:      []mangadexTag{},
					OriginalLanguage: "ja",
				},
			},
			expected: "manga",
		},
		{
			name: "manhwa tag",
			detail: &mangadexMediaDetail{
				Attributes: mangadexAttributes{
					Tags: []mangadexTag{
						{Attributes: struct {
							Name map[string]string `json:"name"`
						}{Name: map[string]string{"en": "manhwa"}}},
					},
					OriginalLanguage: "ko",
				},
			},
			expected: "manhwa",
		},
		{
			name: "manhua tag",
			detail: &mangadexMediaDetail{
				Attributes: mangadexAttributes{
					Tags: []mangadexTag{
						{Attributes: struct {
							Name map[string]string `json:"name"`
						}{Name: map[string]string{"en": "manhua"}}},
					},
					OriginalLanguage: "zh",
				},
			},
			expected: "manhua",
		},
		{
			name: "webtoon tag",
			detail: &mangadexMediaDetail{
				Attributes: mangadexAttributes{
					Tags: []mangadexTag{
						{Attributes: struct {
							Name map[string]string `json:"name"`
						}{Name: map[string]string{"en": "webtoon"}}},
					},
					OriginalLanguage: "ko",
				},
			},
			expected: "webtoon",
		},
		{
			name: "multiple tags",
			detail: &mangadexMediaDetail{
				Attributes: mangadexAttributes{
					Tags: []mangadexTag{
						{Attributes: struct {
							Name map[string]string `json:"name"`
						}{Name: map[string]string{"en": "some tag"}}},
						{Attributes: struct {
							Name map[string]string `json:"name"`
						}{Name: map[string]string{"en": "manhwa"}}},
					},
					OriginalLanguage: "ko",
				},
			},
			expected: "manhwa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineMediaType(tt.detail)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractCoverURL(t *testing.T) {
	tests := []struct {
		name         string
		mangaID      string
		relationships []mangadexRelationship
		expected     string
	}{
		{
			name:    "cover_art relationship with fileName",
			mangaID: "manga-123",
			relationships: []mangadexRelationship{
				{
					ID:   "cover-456",
					Type: "cover_art",
					Attributes: map[string]interface{}{
						"fileName": "cover.jpg",
					},
				},
			},
			expected: "https://uploads.mangadex.org/covers/manga-123/cover.jpg",
		},
		{
			name:    "multiple relationships, cover_art first",
			mangaID: "manga-789",
			relationships: []mangadexRelationship{
				{
					ID:   "cover-101",
					Type: "cover_art",
					Attributes: map[string]interface{}{
						"fileName": "first-cover.png",
					},
				},
				{
					ID:   "author-202",
					Type: "author",
					Attributes: map[string]interface{}{
						"name": "Author Name",
					},
				},
			},
			expected: "https://uploads.mangadex.org/covers/manga-789/first-cover.png",
		},
		{
			name:    "cover_art relationship without fileName",
			mangaID: "manga-999",
			relationships: []mangadexRelationship{
				{
					ID:   "cover-303",
					Type: "cover_art",
					Attributes: map[string]interface{}{
						"volume": "1",
					},
				},
			},
			expected: "",
		},
		{
			name:    "no cover_art relationship",
			mangaID: "manga-404",
			relationships: []mangadexRelationship{
				{
					ID:   "author-505",
					Type: "author",
					Attributes: map[string]interface{}{
						"name": "Author Name",
					},
				},
				{
					ID:   "artist-606",
					Type: "artist",
					Attributes: map[string]interface{}{
						"name": "Artist Name",
					},
				},
			},
			expected: "",
		},
		{
			name:           "empty relationships",
			mangaID:        "manga-000",
			relationships:  []mangadexRelationship{},
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCoverURL(tt.mangaID, tt.relationships)
			assert.Equal(t, tt.expected, result)
		})
	}
}