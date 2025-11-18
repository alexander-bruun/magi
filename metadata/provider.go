package metadata

import (
	"errors"
)

// MangaMetadata represents the standardized metadata structure returned by all providers
type MangaMetadata struct {
	Title             string
	Description       string
	Year              int
	OriginalLanguage  string
	Status            string // ongoing, completed, hiatus, cancelled
	ContentRating     string // safe, suggestive, erotica, pornographic
	CoverArtURL       string
	Tags              []string
	Type              string // manga, manhwa, manhua, webtoon, etc.
	AlternativeTitles []string
	Author            string
	ExternalID        string // Provider-specific ID
}

// SearchResult represents a single manga search result
type SearchResult struct {
	ID               string
	Title            string
	Description      string
	CoverArtURL      string
	Year             int
	SimilarityScore  float64
}

// Provider is the interface that all metadata providers must implement
type Provider interface {
	// Name returns the provider name (e.g., "mangadex", "mal", "anilist", "jikan")
	Name() string

	// Search searches for manga by title and returns a list of results
	Search(title string) ([]SearchResult, error)

	// GetMetadata fetches detailed metadata for a specific manga by provider ID
	GetMetadata(id string) (*MangaMetadata, error)

	// FindBestMatch searches for manga and returns the best matching result
	FindBestMatch(title string) (*MangaMetadata, error)

	// RequiresAuth returns true if this provider requires an API token
	RequiresAuth() bool

	// SetAuthToken sets the authentication token for the provider
	SetAuthToken(token string)

	// GetCoverImageURL returns the actual downloadable URL for cover art
	// This allows each provider to handle URL construction differently
	GetCoverImageURL(metadata *MangaMetadata) string
}

var (
	ErrProviderNotFound   = errors.New("metadata provider not found")
	ErrNoResults          = errors.New("no search results found")
	ErrAuthRequired       = errors.New("authentication required for this provider")
	ErrInvalidCredentials = errors.New("invalid authentication credentials")
)

// Registry holds all registered metadata providers
var providerRegistry = make(map[string]func(string) Provider)

// RegisterProvider registers a new metadata provider constructor
func RegisterProvider(name string, constructor func(apiToken string) Provider) {
	providerRegistry[name] = constructor
}

// GetProvider returns a provider instance by name with the given API token
func GetProvider(name string, apiToken string) (Provider, error) {
	constructor, exists := providerRegistry[name]
	if !exists {
		return nil, ErrProviderNotFound
	}
	return constructor(apiToken), nil
}

// ListProviders returns a list of all registered provider names
func ListProviders() []string {
	names := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		names = append(names, name)
	}
	return names
}
