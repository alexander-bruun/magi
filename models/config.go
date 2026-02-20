package models

import (
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// AppConfig – global application settings (single-row table app_config id=1)
// ---------------------------------------------------------------------------

type AppConfig struct {
	// General
	AllowRegistration  bool   `json:"allow_registration" form:"allow_registration"`
	MaxUsers           int64  `json:"max_users" form:"max_users"`                       // 0 means unlimited
	ContentRatingLimit int    `json:"content_rating_limit" form:"content_rating_limit"` // 0=safe, 1=suggestive, 2=erotica, 3=pornographic
	MetadataProvider   string `json:"metadata_provider" form:"metadata_provider"`       // mangadex, anilist, jikan, mangaupdates, kitsu
	ImageAccessSecret  string `json:"image_access_secret" form:"image_access_secret"`

	// Stripe
	StripeEnabled        bool   `json:"stripe_enabled" form:"stripe_enabled"`
	StripePublishableKey string `json:"stripe_publishable_key" form:"stripe_publishable_key"`
	StripeSecretKey      string `json:"stripe_secret_key" form:"stripe_secret_key"`
	StripeWebhookSecret  string `json:"stripe_webhook_secret" form:"stripe_webhook_secret"`

	// Rate limiting
	RateLimitEnabled       bool `json:"rate_limit_enabled" form:"rate_limit_enabled"`
	RateLimitRequests      int  `json:"rate_limit_requests" form:"rate_limit_requests"`             // per window
	RateLimitWindow        int  `json:"rate_limit_window" form:"rate_limit_window"`                 // seconds
	RateLimitBlockDuration int  `json:"rate_limit_block_duration" form:"rate_limit_block_duration"` // seconds

	// Bot detection
	BotDetectionEnabled bool `json:"bot_detection_enabled" form:"bot_detection_enabled"`
	BotSeriesThreshold  int  `json:"bot_series_threshold" form:"bot_series_threshold"`
	BotChapterThreshold int  `json:"bot_chapter_threshold" form:"bot_chapter_threshold"`
	BotDetectionWindow  int  `json:"bot_detection_window" form:"bot_detection_window"` // seconds
	BotBanDuration      int  `json:"bot_ban_duration" form:"bot_ban_duration"`         // seconds (0 = permanent)

	// Browser challenge
	BrowserChallengeEnabled       bool `json:"browser_challenge_enabled" form:"browser_challenge_enabled"`
	BrowserChallengeDifficulty    int  `json:"browser_challenge_difficulty" form:"browser_challenge_difficulty"`
	BrowserChallengeValidityHours int  `json:"browser_challenge_validity_hours" form:"browser_challenge_validity_hours"`
	BrowserChallengeIPBound       bool `json:"browser_challenge_ip_bound" form:"browser_challenge_ip_bound"`

	// Referer validation
	RefererValidationEnabled bool `json:"referer_validation_enabled" form:"referer_validation_enabled"`

	// Tarpit
	TarpitEnabled  bool `json:"tarpit_enabled" form:"tarpit_enabled"`
	TarpitMaxDelay int  `json:"tarpit_max_delay" form:"tarpit_max_delay"` // milliseconds

	// Timing analysis
	TimingAnalysisEnabled   bool    `json:"timing_analysis_enabled" form:"timing_analysis_enabled"`
	TimingVarianceThreshold float64 `json:"timing_variance_threshold" form:"timing_variance_threshold"`

	// TLS fingerprint
	TLSFingerprintEnabled bool `json:"tls_fingerprint_enabled" form:"tls_fingerprint_enabled"`
	TLSFingerprintStrict  bool `json:"tls_fingerprint_strict" form:"tls_fingerprint_strict"`

	// Behavioral analysis
	BehavioralAnalysisEnabled bool `json:"behavioral_analysis_enabled" form:"behavioral_analysis_enabled"`
	BehavioralScoreThreshold  int  `json:"behavioral_score_threshold" form:"behavioral_score_threshold"`

	// Header analysis
	HeaderAnalysisEnabled   bool `json:"header_analysis_enabled" form:"header_analysis_enabled"`
	HeaderAnalysisThreshold int  `json:"header_analysis_threshold" form:"header_analysis_threshold"`
	HeaderAnalysisStrict    bool `json:"header_analysis_strict" form:"header_analysis_strict"`

	// Honeypot
	HoneypotEnabled       bool `json:"honeypot_enabled" form:"honeypot_enabled"`
	HoneypotAutoBlock     bool `json:"honeypot_auto_block" form:"honeypot_auto_block"`
	HoneypotAutoBan       bool `json:"honeypot_auto_ban" form:"honeypot_auto_ban"`
	HoneypotBlockDuration int  `json:"honeypot_block_duration" form:"honeypot_block_duration"` // minutes

	// Premium
	PremiumEarlyAccessDuration    int  `json:"premium_early_access_duration" form:"premium_early_access_duration"` // seconds
	MaxPremiumChapters            int  `json:"max_premium_chapters" form:"max_premium_chapters"`
	PremiumCooldownScalingEnabled bool `json:"premium_cooldown_scaling_enabled" form:"premium_cooldown_scaling_enabled"`

	// Maintenance
	MaintenanceEnabled bool   `json:"maintenance_enabled" form:"maintenance_enabled"`
	MaintenanceMessage string `json:"maintenance_message" form:"maintenance_message"`

	// New badge
	NewBadgeDuration int `json:"new_badge_duration" form:"new_badge_duration"` // hours

	// Parallel indexing
	ParallelIndexingEnabled   bool `json:"parallel_indexing_enabled" form:"parallel_indexing_enabled"`
	ParallelIndexingThreshold int  `json:"parallel_indexing_threshold" form:"parallel_indexing_threshold"`

	// Discord
	DiscordInviteLink string `json:"discord_invite_link" form:"discord_invite_link"`

	// Metadata embedding
	MetadataEmbeddingEnabled bool `json:"metadata_embedding_enabled" form:"metadata_embedding_enabled"`
}

// metadata.ConfigProvider interface
func (c *AppConfig) GetMetadataProvider() string { return c.MetadataProvider }
func (c *AppConfig) GetContentRatingLimit() int  { return c.ContentRatingLimit }

// ---------------------------------------------------------------------------
// Column registry – single source of truth for DB column names & defaults.
// Order must match configRow.scanArgs() and configWriteArgs().
// ---------------------------------------------------------------------------

type colDef struct{ name, def string }

var configColumns = []colDef{
	{"allow_registration", "0"},
	{"max_users", "0"},
	{"content_rating_limit", "0"},
	{"metadata_provider", "'mangadex'"},
	{"image_access_secret", "''"},
	{"stripe_enabled", "0"},
	{"stripe_publishable_key", "''"},
	{"stripe_secret_key", "''"},
	{"stripe_webhook_secret", "''"},
	{"rate_limit_enabled", "1"},
	{"rate_limit_requests", "100"},
	{"rate_limit_window", "60"},
	{"rate_limit_block_duration", "300"},
	{"bot_detection_enabled", "1"},
	{"bot_series_threshold", "5"},
	{"bot_chapter_threshold", "10"},
	{"bot_detection_window", "60"},
	{"bot_ban_duration", "300"},
	{"browser_challenge_enabled", "0"},
	{"browser_challenge_difficulty", "3"},
	{"browser_challenge_validity_hours", "24"},
	{"browser_challenge_ip_bound", "0"},
	{"referer_validation_enabled", "0"},
	{"tarpit_enabled", "0"},
	{"tarpit_max_delay", "5000"},
	{"timing_analysis_enabled", "0"},
	{"timing_variance_threshold", "0.1"},
	{"tls_fingerprint_enabled", "0"},
	{"tls_fingerprint_strict", "0"},
	{"behavioral_analysis_enabled", "0"},
	{"behavioral_score_threshold", "40"},
	{"header_analysis_enabled", "0"},
	{"header_analysis_threshold", "5"},
	{"header_analysis_strict", "0"},
	{"honeypot_enabled", "0"},
	{"honeypot_auto_block", "1"},
	{"honeypot_auto_ban", "0"},
	{"honeypot_block_duration", "60"},
	{"premium_early_access_duration", "3600"},
	{"max_premium_chapters", "3"},
	{"premium_cooldown_scaling_enabled", "0"},
	{"maintenance_enabled", "0"},
	{"maintenance_message", "'We are currently performing maintenance. Please check back later.'"},
	{"new_badge_duration", "48"},
	{"parallel_indexing_enabled", "1"},
	{"parallel_indexing_threshold", "100"},
	{"discord_invite_link", "''"},
	{"metadata_embedding_enabled", "0"},
}

// configSelectSQL and configUpdateSQL are built once at init from configColumns.
var (
	configSelectSQL string
	configUpdateSQL string
)

func init() {
	sel := make([]string, len(configColumns))
	upd := make([]string, len(configColumns))
	for i, c := range configColumns {
		sel[i] = fmt.Sprintf("COALESCE(%s, %s)", c.name, c.def)
		upd[i] = c.name + " = ?"
	}
	configSelectSQL = "SELECT " + strings.Join(sel, ", ") + " FROM app_config WHERE id = 1"
	configUpdateSQL = "UPDATE app_config SET " + strings.Join(upd, ", ") + " WHERE id = 1"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// defaultConfig returns an AppConfig with safe defaults (used when the DB row is missing).
func defaultConfig() AppConfig {
	return AppConfig{
		AllowRegistration:             true,
		MaxUsers:                      0,
		ContentRatingLimit:            3,
		MetadataProvider:              "mangadex",
		RateLimitEnabled:              true,
		RateLimitRequests:             100,
		RateLimitWindow:               60,
		RateLimitBlockDuration:        300,
		BotDetectionEnabled:           true,
		BotSeriesThreshold:            5,
		BotChapterThreshold:           10,
		BotDetectionWindow:            60,
		BotBanDuration:                300,
		BrowserChallengeDifficulty:    3,
		BrowserChallengeValidityHours: 24,
		TarpitMaxDelay:                5000,
		TimingVarianceThreshold:       0.1,
		BehavioralScoreThreshold:      40,
		HeaderAnalysisThreshold:       5,
		HoneypotAutoBlock:             true,
		HoneypotBlockDuration:         60,
		PremiumEarlyAccessDuration:    3600,
		MaxPremiumChapters:            3,
		MaintenanceMessage:            "We are currently performing maintenance. Please check back later.",
		NewBadgeDuration:              48,
		ParallelIndexingEnabled:       true,
		ParallelIndexingThreshold:     100,
	}
}

// Validate clamps all fields to valid ranges, applying defaults where needed.
func (c *AppConfig) Validate() {
	c.ContentRatingLimit = clampInt(c.ContentRatingLimit, 0, 3)
	if c.MaxUsers < 0 {
		c.MaxUsers = 0
	}

	switch c.MetadataProvider {
	case "mangadex", "anilist", "jikan", "mangaupdates", "kitsu":
	default:
		c.MetadataProvider = "mangadex"
	}

	if c.MaintenanceMessage == "" {
		c.MaintenanceMessage = "We are currently performing maintenance. Please check back later."
	}
	if c.RateLimitRequests <= 0 {
		c.RateLimitRequests = 100
	}
	if c.RateLimitWindow <= 0 {
		c.RateLimitWindow = 60
	}
	if c.RateLimitBlockDuration <= 0 {
		c.RateLimitBlockDuration = 300
	}
	if c.BotSeriesThreshold <= 0 {
		c.BotSeriesThreshold = 5
	}
	if c.BotChapterThreshold <= 0 {
		c.BotChapterThreshold = 10
	}
	if c.BotDetectionWindow <= 0 {
		c.BotDetectionWindow = 60
	}
	if c.BotBanDuration <= 0 {
		c.BotBanDuration = 300
	}

	c.BrowserChallengeDifficulty = clampInt(c.BrowserChallengeDifficulty, 1, 6)
	c.BrowserChallengeValidityHours = clampInt(c.BrowserChallengeValidityHours, 1, 168)
	c.TarpitMaxDelay = clampInt(c.TarpitMaxDelay, 100, 30000)

	if c.TimingVarianceThreshold < 0.01 {
		c.TimingVarianceThreshold = 0.01
	}
	if c.TimingVarianceThreshold > 1.0 {
		c.TimingVarianceThreshold = 1.0
	}

	c.BehavioralScoreThreshold = clampInt(c.BehavioralScoreThreshold, 0, 100)
	c.HeaderAnalysisThreshold = clampInt(c.HeaderAnalysisThreshold, 1, 20)
	c.HoneypotBlockDuration = clampInt(c.HoneypotBlockDuration, 1, 1440)

	if c.PremiumEarlyAccessDuration < 0 {
		c.PremiumEarlyAccessDuration = 0
	}
	if c.MaxPremiumChapters < 0 {
		c.MaxPremiumChapters = 0
	}
	if c.NewBadgeDuration < 1 {
		c.NewBadgeDuration = 48
	}
	if c.ParallelIndexingThreshold < 1 {
		c.ParallelIndexingThreshold = 100
	}
}

// ---------------------------------------------------------------------------
// configRow – intermediate scan target. SQLite stores bools as INTEGER 0/1.
// Order of fields must match configColumns.
// ---------------------------------------------------------------------------

type configRow struct {
	allowRegistration, stripeEnabled, rateLimitEnabled, botDetectionEnabled    int
	browserChallengeEnabled, browserChallengeIPBound, refererValidationEnabled int
	tarpitEnabled, timingAnalysisEnabled, tlsFingerprintEnabled                int
	tlsFingerprintStrict, behavioralAnalysisEnabled, headerAnalysisEnabled     int
	headerAnalysisStrict, honeypotEnabled, honeypotAutoBlock, honeypotAutoBan  int
	premiumCooldownScalingEnabled, maintenanceEnabled, parallelIndexingEnabled int
	metadataEmbeddingEnabled                                                   int

	maxUsers                                                  int64
	contentRatingLimit, rateLimitRequests, rateLimitWindow    int
	rateLimitBlockDuration, botSeriesThreshold                int
	botChapterThreshold, botDetectionWindow, botBanDuration   int
	browserChallengeDifficulty, browserChallengeValidityHours int
	tarpitMaxDelay, behavioralScoreThreshold                  int
	headerAnalysisThreshold, honeypotBlockDuration            int
	premiumEarlyAccessDuration, maxPremiumChapters            int
	newBadgeDuration, parallelIndexingThreshold               int

	timingVarianceThreshold float64

	metadataProvider, imageAccessSecret                        string
	stripePublishableKey, stripeSecretKey, stripeWebhookSecret string
	maintenanceMessage, discordInviteLink                      string
}

// scanArgs returns pointers in configColumns order for row.Scan().
func (r *configRow) scanArgs() []any {
	return []any{
		&r.allowRegistration, &r.maxUsers, &r.contentRatingLimit,
		&r.metadataProvider, &r.imageAccessSecret,
		&r.stripeEnabled, &r.stripePublishableKey, &r.stripeSecretKey, &r.stripeWebhookSecret,
		&r.rateLimitEnabled, &r.rateLimitRequests, &r.rateLimitWindow, &r.rateLimitBlockDuration,
		&r.botDetectionEnabled, &r.botSeriesThreshold, &r.botChapterThreshold, &r.botDetectionWindow, &r.botBanDuration,
		&r.browserChallengeEnabled, &r.browserChallengeDifficulty, &r.browserChallengeValidityHours, &r.browserChallengeIPBound,
		&r.refererValidationEnabled,
		&r.tarpitEnabled, &r.tarpitMaxDelay,
		&r.timingAnalysisEnabled, &r.timingVarianceThreshold,
		&r.tlsFingerprintEnabled, &r.tlsFingerprintStrict,
		&r.behavioralAnalysisEnabled, &r.behavioralScoreThreshold,
		&r.headerAnalysisEnabled, &r.headerAnalysisThreshold, &r.headerAnalysisStrict,
		&r.honeypotEnabled, &r.honeypotAutoBlock, &r.honeypotAutoBan, &r.honeypotBlockDuration,
		&r.premiumEarlyAccessDuration, &r.maxPremiumChapters, &r.premiumCooldownScalingEnabled,
		&r.maintenanceEnabled, &r.maintenanceMessage,
		&r.newBadgeDuration,
		&r.parallelIndexingEnabled, &r.parallelIndexingThreshold,
		&r.discordInviteLink, &r.metadataEmbeddingEnabled,
	}
}

func (r *configRow) toAppConfig() AppConfig {
	b := func(v int) bool { return v == 1 }
	return AppConfig{
		AllowRegistration:             b(r.allowRegistration),
		MaxUsers:                      r.maxUsers,
		ContentRatingLimit:            r.contentRatingLimit,
		MetadataProvider:              r.metadataProvider,
		ImageAccessSecret:             r.imageAccessSecret,
		StripeEnabled:                 b(r.stripeEnabled),
		StripePublishableKey:          r.stripePublishableKey,
		StripeSecretKey:               r.stripeSecretKey,
		StripeWebhookSecret:           r.stripeWebhookSecret,
		RateLimitEnabled:              b(r.rateLimitEnabled),
		RateLimitRequests:             r.rateLimitRequests,
		RateLimitWindow:               r.rateLimitWindow,
		RateLimitBlockDuration:        r.rateLimitBlockDuration,
		BotDetectionEnabled:           b(r.botDetectionEnabled),
		BotSeriesThreshold:            r.botSeriesThreshold,
		BotChapterThreshold:           r.botChapterThreshold,
		BotDetectionWindow:            r.botDetectionWindow,
		BotBanDuration:                r.botBanDuration,
		BrowserChallengeEnabled:       b(r.browserChallengeEnabled),
		BrowserChallengeDifficulty:    r.browserChallengeDifficulty,
		BrowserChallengeValidityHours: r.browserChallengeValidityHours,
		BrowserChallengeIPBound:       b(r.browserChallengeIPBound),
		RefererValidationEnabled:      b(r.refererValidationEnabled),
		TarpitEnabled:                 b(r.tarpitEnabled),
		TarpitMaxDelay:                r.tarpitMaxDelay,
		TimingAnalysisEnabled:         b(r.timingAnalysisEnabled),
		TimingVarianceThreshold:       r.timingVarianceThreshold,
		TLSFingerprintEnabled:         b(r.tlsFingerprintEnabled),
		TLSFingerprintStrict:          b(r.tlsFingerprintStrict),
		BehavioralAnalysisEnabled:     b(r.behavioralAnalysisEnabled),
		BehavioralScoreThreshold:      r.behavioralScoreThreshold,
		HeaderAnalysisEnabled:         b(r.headerAnalysisEnabled),
		HeaderAnalysisThreshold:       r.headerAnalysisThreshold,
		HeaderAnalysisStrict:          b(r.headerAnalysisStrict),
		HoneypotEnabled:               b(r.honeypotEnabled),
		HoneypotAutoBlock:             b(r.honeypotAutoBlock),
		HoneypotAutoBan:               b(r.honeypotAutoBan),
		HoneypotBlockDuration:         r.honeypotBlockDuration,
		PremiumEarlyAccessDuration:    r.premiumEarlyAccessDuration,
		MaxPremiumChapters:            r.maxPremiumChapters,
		PremiumCooldownScalingEnabled: b(r.premiumCooldownScalingEnabled),
		MaintenanceEnabled:            b(r.maintenanceEnabled),
		MaintenanceMessage:            r.maintenanceMessage,
		NewBadgeDuration:              r.newBadgeDuration,
		ParallelIndexingEnabled:       b(r.parallelIndexingEnabled),
		ParallelIndexingThreshold:     r.parallelIndexingThreshold,
		DiscordInviteLink:             r.discordInviteLink,
		MetadataEmbeddingEnabled:      b(r.metadataEmbeddingEnabled),
	}
}

// configWriteArgs converts an AppConfig into ordered values matching configColumns.
func configWriteArgs(cfg AppConfig) []any {
	b := boolToInt
	return []any{
		b(cfg.AllowRegistration), cfg.MaxUsers, cfg.ContentRatingLimit,
		cfg.MetadataProvider, cfg.ImageAccessSecret,
		b(cfg.StripeEnabled), cfg.StripePublishableKey, cfg.StripeSecretKey, cfg.StripeWebhookSecret,
		b(cfg.RateLimitEnabled), cfg.RateLimitRequests, cfg.RateLimitWindow, cfg.RateLimitBlockDuration,
		b(cfg.BotDetectionEnabled), cfg.BotSeriesThreshold, cfg.BotChapterThreshold, cfg.BotDetectionWindow, cfg.BotBanDuration,
		b(cfg.BrowserChallengeEnabled), cfg.BrowserChallengeDifficulty, cfg.BrowserChallengeValidityHours, b(cfg.BrowserChallengeIPBound),
		b(cfg.RefererValidationEnabled),
		b(cfg.TarpitEnabled), cfg.TarpitMaxDelay,
		b(cfg.TimingAnalysisEnabled), cfg.TimingVarianceThreshold,
		b(cfg.TLSFingerprintEnabled), b(cfg.TLSFingerprintStrict),
		b(cfg.BehavioralAnalysisEnabled), cfg.BehavioralScoreThreshold,
		b(cfg.HeaderAnalysisEnabled), cfg.HeaderAnalysisThreshold, b(cfg.HeaderAnalysisStrict),
		b(cfg.HoneypotEnabled), b(cfg.HoneypotAutoBlock), b(cfg.HoneypotAutoBan), cfg.HoneypotBlockDuration,
		cfg.PremiumEarlyAccessDuration, cfg.MaxPremiumChapters, b(cfg.PremiumCooldownScalingEnabled),
		b(cfg.MaintenanceEnabled), cfg.MaintenanceMessage,
		cfg.NewBadgeDuration,
		b(cfg.ParallelIndexingEnabled), cfg.ParallelIndexingThreshold,
		cfg.DiscordInviteLink, b(cfg.MetadataEmbeddingEnabled),
	}
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

var (
	configCache     atomic.Value // stores AppConfig
	configCacheTime time.Time
	configCacheTTL  = 5 * time.Minute
)

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

func loadConfigFromDB() (AppConfig, error) {
	var r configRow
	if err := db.QueryRow(configSelectSQL).Scan(r.scanArgs()...); err != nil {
		if err == sql.ErrNoRows {
			return defaultConfig(), nil
		}
		return AppConfig{}, err
	}
	return r.toAppConfig(), nil
}

// GetAppConfig returns the cached configuration, refreshing if the TTL has expired.
func GetAppConfig() (AppConfig, error) {
	if !configCacheTime.IsZero() && time.Since(configCacheTime) < configCacheTTL {
		if cfg := configCache.Load(); cfg != nil {
			return cfg.(AppConfig), nil
		}
	}
	cfg, err := loadConfigFromDB()
	if err != nil {
		return AppConfig{}, err
	}
	configCache.Store(cfg)
	configCacheTime = time.Now()
	return cfg, nil
}

// RefreshAppConfig forces a reload from the database.
func RefreshAppConfig() (AppConfig, error) {
	cfg, err := loadConfigFromDB()
	if err != nil {
		return AppConfig{}, err
	}
	configCache.Store(cfg)
	configCacheTime = time.Now()
	return cfg, nil
}

func execAndRefresh(query string, args ...any) (AppConfig, error) {
	if _, err := db.Exec(query, args...); err != nil {
		return AppConfig{}, err
	}
	return RefreshAppConfig()
}

// ---------------------------------------------------------------------------
// Save / update
// ---------------------------------------------------------------------------

// SaveFullConfig persists all user-editable fields in a single query.
// Call cfg.Validate() first to ensure values are within valid ranges.
func SaveFullConfig(cfg AppConfig) (AppConfig, error) {
	return execAndRefresh(configUpdateSQL, configWriteArgs(cfg)...)
}

// UpdateMaintenanceConfig updates maintenance mode (used by CLI).
func UpdateMaintenanceConfig(enabled bool, message string) (AppConfig, error) {
	return execAndRefresh(
		"UPDATE app_config SET maintenance_enabled = ?, maintenance_message = ? WHERE id = 1",
		boolToInt(enabled), message,
	)
}

// UpdateImageAccessSecret updates the image access secret (used at startup).
func UpdateImageAccessSecret(secret string) (AppConfig, error) {
	return execAndRefresh(
		"UPDATE app_config SET image_access_secret = ? WHERE id = 1",
		secret,
	)
}

// ---------------------------------------------------------------------------
// Content rating utilities
// ---------------------------------------------------------------------------

// ContentRatingToInt converts a rating string to its integer level.
// 0=safe, 1=suggestive, 2=erotica, 3=pornographic
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
		return 3
	}
}

// IsContentRatingAllowed checks if a content rating is within the configured limit.
func IsContentRatingAllowed(rating string, limit int) bool {
	return ContentRatingToInt(rating) <= limit
}

// GetAllowedRatings returns the list of allowed content rating strings for a given limit.
func GetAllowedRatings(limit int) []string {
	all := []string{"safe", "suggestive", "erotica", "pornographic"}
	if limit < 0 || limit >= len(all) {
		return all
	}
	return all[:limit+1]
}

// AllowedRatingsPlaceholders returns the allowed ratings and a SQL placeholder string.
func AllowedRatingsPlaceholders(limit int) ([]string, string) {
	ratings := GetAllowedRatings(limit)
	placeholders := strings.Repeat("?,", len(ratings))
	return ratings, placeholders[:len(placeholders)-1]
}

// ---------------------------------------------------------------------------
// Convenience
// ---------------------------------------------------------------------------

// GetMaintenanceStatus returns whether maintenance mode is active and the message.
func GetMaintenanceStatus() (enabled bool, message string, err error) {
	cfg, err := GetAppConfig()
	if err != nil {
		return false, "", err
	}
	return cfg.MaintenanceEnabled, cfg.MaintenanceMessage, nil
}
