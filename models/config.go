package models

import (
	"database/sql"
	"sync/atomic"
	"time"
)

// AppConfig holds global application settings (single-row table app_config id=1)
type AppConfig struct {
	AllowRegistration  bool   `json:"allow_registration" form:"allow_registration"`
	MaxUsers           int64  `json:"max_users" form:"max_users"`                       // 0 means unlimited
	ContentRatingLimit int    `json:"content_rating_limit" form:"content_rating_limit"` // 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
	MetadataProvider   string `json:"metadata_provider" form:"metadata_provider"`       // mangadex, mal, anilist, jikan, mangaupdates, kitsu
	MALClientID        string `json:"mal_client_id" form:"mal_client_id"`               // MyAnimeList OAuth2 client ID
	MALClientSecret    string `json:"mal_client_secret" form:"mal_client_secret"`       // MyAnimeList OAuth2 client secret
	AniListApiToken    string `json:"anilist_api_token" form:"anilist_api_token"`       // AniList API token (optional)
	ImageAccessSecret  string `json:"image_access_secret" form:"image_access_secret"`

	// Stripe payment settings
	StripeEnabled        bool   `json:"stripe_enabled" form:"stripe_enabled"`                 // whether Stripe payments are enabled
	StripePublishableKey string `json:"stripe_publishable_key" form:"stripe_publishable_key"` // Stripe publishable key
	StripeSecretKey      string `json:"stripe_secret_key" form:"stripe_secret_key"`           // Stripe secret key
	StripeWebhookSecret  string `json:"stripe_webhook_secret" form:"stripe_webhook_secret"`   // Stripe webhook secret for verification

	// Rate limiting settings
	RateLimitEnabled       bool `json:"rate_limit_enabled" form:"rate_limit_enabled"`
	RateLimitRequests      int  `json:"rate_limit_requests" form:"rate_limit_requests"`             // requests per window
	RateLimitWindow        int  `json:"rate_limit_window" form:"rate_limit_window"`                 // window in seconds
	RateLimitBlockDuration int  `json:"rate_limit_block_duration" form:"rate_limit_block_duration"` // block duration in seconds

	// Bot detection settings
	BotDetectionEnabled bool `json:"bot_detection_enabled" form:"bot_detection_enabled"`
	BotSeriesThreshold  int  `json:"bot_series_threshold" form:"bot_series_threshold"`   // max series accesses per time window
	BotChapterThreshold int  `json:"bot_chapter_threshold" form:"bot_chapter_threshold"` // max chapter accesses per time window
	BotDetectionWindow  int  `json:"bot_detection_window" form:"bot_detection_window"`   // time window in seconds for bot detection

	// Browser challenge settings (invisible JS/cookie challenge)
	BrowserChallengeEnabled       bool `json:"browser_challenge_enabled" form:"browser_challenge_enabled"`               // whether browser challenge is enabled
	BrowserChallengeDifficulty    int  `json:"browser_challenge_difficulty" form:"browser_challenge_difficulty"`         // proof-of-work difficulty (number of leading zeros)
	BrowserChallengeValidityHours int  `json:"browser_challenge_validity_hours" form:"browser_challenge_validity_hours"` // how long the challenge verification is valid
	BrowserChallengeIPBound       bool `json:"browser_challenge_ip_bound" form:"browser_challenge_ip_bound"`             // whether to bind verification to IP address

	// Referer validation settings
	RefererValidationEnabled bool `json:"referer_validation_enabled" form:"referer_validation_enabled"` // whether to validate Referer headers

	// Tarpit settings
	TarpitEnabled  bool `json:"tarpit_enabled" form:"tarpit_enabled"`     // whether tarpit is enabled
	TarpitMaxDelay int  `json:"tarpit_max_delay" form:"tarpit_max_delay"` // maximum delay in milliseconds

	// Timing analysis settings
	TimingAnalysisEnabled   bool    `json:"timing_analysis_enabled" form:"timing_analysis_enabled"`     // whether timing analysis is enabled
	TimingVarianceThreshold float64 `json:"timing_variance_threshold" form:"timing_variance_threshold"` // coefficient of variation threshold

	// TLS fingerprint settings
	TLSFingerprintEnabled bool `json:"tls_fingerprint_enabled" form:"tls_fingerprint_enabled"` // whether TLS fingerprinting is enabled
	TLSFingerprintStrict  bool `json:"tls_fingerprint_strict" form:"tls_fingerprint_strict"`   // whether to block suspicious fingerprints

	// Behavioral analysis settings
	BehavioralAnalysisEnabled bool `json:"behavioral_analysis_enabled" form:"behavioral_analysis_enabled"` // whether behavioral analysis is enabled
	BehavioralScoreThreshold  int  `json:"behavioral_score_threshold" form:"behavioral_score_threshold"`   // score below which to flag as suspicious

	// Header analysis settings
	HeaderAnalysisEnabled   bool `json:"header_analysis_enabled" form:"header_analysis_enabled"`     // whether header analysis is enabled
	HeaderAnalysisThreshold int  `json:"header_analysis_threshold" form:"header_analysis_threshold"` // suspicion score threshold
	HeaderAnalysisStrict    bool `json:"header_analysis_strict" form:"header_analysis_strict"`       // whether to block suspicious requests

	// Honeypot settings
	HoneypotEnabled       bool `json:"honeypot_enabled" form:"honeypot_enabled"`               // whether honeypot is enabled
	HoneypotAutoBlock     bool `json:"honeypot_auto_block" form:"honeypot_auto_block"`         // whether to auto-block IPs that trigger honeypots temporarily
	HoneypotAutoBan       bool `json:"honeypot_auto_ban" form:"honeypot_auto_ban"`             // whether to auto-ban IPs that trigger honeypots permanently
	HoneypotBlockDuration int  `json:"honeypot_block_duration" form:"honeypot_block_duration"` // block duration in minutes

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

	// Parallel indexing settings
	ParallelIndexingEnabled   bool `json:"parallel_indexing_enabled" form:"parallel_indexing_enabled"`     // whether parallel indexing is enabled
	ParallelIndexingThreshold int  `json:"parallel_indexing_threshold" form:"parallel_indexing_threshold"` // minimum series count to trigger parallel processing

	// Discord settings
	DiscordInviteLink string `json:"discord_invite_link" form:"discord_invite_link"` // Discord invite link for community access
}

// Implement metadata.ConfigProvider interface
func (c *AppConfig) GetMetadataProvider() string {
	return c.MetadataProvider
}

func (c *AppConfig) GetMALClientID() string {
	return c.MALClientID
}

func (c *AppConfig) GetMALClientSecret() string {
	return c.MALClientSecret
}

func (c *AppConfig) GetAniListApiToken() string {
	return c.AniListApiToken
}

func (c *AppConfig) GetContentRatingLimit() int {
	return c.ContentRatingLimit
}

var (
	configCache     atomic.Value // AppConfig
	configCacheTime time.Time
	configCacheTTL  = 5 * time.Minute // Cache config for 5 minutes to reduce lock contention
)

// loadConfigFromDB loads the config row (id=1) from the database.
func loadConfigFromDB() (AppConfig, error) {
	row := db.QueryRow(`SELECT allow_registration, max_users, content_rating_limit, 
        COALESCE(metadata_provider, 'mangadex'), 
        COALESCE(mal_client_id, ''), 
        COALESCE(mal_client_secret, ''),
        COALESCE(anilist_api_token, ''),
        COALESCE(image_access_secret, ''),
        COALESCE(stripe_enabled, 0),
        COALESCE(stripe_publishable_key, ''),
        COALESCE(stripe_secret_key, ''),
        COALESCE(stripe_webhook_secret, ''),
        COALESCE(rate_limit_enabled, 1),
        COALESCE(rate_limit_requests, 100),
        COALESCE(rate_limit_window, 60),
        COALESCE(rate_limit_block_duration, 300),
        COALESCE(bot_detection_enabled, 1),
        COALESCE(bot_series_threshold, 5),
        COALESCE(bot_chapter_threshold, 10),
        COALESCE(bot_detection_window, 60),
        COALESCE(browser_challenge_enabled, 0),
        COALESCE(browser_challenge_difficulty, 3),
        COALESCE(browser_challenge_validity_hours, 24),
        COALESCE(browser_challenge_ip_bound, 0),
        COALESCE(referer_validation_enabled, 0),
        COALESCE(tarpit_enabled, 0),
        COALESCE(tarpit_max_delay, 5000),
        COALESCE(timing_analysis_enabled, 0),
        COALESCE(timing_variance_threshold, 0.1),
        COALESCE(tls_fingerprint_enabled, 0),
        COALESCE(tls_fingerprint_strict, 0),
        COALESCE(behavioral_analysis_enabled, 0),
        COALESCE(behavioral_score_threshold, 40),
        COALESCE(header_analysis_enabled, 0),
        COALESCE(header_analysis_threshold, 5),
        COALESCE(header_analysis_strict, 0),
        COALESCE(honeypot_enabled, 0),
        COALESCE(honeypot_auto_block, 1),
        COALESCE(honeypot_auto_ban, 0),
        COALESCE(honeypot_block_duration, 60),
        COALESCE(image_token_validity_minutes, 5),
        COALESCE(premium_early_access_duration, 3600),
        COALESCE(max_premium_chapters, 3),
        COALESCE(premium_cooldown_scaling_enabled, 0),
        COALESCE(maintenance_enabled, 0),
        COALESCE(maintenance_message, 'We are currently performing maintenance. Please check back later.'),
        COALESCE(new_badge_duration, 48),
        COALESCE(parallel_indexing_enabled, 1),
        COALESCE(parallel_indexing_threshold, 100),
        COALESCE(discord_invite_link, '')
        FROM app_config WHERE id = 1`)
	var allowInt int
	var maxUsers int64
	var contentRatingLimit int
	var metadataProvider string
	var malClientID string
	var malClientSecret string
	var anilistApiToken string
	var imageAccessSecret string
	var stripeEnabled int
	var stripePublishableKey string
	var stripeSecretKey string
	var stripeWebhookSecret string
	var rateLimitEnabled int
	var rateLimitRequests int
	var rateLimitWindow int
	var rateLimitBlockDuration int
	var botDetectionEnabled int
	var botSeriesThreshold int
	var botChapterThreshold int
	var botDetectionWindow int
	var browserChallengeEnabled int
	var browserChallengeDifficulty int
	var browserChallengeValidityHours int
	var browserChallengeIPBound int
	var refererValidationEnabled int
	var tarpitEnabled int
	var tarpitMaxDelay int
	var timingAnalysisEnabled int
	var timingVarianceThreshold float64
	var tlsFingerprintEnabled int
	var tlsFingerprintStrict int
	var behavioralAnalysisEnabled int
	var behavioralScoreThreshold int
	var headerAnalysisEnabled int
	var headerAnalysisThreshold int
	var headerAnalysisStrict int
	var honeypotEnabled int
	var honeypotAutoBlock int
	var honeypotAutoBan int
	var honeypotBlockDuration int
	var imageTokenValidityMinutes int
	var premiumEarlyAccessDuration int
	var maxPremiumChapters int
	var premiumCooldownScalingEnabled int
	var maintenanceEnabled int
	var maintenanceMessage string
	var newBadgeDuration int
	var parallelIndexingEnabled int
	var parallelIndexingThreshold int
	var discordInviteLink string

	if err := row.Scan(&allowInt, &maxUsers, &contentRatingLimit, &metadataProvider, &malClientID, &malClientSecret, &anilistApiToken, &imageAccessSecret,
		&stripeEnabled, &stripePublishableKey, &stripeSecretKey, &stripeWebhookSecret,
		&rateLimitEnabled, &rateLimitRequests, &rateLimitWindow, &rateLimitBlockDuration, &botDetectionEnabled, &botSeriesThreshold, &botChapterThreshold, &botDetectionWindow,
		&browserChallengeEnabled, &browserChallengeDifficulty, &browserChallengeValidityHours, &browserChallengeIPBound, &refererValidationEnabled,
		&tarpitEnabled, &tarpitMaxDelay, &timingAnalysisEnabled, &timingVarianceThreshold, &tlsFingerprintEnabled, &tlsFingerprintStrict, &behavioralAnalysisEnabled, &behavioralScoreThreshold, &headerAnalysisEnabled, &headerAnalysisThreshold, &headerAnalysisStrict, &honeypotEnabled, &honeypotAutoBlock, &honeypotAutoBan, &honeypotBlockDuration,
		&imageTokenValidityMinutes, &premiumEarlyAccessDuration, &maxPremiumChapters, &premiumCooldownScalingEnabled, &maintenanceEnabled, &maintenanceMessage, &newBadgeDuration, &parallelIndexingEnabled, &parallelIndexingThreshold, &discordInviteLink); err != nil {
		if err == sql.ErrNoRows {
			// Fallback defaults if row missing.
			return AppConfig{
				AllowRegistration:             true,
				MaxUsers:                      0,
				ContentRatingLimit:            3,
				MetadataProvider:              "mangadex",
				MALClientID:                   "",
				MALClientSecret:               "",
				AniListApiToken:               "",
				ImageAccessSecret:             "",
				StripeEnabled:                 false,
				StripePublishableKey:          "",
				StripeSecretKey:               "",
				StripeWebhookSecret:           "",
				RateLimitEnabled:              true,
				RateLimitRequests:             100,
				RateLimitWindow:               60,
				RateLimitBlockDuration:        300,
				BotDetectionEnabled:           true,
				BotSeriesThreshold:            5,
				BotChapterThreshold:           10,
				BotDetectionWindow:            60,
				BrowserChallengeEnabled:       false,
				BrowserChallengeDifficulty:    3,
				BrowserChallengeValidityHours: 24,
				BrowserChallengeIPBound:       false,
				RefererValidationEnabled:      false,
				TarpitEnabled:                 false,
				TarpitMaxDelay:                5000,
				TimingAnalysisEnabled:         false,
				TimingVarianceThreshold:       0.1,
				TLSFingerprintEnabled:         false,
				TLSFingerprintStrict:          false,
				BehavioralAnalysisEnabled:     false,
				BehavioralScoreThreshold:      40,
				HeaderAnalysisEnabled:         false,
				HeaderAnalysisThreshold:       5,
				HeaderAnalysisStrict:          false,
				HoneypotEnabled:               false,
				HoneypotAutoBlock:             true,
				HoneypotAutoBan:               false,
				HoneypotBlockDuration:         60,
				ImageTokenValidityMinutes:     5,
				PremiumEarlyAccessDuration:    3600,
				MaxPremiumChapters:            3,
				PremiumCooldownScalingEnabled: false,
				MaintenanceEnabled:            false,
				MaintenanceMessage:            "We are currently performing maintenance. Please check back later.",
				NewBadgeDuration:              48,
				ParallelIndexingEnabled:       true,
				ParallelIndexingThreshold:     100,
				DiscordInviteLink:             "",
			}, nil
		}
		return AppConfig{}, err
	}

	return AppConfig{
		AllowRegistration:             allowInt == 1,
		MaxUsers:                      maxUsers,
		ContentRatingLimit:            contentRatingLimit,
		MetadataProvider:              metadataProvider,
		MALClientID:                   malClientID,
		MALClientSecret:               malClientSecret,
		AniListApiToken:               anilistApiToken,
		ImageAccessSecret:             imageAccessSecret,
		StripeEnabled:                 stripeEnabled == 1,
		StripePublishableKey:          stripePublishableKey,
		StripeSecretKey:               stripeSecretKey,
		StripeWebhookSecret:           stripeWebhookSecret,
		RateLimitEnabled:              rateLimitEnabled == 1,
		RateLimitRequests:             rateLimitRequests,
		RateLimitWindow:               rateLimitWindow,
		RateLimitBlockDuration:        rateLimitBlockDuration,
		BotDetectionEnabled:           botDetectionEnabled == 1,
		BotSeriesThreshold:            botSeriesThreshold,
		BotChapterThreshold:           botChapterThreshold,
		BotDetectionWindow:            botDetectionWindow,
		BrowserChallengeEnabled:       browserChallengeEnabled == 1,
		BrowserChallengeDifficulty:    browserChallengeDifficulty,
		BrowserChallengeValidityHours: browserChallengeValidityHours,
		BrowserChallengeIPBound:       browserChallengeIPBound == 1,
		RefererValidationEnabled:      refererValidationEnabled == 1,
		TarpitEnabled:                 tarpitEnabled == 1,
		TarpitMaxDelay:                tarpitMaxDelay,
		TimingAnalysisEnabled:         timingAnalysisEnabled == 1,
		TimingVarianceThreshold:       timingVarianceThreshold,
		TLSFingerprintEnabled:         tlsFingerprintEnabled == 1,
		TLSFingerprintStrict:          tlsFingerprintStrict == 1,
		BehavioralAnalysisEnabled:     behavioralAnalysisEnabled == 1,
		BehavioralScoreThreshold:      behavioralScoreThreshold,
		HeaderAnalysisEnabled:         headerAnalysisEnabled == 1,
		HeaderAnalysisThreshold:       headerAnalysisThreshold,
		HeaderAnalysisStrict:          headerAnalysisStrict == 1,
		HoneypotEnabled:               honeypotEnabled == 1,
		HoneypotAutoBlock:             honeypotAutoBlock == 1,
		HoneypotAutoBan:               honeypotAutoBan == 1,
		HoneypotBlockDuration:         honeypotBlockDuration,
		ImageTokenValidityMinutes:     imageTokenValidityMinutes,
		PremiumEarlyAccessDuration:    premiumEarlyAccessDuration,
		MaxPremiumChapters:            maxPremiumChapters,
		PremiumCooldownScalingEnabled: premiumCooldownScalingEnabled == 1,
		MaintenanceEnabled:            maintenanceEnabled == 1,
		MaintenanceMessage:            maintenanceMessage,
		NewBadgeDuration:              newBadgeDuration,
		ParallelIndexingEnabled:       parallelIndexingEnabled == 1,
		ParallelIndexingThreshold:     parallelIndexingThreshold,
		DiscordInviteLink:             discordInviteLink,
	}, nil
}

// GetAppConfig returns the cached configuration with TTL-based refresh
// Cache is valid for 5 minutes to balance freshness with performance (reduces lock contention)
func GetAppConfig() (AppConfig, error) {
	// Check if cache is still valid
	if !configCacheTime.IsZero() && time.Since(configCacheTime) < configCacheTTL {
		if cfg := configCache.Load(); cfg != nil {
			return cfg.(AppConfig), nil
		}
	}

	// Cache expired or not yet loaded - refresh it
	cfg, err := loadConfigFromDB()
	if err != nil {
		return AppConfig{}, err
	}

	configCache.Store(cfg)
	configCacheTime = time.Now()

	return cfg, nil
}

// RefreshAppConfig forces a reload from the database (used after updates).
func RefreshAppConfig() (AppConfig, error) {
	cfg, err := loadConfigFromDB()
	if err != nil {
		return AppConfig{}, err
	}
	configCache.Store(cfg)
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
func UpdateRateLimitConfig(enabled bool, requests, window, blockDuration int) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE app_config SET rate_limit_enabled = ?, rate_limit_requests = ?, rate_limit_window = ?, rate_limit_block_duration = ? WHERE id = 1`,
		enabledInt, requests, window, blockDuration)
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
func UpdateMetadataConfig(provider, malClientID, malClientSecret, anilistToken string) (AppConfig, error) {
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

	_, err := db.Exec(`UPDATE app_config SET metadata_provider = ?, mal_client_id = ?, mal_client_secret = ?, anilist_api_token = ? WHERE id = 1`,
		provider, malClientID, malClientSecret, anilistToken)
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

// UpdateBrowserChallengeConfig updates the browser challenge configuration
func UpdateBrowserChallengeConfig(enabled bool, difficulty, validityHours int, ipBound bool) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	ipBoundInt := 0
	if ipBound {
		ipBoundInt = 1
	}
	// Ensure difficulty is within reasonable range (1-6)
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 6 {
		difficulty = 6
	}
	// Ensure validity hours is within reasonable range (1-168 = 1 week)
	if validityHours < 1 {
		validityHours = 1
	}
	if validityHours > 168 {
		validityHours = 168
	}
	_, err := db.Exec(`UPDATE app_config SET browser_challenge_enabled = ?, browser_challenge_difficulty = ?, browser_challenge_validity_hours = ?, browser_challenge_ip_bound = ? WHERE id = 1`,
		enabledInt, difficulty, validityHours, ipBoundInt)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateRefererValidationConfig updates the referer validation configuration
func UpdateRefererValidationConfig(enabled bool) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE app_config SET referer_validation_enabled = ? WHERE id = 1`, enabledInt)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateTarpitConfig updates the tarpit configuration
func UpdateTarpitConfig(enabled bool, maxDelay int) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	// Ensure max delay is within reasonable range (100ms - 30 seconds)
	if maxDelay < 100 {
		maxDelay = 100
	}
	if maxDelay > 30000 {
		maxDelay = 30000
	}
	_, err := db.Exec(`UPDATE app_config SET tarpit_enabled = ?, tarpit_max_delay = ? WHERE id = 1`, enabledInt, maxDelay)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateTimingAnalysisConfig updates the timing analysis configuration
func UpdateTimingAnalysisConfig(enabled bool, varianceThreshold float64) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	// Ensure threshold is within reasonable range (0.01 - 1.0)
	if varianceThreshold < 0.01 {
		varianceThreshold = 0.01
	}
	if varianceThreshold > 1.0 {
		varianceThreshold = 1.0
	}
	_, err := db.Exec(`UPDATE app_config SET timing_analysis_enabled = ?, timing_variance_threshold = ? WHERE id = 1`, enabledInt, varianceThreshold)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateTLSFingerprintConfig updates the TLS fingerprint configuration
func UpdateTLSFingerprintConfig(enabled, strict bool) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	strictInt := 0
	if strict {
		strictInt = 1
	}
	_, err := db.Exec(`UPDATE app_config SET tls_fingerprint_enabled = ?, tls_fingerprint_strict = ? WHERE id = 1`, enabledInt, strictInt)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateBehavioralAnalysisConfig updates the behavioral analysis configuration
func UpdateBehavioralAnalysisConfig(enabled bool, scoreThreshold int) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	// Ensure threshold is within reasonable range (0-100)
	if scoreThreshold < 0 {
		scoreThreshold = 0
	}
	if scoreThreshold > 100 {
		scoreThreshold = 100
	}
	_, err := db.Exec(`UPDATE app_config SET behavioral_analysis_enabled = ?, behavioral_score_threshold = ? WHERE id = 1`, enabledInt, scoreThreshold)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateHeaderAnalysisConfig updates the header analysis configuration
func UpdateHeaderAnalysisConfig(enabled bool, threshold int, strict bool) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	strictInt := 0
	if strict {
		strictInt = 1
	}
	// Ensure threshold is within reasonable range (1-20)
	if threshold < 1 {
		threshold = 1
	}
	if threshold > 20 {
		threshold = 20
	}
	_, err := db.Exec(`UPDATE app_config SET header_analysis_enabled = ?, header_analysis_threshold = ?, header_analysis_strict = ? WHERE id = 1`, enabledInt, threshold, strictInt)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// UpdateHoneypotConfig updates the honeypot configuration
func UpdateHoneypotConfig(enabled, autoBlock, autoBan bool, blockDuration int) (AppConfig, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	autoBlockInt := 0
	if autoBlock {
		autoBlockInt = 1
	}
	autoBanInt := 0
	if autoBan {
		autoBanInt = 1
	}
	// Ensure block duration is within reasonable range (1-1440 minutes = 1 min to 24 hours)
	if blockDuration < 1 {
		blockDuration = 1
	}
	if blockDuration > 1440 {
		blockDuration = 1440
	}
	_, err := db.Exec(`UPDATE app_config SET honeypot_enabled = ?, honeypot_auto_block = ?, honeypot_auto_ban = ?, honeypot_block_duration = ? WHERE id = 1`, enabledInt, autoBlockInt, autoBanInt, blockDuration)
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

func UpdateParallelIndexingConfig(enabled bool, threshold int) (AppConfig, error) {
	// Ensure threshold is at least 1
	if threshold < 1 {
		threshold = 1
	}
	_, err := db.Exec(`UPDATE app_config SET parallel_indexing_enabled = ?, parallel_indexing_threshold = ? WHERE id = 1`, enabled, threshold)
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

// GetImageTokenValidityMinutes returns the configured validity time for image tokens in minutes
func GetImageTokenValidityMinutes() int {
	cfg, err := GetAppConfig()
	if err != nil {
		// Return default if config can't be loaded
		return 60
	}
	return cfg.ImageTokenValidityMinutes
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
	configCacheTime = time.Time{}

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

// UpdateDiscordInviteLinkConfig updates the Discord invite link configuration
func UpdateDiscordInviteLinkConfig(inviteLink string) (AppConfig, error) {
	_, err := db.Exec(`UPDATE app_config SET discord_invite_link = ? WHERE id = 1`, inviteLink)
	if err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// GetMaintenanceStatus returns whether maintenance mode is active and the message
func GetMaintenanceStatus() (enabled bool, message string, err error) {
	cfg, err := GetAppConfig()
	if err != nil {
		return false, "", err
	}
	return cfg.MaintenanceEnabled, cfg.MaintenanceMessage, nil
}
