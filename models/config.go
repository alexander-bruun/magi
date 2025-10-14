package models

import (
    "database/sql"
    "sync"
)

// AppConfig holds global application settings (single-row table app_config id=1)
type AppConfig struct {
    AllowRegistration bool
    MaxUsers          int64 // 0 means unlimited
}

var (
    cachedConfig AppConfig
    configOnce   sync.Once
    configMu     sync.RWMutex
)

// loadConfigFromDB loads the config row (id=1) from the database.
func loadConfigFromDB() (AppConfig, error) {
    row := db.QueryRow(`SELECT allow_registration, max_users FROM app_config WHERE id = 1`)
    var allowInt int
    var maxUsers int64
    if err := row.Scan(&allowInt, &maxUsers); err != nil {
        if err == sql.ErrNoRows {
            // Fallback defaults if row missing.
            return AppConfig{AllowRegistration: true, MaxUsers: 0}, nil
        }
        return AppConfig{}, err
    }
    return AppConfig{AllowRegistration: allowInt == 1, MaxUsers: maxUsers}, nil
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
func UpdateAppConfig(allowRegistration bool, maxUsers int64) (AppConfig, error) {
    allow := 0
    if allowRegistration {
        allow = 1
    }
    _, err := db.Exec(`UPDATE app_config SET allow_registration = ?, max_users = ? WHERE id = 1`, allow, maxUsers)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}
