package models

import (
    "database/sql"
    "sync"
)

// AppConfig holds global application settings (single-row table app_config id=1)
type AppConfig struct {
    AllowRegistration  bool
    MaxUsers           int64 // 0 means unlimited
    ContentRatingLimit int   // 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
}

var (
    cachedConfig AppConfig
    configOnce   sync.Once
    configMu     sync.RWMutex
)

// loadConfigFromDB loads the config row (id=1) from the database.
func loadConfigFromDB() (AppConfig, error) {
    row := db.QueryRow(`SELECT allow_registration, max_users, content_rating_limit FROM app_config WHERE id = 1`)
    var allowInt int
    var maxUsers int64
    var contentRatingLimit int
    if err := row.Scan(&allowInt, &maxUsers, &contentRatingLimit); err != nil {
        if err == sql.ErrNoRows {
            // Fallback defaults if row missing.
            return AppConfig{AllowRegistration: true, MaxUsers: 0, ContentRatingLimit: 3}, nil
        }
        return AppConfig{}, err
    }
    return AppConfig{AllowRegistration: allowInt == 1, MaxUsers: maxUsers, ContentRatingLimit: contentRatingLimit}, nil
}

// GetAppConfig returns the cached configuration, loading it from the DB once or when forced refresh.
func GetAppConfig() (AppConfig, error) {
    var err error
    configOnce.Do(func() {
        var cfg AppConfig
        cfg, err = loadConfigFromDB()
        if err == nil {
            cachedConfig = cfg
        }
    })
    if err != nil {
        return AppConfig{}, err
    }
    configMu.RLock()
    cfg := cachedConfig
    configMu.RUnlock()
    return cfg, nil
}

// RefreshAppConfig forces a reload from the database (used after updates).
func RefreshAppConfig() (AppConfig, error) {
    cfg, err := loadConfigFromDB()
    if err != nil {
        return AppConfig{}, err
    }
    configMu.Lock()
    cachedConfig = cfg
    configMu.Unlock()
    return cfg, nil
}

// UpdateAppConfig updates the settings atomically and refreshes cache.
func UpdateAppConfig(allowRegistration bool, maxUsers int64, contentRatingLimit int) (AppConfig, error) {
    allow := 0
    if allowRegistration {
        allow = 1
    }
    // Ensure content rating limit is within valid range (0-3)
    if contentRatingLimit < 0 {
        contentRatingLimit = 0
    }
    if contentRatingLimit > 3 {
        contentRatingLimit = 3
    }
    _, err := db.Exec(`UPDATE app_config SET allow_registration = ?, max_users = ?, content_rating_limit = ? WHERE id = 1`, allow, maxUsers, contentRatingLimit)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// ContentRatingToInt converts a content rating string to its integer level
// 0=safe, 1=suggestive, 2=erotica, 3=pornographic
// https://api.mangadex.org/docs/3-enumerations/#manga-content-rating
func ContentRatingToInt(rating string) int {
    switch rating {
    case "safe":
        return 0
    case "suggestive":
        return 1
    case "erotica":
        return 2
    case "pornographic":
        return 3
    default:
        return 3 // Unknown ratings default to highest (show all)
    }
}

// IsContentRatingAllowed checks if a content rating is within the configured limit
func IsContentRatingAllowed(rating string, limit int) bool {
    ratingLevel := ContentRatingToInt(rating)
    return ratingLevel <= limit
}
