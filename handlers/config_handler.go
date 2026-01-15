package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/text"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	websocket "github.com/gofiber/websocket/v2"
)

// HandleConfiguration renders the configuration page.
func HandleConfiguration(c *fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrConfigLoadFailed, err)
	}
	return handleView(c, views.Config(cfg))
}

// HandleConfigurationUpdate processes updates to the global configuration.
func HandleConfigurationUpdate(c *fiber.Ctx) error {
	var config models.AppConfig
	if err := c.BodyParser(&config); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	// Process content_rating_limit
	contentRatingLimit := config.ContentRatingLimit
	if contentRatingLimit < 0 || contentRatingLimit > 3 {
		contentRatingLimit = 3 // default to show all
	}

	// Process max_users
	maxUsers := max(config.MaxUsers, 0)

	if _, err := models.UpdateAppConfig(config.AllowRegistration, maxUsers, contentRatingLimit); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update maintenance mode configuration
	maintenanceEnabled := config.MaintenanceEnabled
	maintenanceMessage := config.MaintenanceMessage
	if maintenanceMessage == "" {
		maintenanceMessage = "We are currently performing maintenance. Please check back later."
	}
	if _, err := models.UpdateMaintenanceConfig(maintenanceEnabled, maintenanceMessage); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update metadata provider configuration
	metadataProvider := config.MetadataProvider
	if metadataProvider == "" {
		metadataProvider = "mangadex"
	}
	if _, err := models.UpdateMetadataConfig(metadataProvider); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update Stripe configuration
	stripeEnabled := config.StripeEnabled
	stripePublishableKey := config.StripePublishableKey
	stripeSecretKey := config.StripeSecretKey
	stripeWebhookSecret := config.StripeWebhookSecret
	if _, err := models.UpdateStripeConfig(stripeEnabled, stripePublishableKey, stripeSecretKey, stripeWebhookSecret); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update rate limiting configuration
	rateLimitEnabled := config.RateLimitEnabled
	rateLimitRequests := config.RateLimitRequests
	if rateLimitRequests <= 0 {
		rateLimitRequests = 100
	}
	rateLimitWindow := config.RateLimitWindow
	if rateLimitWindow <= 0 {
		rateLimitWindow = 60
	}
	rateLimitBlockDuration := config.RateLimitBlockDuration
	if rateLimitBlockDuration <= 0 {
		rateLimitBlockDuration = 300
	}
	if _, err := models.UpdateRateLimitConfig(rateLimitEnabled, rateLimitRequests, rateLimitWindow, rateLimitBlockDuration); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update bot detection configuration
	botDetectionEnabled := config.BotDetectionEnabled
	botSeriesThreshold := config.BotSeriesThreshold
	if botSeriesThreshold <= 0 {
		botSeriesThreshold = 5
	}
	botChapterThreshold := config.BotChapterThreshold
	if botChapterThreshold <= 0 {
		botChapterThreshold = 10
	}
	botDetectionWindow := config.BotDetectionWindow
	if botDetectionWindow <= 0 {
		botDetectionWindow = 60
	}
	if _, err := models.UpdateBotDetectionConfig(botDetectionEnabled, botSeriesThreshold, botChapterThreshold, botDetectionWindow); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update browser challenge configuration
	browserChallengeEnabled := config.BrowserChallengeEnabled
	browserChallengeDifficulty := config.BrowserChallengeDifficulty
	if browserChallengeDifficulty < 1 || browserChallengeDifficulty > 6 {
		browserChallengeDifficulty = 3
	}
	browserChallengeValidityHours := config.BrowserChallengeValidityHours
	if browserChallengeValidityHours < 1 || browserChallengeValidityHours > 168 {
		browserChallengeValidityHours = 24
	}
	browserChallengeIPBound := config.BrowserChallengeIPBound
	if _, err := models.UpdateBrowserChallengeConfig(browserChallengeEnabled, browserChallengeDifficulty, browserChallengeValidityHours, browserChallengeIPBound); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update referer validation configuration
	if _, err := models.UpdateRefererValidationConfig(config.RefererValidationEnabled); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update tarpit configuration
	tarpitMaxDelay := min(max(config.TarpitMaxDelay, 100), 30000)
	if _, err := models.UpdateTarpitConfig(config.TarpitEnabled, tarpitMaxDelay); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update timing analysis configuration
	timingVarianceThreshold := config.TimingVarianceThreshold
	if timingVarianceThreshold < 0.01 {
		timingVarianceThreshold = 0.01
	}
	if timingVarianceThreshold > 1.0 {
		timingVarianceThreshold = 1.0
	}
	if _, err := models.UpdateTimingAnalysisConfig(config.TimingAnalysisEnabled, timingVarianceThreshold); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update TLS fingerprint configuration
	if _, err := models.UpdateTLSFingerprintConfig(config.TLSFingerprintEnabled, config.TLSFingerprintStrict); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update behavioral analysis configuration
	behavioralScoreThreshold := min(max(config.BehavioralScoreThreshold, 0), 100)
	if _, err := models.UpdateBehavioralAnalysisConfig(config.BehavioralAnalysisEnabled, behavioralScoreThreshold); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update header analysis configuration
	headerAnalysisThreshold := min(max(config.HeaderAnalysisThreshold, 1), 20)
	if _, err := models.UpdateHeaderAnalysisConfig(config.HeaderAnalysisEnabled, headerAnalysisThreshold, config.HeaderAnalysisStrict); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update honeypot configuration
	honeypotBlockDuration := min(max(config.HoneypotBlockDuration, 1), 1440)
	if _, err := models.UpdateHoneypotConfig(config.HoneypotEnabled, config.HoneypotAutoBlock, config.HoneypotAutoBan, honeypotBlockDuration); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update image token validity configuration
	imageTokenValidity := config.ImageTokenValidityMinutes
	if imageTokenValidity < 1 || imageTokenValidity > 60 {
		imageTokenValidity = 5
	}
	if _, err := models.UpdateImageTokenConfig(imageTokenValidity); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update premium early access configuration
	premiumEarlyAccess := config.PremiumEarlyAccessDuration
	if premiumEarlyAccess < 0 {
		premiumEarlyAccess = 3600
	}
	if _, err := models.UpdatePremiumEarlyAccessConfig(premiumEarlyAccess); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update max premium chapters configuration
	maxPremiumChapters := config.MaxPremiumChapters
	if maxPremiumChapters < 0 {
		maxPremiumChapters = 3
	}
	if _, err := models.UpdateMaxPremiumChaptersConfig(maxPremiumChapters); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update premium cooldown scaling configuration
	premiumCooldownScalingEnabled := config.PremiumCooldownScalingEnabled
	if _, err := models.UpdatePremiumCooldownScalingConfig(premiumCooldownScalingEnabled); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update NEW badge duration configuration
	newBadgeDuration := config.NewBadgeDuration
	if newBadgeDuration < 1 {
		newBadgeDuration = 48 // default to 48 hours
	}
	if _, err := models.UpdateNewBadgeDurationConfig(newBadgeDuration); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update parallel indexing configuration
	parallelIndexingEnabled := config.ParallelIndexingEnabled
	parallelIndexingThreshold := config.ParallelIndexingThreshold
	if parallelIndexingThreshold < 1 {
		parallelIndexingThreshold = 100 // default to 100 series
	}
	if _, err := models.UpdateParallelIndexingConfig(parallelIndexingEnabled, parallelIndexingThreshold); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	// Update Discord invite link configuration
	discordInviteLink := config.DiscordInviteLink
	if _, err := models.UpdateDiscordInviteLinkConfig(discordInviteLink); err != nil {
		return sendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	return handleView(c, views.ConfigForm())
}

// HandleConsoleLogsWebSocketUpgrade upgrades the connection to WebSocket for console logs
func HandleConsoleLogsWebSocketUpgrade(c *fiber.Ctx) error {
	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(c) {
		// Upgrade to WebSocket with authentication validation
		return websocket.New(func(conn *websocket.Conn) {
			// Verify user is authenticated as admin via Locals
			userName := conn.Locals("user_name")
			if userName == nil {
				conn.Close()
				return
			}

			// Additional role check - verify admin role
			user, err := models.FindUserByUsername(userName.(string))
			if err != nil || user == nil || user.Role != "admin" {
				conn.Close()
				return
			}

			// Authentication passed, handle WebSocket connection
			text.HandleConsoleLogsWebSocket(conn)
		})(c)
	}
	return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
}
