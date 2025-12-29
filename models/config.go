package models

import (
	"database/sql"
	"sync"
	"time"
)

// AppConfig holds global application settings (single-row table app_config id=1)
type AppConfig struct {
	AllowRegistration  bool   `json:"allow_registration" form:"allow_registration"`
	MaxUsers           int64  `json:"max_users" form:"max_users"`                       // 0 means unlimited
	ContentRatingLimit int    `json:"content_rating_limit" form:"content_rating_limit"` // 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
	MetadataProvider   string `json:"metadata_provider" form:"metadata_provider"`       // mangadex, mal, anilist, jikan, mangaupdates, kitsu
	MALApiToken        string `json:"mal_api_token" form:"mal_api_token"`               // MyAnimeList API token
	AniListApiToken    string `json:"anilist_api_token" form:"anilist_api_token"`       // AniList API token (optional)
	ImageAccessSecret  string `json:"image_access_secret" form:"image_access_secret"`

	// Stripe payment settings
	StripeEnabled        bool   `json:"stripe_enabled" form:"stripe_enabled"`                 // whether Stripe payments are enabled
	StripePublishableKey string `json:"stripe_publishable_key" form:"stripe_publishable_key"` // Stripe publishable key
	StripeSecretKey      string `json:"stripe_secret_key" form:"stripe_secret_key"`           // Stripe secret key
	StripeWebhookSecret  string `json:"stripe_webhook_secret" form:"stripe_webhook_secret"`   // Stripe webhook secret for verification

	// Rate limiting settings
	RateLimitEnabled  bool `json:"rate_limit_enabled" form:"rate_limit_enabled"`
	RateLimitRequests int  `json:"rate_limit_requests" form:"rate_limit_requests"` // requests per window
	RateLimitWindow   int  `json:"rate_limit_window" form:"rate_limit_window"`     // window in seconds

	// Bot detection settings
	BotDetectionEnabled bool `json:"bot_detection_enabled" form:"bot_detection_enabled"`
	BotSeriesThreshold  int  `json:"bot_series_threshold" form:"bot_series_threshold"`   // max series accesses per time window
	BotChapterThreshold int  `json:"bot_chapter_threshold" form:"bot_chapter_threshold"` // max chapter accesses per time window
	BotDetectionWindow  int  `json:"bot_detection_window" form:"bot_detection_window"`   // time window in seconds for bot detection

	// Compression quality settings per role
	ReaderCompressionQuality    int `json:"reader_compression_quality" form:"reader_compression_quality"`       // JPEG quality for reader role (0-100)
	ModeratorCompressionQuality int `json:"moderator_compression_quality" form:"moderator_compression_quality"` // JPEG quality for moderator role (0-100)
	AdminCompressionQuality     int `json:"admin_compression_quality" form:"admin_compression_quality"`         // JPEG quality for admin role (0-100)
	PremiumCompressionQuality   int `json:"premium_compression_quality" form:"premium_compression_quality"`     // JPEG quality for premium role (0-100)
	AnonymousCompressionQuality int `json:"anonymous_compression_quality" form:"anonymous_compression_quality"` // JPEG quality for anonymous users (0-100)
	ProcessedImageQuality       int `json:"processed_image_quality" form:"processed_image_quality"`             // Image quality for processed images (thumbnails, covers) (0-100)

	// Image token settings
	ImageTokenValidityMinutes int `json:"image_token_validity_minutes" form:"image_token_validity_minutes"` // validity time for image access tokens in minutes

	// Premium early access settings
	PremiumEarlyAccessDuration    int  `json:"premium_early_access_duration" form:"premium_early_access_duration"`       // duration in seconds that premium users can access chapters early
	MaxPremiumChapters            int  `json:"max_premium_chapters" form:"max_premium_chapters"`                         // maximum number of chapters that can be premium (latest chapters)
	PremiumCooldownScalingEnabled bool `json:"premium_cooldown_scaling_enabled" form:"premium_cooldown_scaling_enabled"` // whether to scale cooldown based on chapter position

	// Maintenance mode settings
	MaintenanceEnabled bool   `json:"maintenance_enabled" form:"maintenance_enabled"` // whether maintenance mode is active
	MaintenanceMessage string `json:"maintenance_message" form:"maintenance_message"` // custom message to display during maintenance

	// New media badge settings
	NewBadgeDuration int `json:"new_badge_duration" form:"new_badge_duration"` // duration in hours that media is marked as NEW after update
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

func (c *AppConfig) GetContentRatingLimit() int {
	return c.ContentRatingLimit
}

var (
	cachedConfig    AppConfig
	configOnce      sync.Once
	configMu        sync.RWMutex
	configCacheTime time.Time
	configCacheTTL  = 5 * time.Minute // Cache config for 5 minutes to reduce lock contention
)

// loadConfigFromDB loads the config row (id=1) from the database.
func loadConfigFromDB() (AppConfig, error) {
	row := db.QueryRow(`SELECT allow_registration, max_users, content_rating_limit, 
        COALESCE(metadata_provider, 'mangadex'), 
        COALESCE(mal_api_token, ''), 
        COALESCE(anilist_api_token, ''),
        COALESCE(image_access_secret, ''),
        COALESCE(stripe_enabled, 0),
        COALESCE(stripe_publishable_key, ''),
        COALESCE(stripe_secret_key, ''),
        COALESCE(stripe_webhook_secret, ''),
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
        COALESCE(premium_cooldown_scaling_enabled, 0),
        COALESCE(maintenance_enabled, 0),
        COALESCE(maintenance_message, 'We are currently performing maintenance. Please check back later.'),
        COALESCE(new_badge_duration, 48)
        FROM app_config WHERE id = 1`)
	var allowInt int
	var maxUsers int64
	var contentRatingLimit int
	var metadataProvider string
	var malApiToken string
	var anilistApiToken string
	var imageAccessSecret string
	var stripeEnabled int
	var stripePublishableKey string
	var stripeSecretKey string
	var stripeWebhookSecret string
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
	var maintenanceEnabled int
	var maintenanceMessage string
	var newBadgeDuration int

	if err := row.Scan(&allowInt, &maxUsers, &contentRatingLimit, &metadataProvider, &malApiToken, &anilistApiToken, &imageAccessSecret,
		&stripeEnabled, &stripePublishableKey, &stripeSecretKey, &stripeWebhookSecret,
		&rateLimitEnabled, &rateLimitRequests, &rateLimitWindow, &botDetectionEnabled, &botSeriesThreshold, &botChapterThreshold, &botDetectionWindow,
		&readerCompressionQuality, &moderatorCompressionQuality, &adminCompressionQuality, &premiumCompressionQuality, &anonymousCompressionQuality, &processedImageQuality, &imageTokenValidityMinutes, &premiumEarlyAccessDuration, &maxPremiumChapters, &premiumCooldownScalingEnabled, &maintenanceEnabled, &maintenanceMessage, &newBadgeDuration); err != nil {
		if err == sql.ErrNoRows {
			// Fallback defaults if row missing.
			return AppConfig{
				AllowRegistration:             true,
				MaxUsers:                      0,
				ContentRatingLimit:            3,
				MetadataProvider:              "mangadex",
				MALApiToken:                   "",
				AniListApiToken:               "",
				ImageAccessSecret:             "",
				StripeEnabled:                 false,
				StripePublishableKey:          "",
				StripeSecretKey:               "",
				StripeWebhookSecret:           "",
				RateLimitEnabled:              true,
				RateLimitRequests:             100,
				RateLimitWindow:               60,
				BotDetectionEnabled:           true,
				BotSeriesThreshold:            5,
				BotChapterThreshold:           10,
				BotDetectionWindow:            60,
				ReaderCompressionQuality:      70,
				ModeratorCompressionQuality:   85,
				AdminCompressionQuality:       100,
				PremiumCompressionQuality:     90,
				AnonymousCompressionQuality:   70,
				ProcessedImageQuality:         85,
				ImageTokenValidityMinutes:     5,
				PremiumEarlyAccessDuration:    3600,
				MaxPremiumChapters:            3,
				PremiumCooldownScalingEnabled: false,
				MaintenanceEnabled:            false,
				MaintenanceMessage:            "We are currently performing maintenance. Please check back later.",
				NewBadgeDuration:              48,
			}, nil
		}
		return AppConfig{}, err
	}

	return AppConfig{
		AllowRegistration:             allowInt == 1,
		MaxUsers:                      maxUsers,
		ContentRatingLimit:            contentRatingLimit,
		MetadataProvider:              metadataProvider,
		MALApiToken:                   malApiToken,
		AniListApiToken:               anilistApiToken,
		ImageAccessSecret:             imageAccessSecret,
		StripeEnabled:                 stripeEnabled == 1,
		StripePublishableKey:          stripePublishableKey,
		StripeSecretKey:               stripeSecretKey,
		StripeWebhookSecret:           stripeWebhookSecret,
		RateLimitEnabled:              rateLimitEnabled == 1,
		RateLimitRequests:             rateLimitRequests,
		RateLimitWindow:               rateLimitWindow,
		BotDetectionEnabled:           botDetectionEnabled == 1,
		BotSeriesThreshold:            botSeriesThreshold,
		BotChapterThreshold:           botChapterThreshold,
		BotDetectionWindow:            botDetectionWindow,
		ReaderCompressionQuality:      readerCompressionQuality,
		ModeratorCompressionQuality:   moderatorCompressionQuality,
		AdminCompressionQuality:       adminCompressionQuality,
		PremiumCompressionQuality:     premiumCompressionQuality,
		AnonymousCompressionQuality:   anonymousCompressionQuality,
		ProcessedImageQuality:         processedImageQuality,
		ImageTokenValidityMinutes:     imageTokenValidityMinutes,
		PremiumEarlyAccessDuration:    premiumEarlyAccessDuration,
		MaxPremiumChapters:            maxPremiumChapters,
		PremiumCooldownScalingEnabled: premiumCooldownScalingEnabled == 1,
		MaintenanceEnabled:            maintenanceEnabled == 1,
		MaintenanceMessage:            maintenanceMessage,
		NewBadgeDuration:              newBadgeDuration,
	}, nil
}

// GetAppConfig returns the cached configuration with TTL-based refresh
// Cache is valid for 5 minutes to balance freshness with performance (reduces lock contention)
func GetAppConfig() (AppConfig, error) {
	configMu.RLock()
	// Check if cache is still valid (fast path - read lock only)
	if !configCacheTime.IsZero() && time.Since(configCacheTime) < configCacheTTL {
		cfg := cachedConfig
		configMu.RUnlock()
		return cfg, nil
	}
	configMu.RUnlock()

	// Cache expired or not yet loaded - refresh it
	cfg, err := loadConfigFromDB()
	if err != nil {
		return AppConfig{}, err
	}

	configMu.Lock()
	cachedConfig = cfg
	configCacheTime = time.Now()
	configMu.Unlock()

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
		"mangadex":     true,
		"mal":          true,
		"anilist":      true,
		"jikan":        true,
		"mangaupdates": true,
		"kitsu":        true,
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

// UpdateNewBadgeDurationConfig updates the duration in hours that media is marked as NEW after update
func UpdateNewBadgeDurationConfig(hours int) (AppConfig, error) {
	// Ensure hours is at least 1
	if hours < 1 {
		hours = 1
	}
	_, err := db.Exec(`UPDATE app_config SET new_badge_duration = ? WHERE id = 1`, hours)
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
		return 60
	}
	return cfg.ImageTokenValidityMinutes
}

// GetProcessedImageQuality returns the image compression quality for processed images (thumbnails, covers)
func GetProcessedImageQuality() int {
	cfg, err := GetAppConfig()
	if err != nil {
		// Return default if config can't be loaded
		return 85
	}
	return cfg.ProcessedImageQuality
}

// UpdateMaintenanceConfig updates the maintenance mode settings
func UpdateMaintenanceConfig(enabled bool, message string) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE app_config SET maintenance_enabled = ?, maintenance_message = ? WHERE id = 1`,
		enabledInt, message)
	if err != nil {
		return AppConfig{}, err
	}

	// Invalidate cache to force reload
	configMu.Lock()
	cachedConfig = AppConfig{}
	configCacheTime = time.Time{}
	configMu.Unlock()

	return GetAppConfig()
}

// UpdateStripeConfig updates the Stripe payment configuration
func UpdateStripeConfig(enabled bool, publishableKey, secretKey, webhookSecret string) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err := db.Exec(`UPDATE app_config SET stripe_enabled = ?, stripe_publishable_key = ?, stripe_secret_key = ?, stripe_webhook_secret = ? WHERE id = 1`,
		enabledInt, publishableKey, secretKey, webhookSecret)
	if err != nil {
		return AppConfig{}, err
	}

	return GetAppConfig()
}

// GetMaintenanceStatus returns whether maintenance mode is active and the message
func GetMaintenanceStatus() (enabled bool, message string, err error) {
	cfg, err := GetAppConfig()
	if err != nil {
		return false, "", err
	}
	return cfg.MaintenanceEnabled, cfg.MaintenanceMessage, nil
}
