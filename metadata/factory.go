package metadata

import (
	"database/sql"
	"fmt"
)

// ConfigProvider is an interface for accessing app configuration
// This allows the metadata package to get provider config without importing models
type ConfigProvider interface {
	GetMetadataProvider() string
	GetMALClientID() string
	GetMALClientSecret() string
	GetAniListApiToken() string
	GetContentRatingLimit() int
}

// LibraryConfigProvider extends ConfigProvider with library-specific settings
type LibraryConfigProvider interface {
	ConfigProvider
	GetLibraryMetadataProvider() string
}

// GetProviderFromConfig returns the configured metadata provider instance
func GetProviderFromConfig(config ConfigProvider) (Provider, error) {
	providerName := config.GetMetadataProvider()

	switch providerName {
	case "mal":
		clientID := config.GetMALClientID()
		clientSecret := config.GetMALClientSecret()
		if clientID == "" || clientSecret == "" {
			return nil, fmt.Errorf("MAL client ID and secret are required")
		}
		combinedToken := clientID + ":" + clientSecret
		return GetProvider("mal", combinedToken)
	case "anilist":
		apiToken := config.GetAniListApiToken()
		return GetProvider("anilist", apiToken)
	case "jikan":
		return GetProvider("jikan", "") // Jikan doesn't require auth
	case "mangadex":
		return GetProvider("mangadex", "") // MangaDex doesn't require auth for public data
	case "mangaupdates":
		return GetProvider("mangaupdates", "") // MangaUpdates doesn't require auth for public data
	case "kitsu":
		return GetProvider("kitsu", "") // Kitsu doesn't require auth for public data
	default:
		// Default to MangaDex
		return GetProvider("mangadex", "")
	}
}

// GetProviderForLibrary returns the metadata provider for a specific library, falling back to global config
func GetProviderForLibrary(libraryProvider sql.NullString, config ConfigProvider) (Provider, error) {
	providerName := libraryProvider.String
	if !libraryProvider.Valid || providerName == "" {
		// Fall back to global config
		providerName = config.GetMetadataProvider()
	}

	var apiToken string
	switch providerName {
	case "mal":
		clientID := config.GetMALClientID()
		clientSecret := config.GetMALClientSecret()
		if clientID != "" && clientSecret != "" {
			apiToken = clientID + ":" + clientSecret
		}
	case "anilist":
		apiToken = config.GetAniListApiToken()
	case "jikan":
		apiToken = "" // Jikan doesn't require auth
	case "mangadex":
		apiToken = "" // MangaDex doesn't require auth for public data
	case "mangaupdates":
		apiToken = "" // MangaUpdates doesn't require auth for public data
	case "kitsu":
		apiToken = "" // Kitsu doesn't require auth for public data
	default:
		// Default to MangaDex
		providerName = "mangadex"
		apiToken = ""
	}

	provider, err := GetProvider(providerName, apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metadata provider '%s': %w", providerName, err)
	}

	provider.SetConfig(config)

	return provider, nil
}
