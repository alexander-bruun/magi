package metadata

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineMediaTypeByLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ja", "media"},
		{"jp", "media"},
		{"JA", "media"}, // test case insensitive
		{"ko", "manhwa"},
		{"zh", "manhua"},
		{"zh-cn", "manhua"},
		{"zh-hk", "manhua"},
		{"zh-tw", "manhua"},
		{"fr", "manfra"},
		{"en", "oel"},
		{"unknown", "media"}, // default case
		{"", "media"},        // empty string
		{"  ja  ", "media"},  // with spaces
	}

	for _, test := range tests {
		result := DetermineMediaTypeByLanguage(test.input)
		assert.Equal(t, test.expected, result, "DetermineMediaTypeByLanguage(%q)", test.input)
	}
}

func TestUpdateMedia(t *testing.T) {
	// Create a mock media updater
	mockMedia := &mockMediaUpdater{}

	// Create test metadata
	meta := &MediaMetadata{
		Title:            "Test Manga",
		Description:      "A test description",
		Year:             2023,
		OriginalLanguage: "ja",
		Status:           "ongoing",
		ContentRating:    "safe",
	}

	// Call UpdateMedia
	UpdateMedia(mockMedia, meta, "http://example.com/cover.jpg")

	// Verify all fields were set
	assert.Equal(t, "Test Manga", mockMedia.name)
	assert.Equal(t, "A test description", mockMedia.description)
	assert.Equal(t, 2023, mockMedia.year)
	assert.Equal(t, "ja", mockMedia.originalLanguage)
	assert.Equal(t, "ongoing", mockMedia.status)
	assert.Equal(t, "safe", mockMedia.contentRating)
	assert.Equal(t, "http://example.com/cover.jpg", mockMedia.coverArtURL)
}

func TestUpdateMedia_NilMetadata(t *testing.T) {
	// Create a mock media updater
	mockMedia := &mockMediaUpdater{}

	// Call UpdateMedia with nil metadata
	UpdateMedia(mockMedia, nil, "http://example.com/cover.jpg")

	// Verify no fields were set (function returns early)
	assert.Equal(t, "", mockMedia.name)
	assert.Equal(t, "", mockMedia.description)
	assert.Equal(t, 0, mockMedia.year)
	assert.Equal(t, "", mockMedia.originalLanguage)
	assert.Equal(t, "", mockMedia.status)
	assert.Equal(t, "", mockMedia.contentRating)
	assert.Equal(t, "", mockMedia.coverArtURL) // coverArtURL is not set when meta is nil
}

// mockMediaUpdater implements MediaUpdater for testing
type mockMediaUpdater struct {
	name             string
	description      string
	year             int
	originalLanguage string
	status           string
	contentRating    string
	coverArtURL      string
}

func (m *mockMediaUpdater) SetName(name string) {
	m.name = name
}

func (m *mockMediaUpdater) SetDescription(description string) {
	m.description = description
}

func (m *mockMediaUpdater) SetYear(year int) {
	m.year = year
}

func (m *mockMediaUpdater) SetOriginalLanguage(lang string) {
	m.originalLanguage = lang
}

func (m *mockMediaUpdater) SetStatus(status string) {
	m.status = status
}

func (m *mockMediaUpdater) SetContentRating(rating string) {
	m.contentRating = rating
}

func (m *mockMediaUpdater) SetCoverArtURL(url string) {
	m.coverArtURL = url
}

func (m *mockMediaUpdater) SetType(mediaType string) {
	// Not used in UpdateMedia, but required by interface
}

func TestRegisterProvider(t *testing.T) {
	// Clear registry for clean test
	originalRegistry := providerRegistry
	providerRegistry = make(map[string]func(string) Provider)
	defer func() { providerRegistry = originalRegistry }()

	// Register a provider
	RegisterProvider("test", func(apiToken string) Provider {
		return &mockProvider{name: "test"}
	})

	// Verify it was registered
	providers := ListProviders()
	assert.Contains(t, providers, "test")
}

func TestGetProvider(t *testing.T) {
	// Clear registry for clean test
	originalRegistry := providerRegistry
	providerRegistry = make(map[string]func(string) Provider)
	defer func() { providerRegistry = originalRegistry }()

	// Register a provider
	RegisterProvider("test", func(apiToken string) Provider {
		return &mockProvider{name: "test", token: apiToken}
	})

	// Get the provider
	provider, err := GetProvider("test", "api-token-123")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "test", provider.Name())

	// Test getting non-existent provider
	_, err = GetProvider("nonexistent", "")
	assert.Error(t, err)
	assert.Equal(t, ErrProviderNotFound, err)
}

func TestListProviders(t *testing.T) {
	// Clear registry for clean test
	originalRegistry := providerRegistry
	providerRegistry = make(map[string]func(string) Provider)
	defer func() { providerRegistry = originalRegistry }()

	// Register multiple providers
	RegisterProvider("provider1", func(apiToken string) Provider {
		return &mockProvider{name: "provider1"}
	})
	RegisterProvider("provider2", func(apiToken string) Provider {
		return &mockProvider{name: "provider2"}
	})

	// List providers
	providers := ListProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "provider1")
	assert.Contains(t, providers, "provider2")
}

func TestListProviders_Empty(t *testing.T) {
	// Clear registry for clean test
	originalRegistry := providerRegistry
	providerRegistry = make(map[string]func(string) Provider)
	defer func() { providerRegistry = originalRegistry }()

	// List providers when empty
	providers := ListProviders()
	assert.Len(t, providers, 0)
}

// mockProvider implements Provider for testing
type mockProvider struct {
	name  string
	token string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Search(title string) ([]SearchResult, error) {
	return nil, nil
}

func (m *mockProvider) GetMetadata(id string) (*MediaMetadata, error) {
	return nil, nil
}

func (m *mockProvider) FindBestMatch(title string) (*MediaMetadata, error) {
	return nil, nil
}

func (m *mockProvider) RequiresAuth() bool {
	return false
}

func (m *mockProvider) SetAuthToken(token string) {
	m.token = token
}

func (m *mockProvider) SetConfig(config ConfigProvider) {
	// Not implemented for mock
}

func (m *mockProvider) GetCoverImageURL(metadata *MediaMetadata) string {
	return ""
}

func TestGetProviderFromConfig(t *testing.T) {
	// Test with MangaDex (default)
	config := &mockConfigProvider{
		metadataProvider: "mangadex",
	}
	
	provider, err := GetProviderFromConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mangadex", provider.Name())

	// Test with MAL
	config = &mockConfigProvider{
		metadataProvider: "mal",
		malToken:         "test-token",
	}
	
	provider, err = GetProviderFromConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mal", provider.Name())

	// Test with invalid provider (should default to mangadex)
	config = &mockConfigProvider{
		metadataProvider: "invalid",
	}
	
	provider, err = GetProviderFromConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mangadex", provider.Name())
}

func TestGetProviderForLibrary(t *testing.T) {
	config := &mockConfigProvider{
		metadataProvider: "mangadex",
	}

	// Test with library-specific provider
	provider, err := GetProviderForLibrary(sql.NullString{String: "mal", Valid: true}, config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mal", provider.Name())

	// Test with null library provider (should fall back to global)
	provider, err = GetProviderForLibrary(sql.NullString{String: "", Valid: false}, config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mangadex", provider.Name())

	// Test with empty library provider (should fall back to global)
	provider, err = GetProviderForLibrary(sql.NullString{String: "", Valid: true}, config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "mangadex", provider.Name())
}

// mockConfigProvider implements ConfigProvider for testing
type mockConfigProvider struct {
	metadataProvider string
	malToken         string
	anilistToken     string
	contentRatingLimit int
}

func (m *mockConfigProvider) GetMetadataProvider() string {
	return m.metadataProvider
}

func (m *mockConfigProvider) GetMALApiToken() string {
	return m.malToken
}

func (m *mockConfigProvider) GetAniListApiToken() string {
	return m.anilistToken
}

func (m *mockConfigProvider) GetContentRatingLimit() int {
	return m.contentRatingLimit
}