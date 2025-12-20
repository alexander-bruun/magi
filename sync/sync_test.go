package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterProvider(t *testing.T) {
	// Clear providers for clean test
	originalProviders := providers
	providers = make(map[string]func(string) SyncProvider)
	defer func() { providers = originalProviders }()

	// Register a provider
	RegisterProvider("test", func(token string) SyncProvider {
		return &mockSyncProvider{name: "test", token: token}
	})

	// Get the provider to verify registration
	provider := GetProvider("test", "test-token")
	assert.NotNil(t, provider)
	assert.Equal(t, "test", provider.Name())
}

func TestGetProvider(t *testing.T) {
	// Clear providers for clean test
	originalProviders := providers
	providers = make(map[string]func(string) SyncProvider)
	defer func() { providers = originalProviders }()

	// Register a provider
	RegisterProvider("test", func(token string) SyncProvider {
		return &mockSyncProvider{name: "test", token: token}
	})

	// Get existing provider
	provider := GetProvider("test", "test-token")
	assert.NotNil(t, provider)
	assert.Equal(t, "test", provider.Name())

	// Get non-existent provider
	provider = GetProvider("nonexistent", "token")
	assert.Nil(t, provider)
}

func TestParseChapterNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"chapter-1", 1, false},
		{"chapter-001", 1, false},
		{"ch-5", 5, false},
		{"vol-1-ch-10", 10, false},
		{"chapter-123", 123, false},
		{"invalid", 0, true},
		{"no-dash", 0, true},
		{"chapter-", 0, true},
		{"", 0, true},
		{"chapter-0", 0, false},
		{"chapter-9999", 9999, false},
		{"special-chapter-5", 5, false},
		{"chapter-extra-7", 7, false},
		{"chapter-1-2", 2, false}, // takes the last number
		{"chapter-abc", 0, true},  // non-numeric
	}

	for _, test := range tests {
		result, err := parseChapterNumber(test.input)
		if test.hasError {
			assert.Error(t, err, "parseChapterNumber(%q) should error", test.input)
		} else {
			assert.NoError(t, err, "parseChapterNumber(%q) should not error", test.input)
			assert.Equal(t, test.expected, result, "parseChapterNumber(%q)", test.input)
		}
	}
}

func TestParseVolumeNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"Vol. 1 Ch. 5", 1, false},
		{"Volume 2 Chapter 10", 2, false},
		{"V 3 Ch 15", 3, false},
		{"Vol. 001 Ch. 005", 1, false},
		{"No volume info", 0, true},
		{"Chapter 5 only", 0, true},
		{"", 0, true},
		{"Vol. X Ch. 1", 0, true}, // Non-numeric volume
		{"Vol. 5 Chapter 2", 5, false}, // matches "Vol. 5"
		{"VOLUME 10 CH. 3", 0, true}, // doesn't match case-sensitive patterns
		{"v 7 ch 1", 0, true}, // doesn't match case-sensitive patterns
		{"Vol. 0 Ch. 1", 0, false}, // zero volume
		{"Vol. 123 Ch. 456", 123, false}, // large numbers
		{"Multiple Vol. 2 and Vol. 3", 2, false}, // takes first match
		{"Vol. ABC Ch. 1", 0, true}, // non-numeric after pattern
	}

	for _, test := range tests {
		result, err := parseVolumeNumber(test.input)
		if test.hasError {
			assert.Error(t, err, "parseVolumeNumber(%q) should error", test.input)
		} else {
			assert.NoError(t, err, "parseVolumeNumber(%q) should not error", test.input)
			assert.Equal(t, test.expected, result, "parseVolumeNumber(%q)", test.input)
		}
	}
}

func TestNewAniListProvider(t *testing.T) {
	provider := NewAniListProvider("test-token")
	assert.NotNil(t, provider)
	assert.Equal(t, "anilist", provider.Name())
	assert.True(t, provider.RequiresAuth())

	// Test SetAuthToken
	provider.SetAuthToken("new-token")
	// Since accessToken is private, we can't directly test it, but the method should not panic
}

func TestNewMALProvider(t *testing.T) {
	provider := NewMALProvider("test-token")
	assert.NotNil(t, provider)
	assert.Equal(t, "mal", provider.Name())
	assert.True(t, provider.RequiresAuth())

	// Test SetAuthToken
	provider.SetAuthToken("new-token")
	// Since apiToken is private, we can't directly test it, but the method should not panic
}

func TestSyncReadingProgressForUser_NoAccounts(t *testing.T) {
	// This test would require mocking getUserExternalAccounts
	// For now, skip this test as it requires more complex mocking setup
	t.Skip("Skipping integration test - requires database and HTTP mocking")
}

func TestSyncReadingProgressForUser_WithAccounts(t *testing.T) {
	// This test would require mocking getUserExternalAccounts and HTTP calls
	// For now, skip this test as it requires more complex mocking setup
	t.Skip("Skipping integration test - requires database and HTTP mocking")
}

// mockSyncProvider implements SyncProvider for testing
type mockSyncProvider struct {
	name  string
	token string
}

func (m *mockSyncProvider) Name() string {
	return m.name
}

func (m *mockSyncProvider) RequiresAuth() bool {
	return true
}

func (m *mockSyncProvider) SetAuthToken(token string) {
	m.token = token
}

func (m *mockSyncProvider) SyncReadingProgress(userName string, mediaSlug string, chapterSlug string) error {
	return nil
}