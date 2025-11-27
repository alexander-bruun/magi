package models

import (
    "database/sql"
    "sync"
)

// AppConfig holds global application settings (single-row table app_config id=1)
type AppConfig struct {
    AllowRegistration  bool
    MaxUsers           int64  // 0 means unlimited
    ContentRatingLimit int    // 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
    MetadataProvider   string // mangadex, mal, anilist, jikan
    MALApiToken        string // MyAnimeList API token
    AniListApiToken    string // AniList API token (optional)
    
    // Rate limiting settings
    RateLimitEnabled     bool
    RateLimitRequests    int  // requests per window
    RateLimitWindow      int  // window in seconds
    
    // Bot detection settings
    BotDetectionEnabled      bool
    BotSeriesThreshold       int  // max series accesses per time window
    BotChapterThreshold      int  // max chapter accesses per time window
    BotDetectionWindow       int  // time window in seconds for bot detection
    
    // Compression quality settings per role
    ReaderCompressionQuality    int // JPEG quality for reader role (0-100)
    ModeratorCompressionQuality int // JPEG quality for moderator role (0-100)
    AdminCompressionQuality     int // JPEG quality for admin role (0-100)
    PremiumCompressionQuality   int // JPEG quality for premium role (0-100)
    AnonymousCompressionQuality int // JPEG quality for anonymous users (0-100)
    ProcessedImageQuality       int // JPEG quality for processed images (thumbnails, covers) (0-100)
    
    // Image token settings
    ImageTokenValidityMinutes int // validity time for image access tokens in minutes
    
    // Premium early access settings
    PremiumEarlyAccessDuration int // duration in seconds that premium users can access chapters early
    MaxPremiumChapters         int // maximum number of chapters that can be premium (latest chapters)
    PremiumCooldownScalingEnabled bool // whether to scale cooldown based on chapter position
}

// Implement metadata.ConfigProvider interface
func (c *AppConfig) GetMetadataProvider() string {
    return c.MetadataProvider
}

func (c *AppConfig) GetMALApiToken() string {
    return c.MALApiToken
}

func (c *AppConfig) GetAniListApiToken() string {
    return c.AniListApiToken
}

var (
    cachedConfig AppConfig
    configOnce   sync.Once
    configMu     sync.RWMutex
)

// loadConfigFromDB loads the config row (id=1) from the database.
func loadConfigFromDB() (AppConfig, error) {
    row := db.QueryRow(`SELECT allow_registration, max_users, content_rating_limit, 
        COALESCE(metadata_provider, 'mangadex'), 
        COALESCE(mal_api_token, ''), 
        COALESCE(anilist_api_token, ''),
        COALESCE(rate_limit_enabled, 1),
        COALESCE(rate_limit_requests, 100),
        COALESCE(rate_limit_window, 60),
        COALESCE(bot_detection_enabled, 1),
        COALESCE(bot_series_threshold, 5),
        COALESCE(bot_chapter_threshold, 10),
        COALESCE(bot_detection_window, 60),
        COALESCE(reader_compression_quality, 70),
        COALESCE(moderator_compression_quality, 85),
        COALESCE(admin_compression_quality, 100),
        COALESCE(premium_compression_quality, 90),
        COALESCE(anonymous_compression_quality, 70),
        COALESCE(processed_image_quality, 85),
        COALESCE(image_token_validity_minutes, 5),
        COALESCE(premium_early_access_duration, 3600),
        COALESCE(max_premium_chapters, 3),
        COALESCE(premium_cooldown_scaling_enabled, 0)
        FROM app_config WHERE id = 1`)
    var allowInt int
    var maxUsers int64
    var contentRatingLimit int
    var metadataProvider string
    var malApiToken string
    var anilistApiToken string
    var rateLimitEnabled int
    var rateLimitRequests int
    var rateLimitWindow int
    var botDetectionEnabled int
    var botSeriesThreshold int
    var botChapterThreshold int
    var botDetectionWindow int
    var readerCompressionQuality int
    var moderatorCompressionQuality int
    var adminCompressionQuality int
    var premiumCompressionQuality int
    var anonymousCompressionQuality int
    var processedImageQuality int
    var imageTokenValidityMinutes int
    var premiumEarlyAccessDuration int
    var maxPremiumChapters int
    var premiumCooldownScalingEnabled int
    
    if err := row.Scan(&allowInt, &maxUsers, &contentRatingLimit, &metadataProvider, &malApiToken, &anilistApiToken, 
        &rateLimitEnabled, &rateLimitRequests, &rateLimitWindow, &botDetectionEnabled, &botSeriesThreshold, &botChapterThreshold, &botDetectionWindow,
        &readerCompressionQuality, &moderatorCompressionQuality, &adminCompressionQuality, &premiumCompressionQuality, &anonymousCompressionQuality, &processedImageQuality, &imageTokenValidityMinutes, &premiumEarlyAccessDuration, &maxPremiumChapters, &premiumCooldownScalingEnabled); err != nil {
        if err == sql.ErrNoRows {
            // Fallback defaults if row missing.
            return AppConfig{
                AllowRegistration:  true,
                MaxUsers:           0,
                ContentRatingLimit: 3,
                MetadataProvider:   "mangadex",
                MALApiToken:        "",
                AniListApiToken:    "",
                RateLimitEnabled:   true,
                RateLimitRequests:  100,
                RateLimitWindow:    60,
                BotDetectionEnabled: true,
                BotSeriesThreshold:  5,
                BotChapterThreshold: 10,
                BotDetectionWindow:  60,
                ReaderCompressionQuality:    70,
                ModeratorCompressionQuality: 85,
                AdminCompressionQuality:     100,
                PremiumCompressionQuality:   90,
                AnonymousCompressionQuality: 70,
                ProcessedImageQuality:       85,
                ImageTokenValidityMinutes:   5,
                PremiumEarlyAccessDuration:  3600,
                MaxPremiumChapters:         3,
                PremiumCooldownScalingEnabled: false,
            }, nil
        }
        return AppConfig{}, err
    }
    
    return AppConfig{
        AllowRegistration:  allowInt == 1,
        MaxUsers:           maxUsers,
        ContentRatingLimit: contentRatingLimit,
        MetadataProvider:   metadataProvider,
        MALApiToken:        malApiToken,
        AniListApiToken:    anilistApiToken,
        RateLimitEnabled:   rateLimitEnabled == 1,
        RateLimitRequests:  rateLimitRequests,
        RateLimitWindow:    rateLimitWindow,
        BotDetectionEnabled: botDetectionEnabled == 1,
        BotSeriesThreshold:  botSeriesThreshold,
        BotChapterThreshold: botChapterThreshold,
        BotDetectionWindow:  botDetectionWindow,
        ReaderCompressionQuality:    readerCompressionQuality,
        ModeratorCompressionQuality: moderatorCompressionQuality,
        AdminCompressionQuality:     adminCompressionQuality,
        PremiumCompressionQuality:   premiumCompressionQuality,
        AnonymousCompressionQuality: anonymousCompressionQuality,
        ProcessedImageQuality:       processedImageQuality,
        ImageTokenValidityMinutes:   imageTokenValidityMinutes,
        PremiumEarlyAccessDuration:  premiumEarlyAccessDuration,
        MaxPremiumChapters:         maxPremiumChapters,
        PremiumCooldownScalingEnabled: premiumCooldownScalingEnabled == 1,
    }, nil
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

// UpdateRateLimitConfig updates the rate limiting configuration
func UpdateRateLimitConfig(enabled bool, requests, window int) (AppConfig, error) {
    enabledInt := 0
    if enabled {
        enabledInt = 1
    }
    _, err := db.Exec(`UPDATE app_config SET rate_limit_enabled = ?, rate_limit_requests = ?, rate_limit_window = ? WHERE id = 1`, 
        enabledInt, requests, window)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdateCompressionConfig updates the compression quality settings per role
func UpdateCompressionConfig(readerQuality, moderatorQuality, adminQuality, premiumQuality, anonymousQuality, processedQuality int) (AppConfig, error) {
    // Ensure qualities are within valid range (0-100)
    if readerQuality < 0 {
        readerQuality = 0
    }
    if readerQuality > 100 {
        readerQuality = 100
    }
    if moderatorQuality < 0 {
        moderatorQuality = 0
    }
    if moderatorQuality > 100 {
        moderatorQuality = 100
    }
    if adminQuality < 0 {
        adminQuality = 0
    }
    if adminQuality > 100 {
        adminQuality = 100
    }
    if premiumQuality < 0 {
        premiumQuality = 0
    }
    if premiumQuality > 100 {
        premiumQuality = 100
    }
    if anonymousQuality < 0 {
        anonymousQuality = 0
    }
    if anonymousQuality > 100 {
        anonymousQuality = 100
    }
    if processedQuality < 0 {
        processedQuality = 0
    }
    if processedQuality > 100 {
        processedQuality = 100
    }
    _, err := db.Exec(`UPDATE app_config SET reader_compression_quality = ?, moderator_compression_quality = ?, admin_compression_quality = ?, premium_compression_quality = ?, anonymous_compression_quality = ?, processed_image_quality = ? WHERE id = 1`,
        readerQuality, moderatorQuality, adminQuality, premiumQuality, anonymousQuality, processedQuality)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdatePremiumEarlyAccessConfig updates the premium early access duration
func UpdatePremiumEarlyAccessConfig(duration int) (AppConfig, error) {
    // Ensure duration is positive
    if duration < 0 {
        duration = 0
    }
    _, err := db.Exec(`UPDATE app_config SET premium_early_access_duration = ? WHERE id = 1`, duration)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdateMaxPremiumChaptersConfig updates the maximum number of premium chapters
func UpdateMaxPremiumChaptersConfig(maxChapters int) (AppConfig, error) {
    // Ensure maxChapters is positive
    if maxChapters < 0 {
        maxChapters = 0
    }
    _, err := db.Exec(`UPDATE app_config SET max_premium_chapters = ? WHERE id = 1`, maxChapters)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdatePremiumCooldownScalingConfig updates whether premium cooldown scaling is enabled
func UpdatePremiumCooldownScalingConfig(enabled bool) (AppConfig, error) {
    enabledInt := 0
    if enabled {
        enabledInt = 1
    }
    _, err := db.Exec(`UPDATE app_config SET premium_cooldown_scaling_enabled = ? WHERE id = 1`, enabledInt)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdateMetadataConfig updates the metadata provider configuration
func UpdateMetadataConfig(provider, malToken, anilistToken string) (AppConfig, error) {
    // Validate provider
    validProviders := map[string]bool{
        "mangadex": true,
        "mal":      true,
        "anilist":  true,
        "jikan":    true,
    }
    if !validProviders[provider] {
        provider = "mangadex"
    }
    
    _, err := db.Exec(`UPDATE app_config SET metadata_provider = ?, mal_api_token = ?, anilist_api_token = ? WHERE id = 1`, 
        provider, malToken, anilistToken)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdateBotDetectionConfig updates the bot detection configuration
func UpdateBotDetectionConfig(enabled bool, seriesThreshold, chapterThreshold, detectionWindow int) (AppConfig, error) {
    enabledInt := 0
    if enabled {
        enabledInt = 1
    }
    _, err := db.Exec(`UPDATE app_config SET bot_detection_enabled = ?, bot_series_threshold = ?, bot_chapter_threshold = ?, bot_detection_window = ? WHERE id = 1`, 
        enabledInt, seriesThreshold, chapterThreshold, detectionWindow)
    if err != nil {
        return AppConfig{}, err
    }
    return RefreshAppConfig()
}

// UpdateImageTokenConfig updates the image token validity time configuration
func UpdateImageTokenConfig(validityMinutes int) (AppConfig, error) {
    // Ensure validity is within reasonable range (1-60 minutes)
    if validityMinutes < 1 {
        validityMinutes = 1
    }
    if validityMinutes > 60 {
        validityMinutes = 60
    }
    _, err := db.Exec(`UPDATE app_config SET image_token_validity_minutes = ? WHERE id = 1`, validityMinutes)
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

// GetCompressionQualityForRole returns the JPEG compression quality for the given user role
func GetCompressionQualityForRole(role string) int {
    cfg, err := GetAppConfig()
    if err != nil {
        // Return default if config can't be loaded
        switch role {
        case "admin":
            return 100
        case "moderator":
            return 85
        case "premium":
            return 90
        case "anonymous":
            return 70
        default:
            return 70
        }
    }
    
    switch role {
    case "admin":
        return cfg.AdminCompressionQuality
    case "moderator":
        return cfg.ModeratorCompressionQuality
    case "premium":
        return cfg.PremiumCompressionQuality
    case "anonymous":
        return cfg.AnonymousCompressionQuality
    default:
        return cfg.ReaderCompressionQuality
    }
}

// GetImageTokenValidityMinutes returns the configured validity time for image tokens in minutes
func GetImageTokenValidityMinutes() int {
    cfg, err := GetAppConfig()
    if err != nil {
        // Return default if config can't be loaded
        return 5
    }
    return cfg.ImageTokenValidityMinutes
}

// GetProcessedImageQuality returns the JPEG compression quality for processed images (thumbnails, covers)
func GetProcessedImageQuality() int {
    cfg, err := GetAppConfig()
    if err != nil {
        // Return default if config can't be loaded
        return 85
    }
    return cfg.ProcessedImageQuality
}