package metadata

import (
	"database/sql"
	"fmt"
)

// ConfigProvider is an interface for accessing app configuration
// This allows the metadata package to get provider config without importing models
type ConfigProvider interface {
	GetMetadataProvider() string
	GetMALApiToken() string
	GetAniListApiToken() string
}

// LibraryConfigProvider extends ConfigProvider with library-specific settings
type LibraryConfigProvider interface {
	ConfigProvider
	GetLibraryMetadataProvider() string
}

// GetProviderFromConfig returns the configured metadata provider instance
func GetProviderFromConfig(config ConfigProvider) (Provider, error) {
	providerName := config.GetMetadataProvider()
	
	var apiToken string
	switch providerName {
	case "mal":
		apiToken = config.GetMALApiToken()
	case "anilist":
		apiToken = config.GetAniListApiToken()
	case "jikan":
		apiToken = "" // Jikan doesn't require auth
	case "mangadex":
		apiToken = "" // Mediadex doesn't require auth for public data
	default:
		// Default to Mediadex
		providerName = "mangadex"
		apiToken = ""
	}

	provider, err := GetProvider(providerName, apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metadata provider '%s': %w", providerName, err)
	}

	return provider, nil
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
		apiToken = config.GetMALApiToken()
	case "anilist":
		apiToken = config.GetAniListApiToken()
	case "jikan":
		apiToken = "" // Jikan doesn't require auth
	case "mangadex":
		apiToken = "" // Mediadex doesn't require auth for public data
	default:
		// Default to Mediadex
		providerName = "mangadex"
		apiToken = ""
	}

	provider, err := GetProvider(providerName, apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metadata provider '%s': %w", providerName, err)
	}

	return provider, nil
}
