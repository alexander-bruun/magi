package handlers

import (
    "strconv"
    "github.com/alexander-bruun/magi/models"
    "github.com/alexander-bruun/magi/utils"
    "github.com/alexander-bruun/magi/views"
    fiber "github.com/gofiber/fiber/v2"
    websocket "github.com/gofiber/websocket/v2"
)

// HandleConfiguration renders the configuration page.
func HandleConfiguration(c *fiber.Ctx) error {
    return HandleView(c, views.Config())
}

// HandleConfigurationUpdate processes updates to the global configuration.
func HandleConfigurationUpdate(c *fiber.Ctx) error {
    // Checkbox only present when enabled
    allow := c.FormValue("allow_registration") == "on"
    contentRatingLimitStr := c.FormValue("content_rating_limit")
    var contentRatingLimit int
    if contentRatingLimitStr != "" {
        if v, err := strconv.Atoi(contentRatingLimitStr); err == nil && v >= 0 && v <= 3 {
            contentRatingLimit = v
        } else {
            contentRatingLimit = 3 // default to show all
        }
    } else {
        contentRatingLimit = 3 // default to show all
    }
    maxUsersStr := c.FormValue("max_users")
    var maxUsers int64
    if maxUsersStr != "" {
        if v, err := strconv.ParseInt(maxUsersStr, 10, 64); err == nil && v >= 0 {
            maxUsers = v
        }
    }
    if _, err := models.UpdateAppConfig(allow, maxUsers, contentRatingLimit); err != nil {
        return handleError(c, err)
    }
    
    // Update metadata provider configuration if provided
    metadataProvider := c.FormValue("metadata_provider")
    if metadataProvider != "" {
        malToken := c.FormValue("mal_api_token")
        anilistToken := c.FormValue("anilist_api_token")
        if _, err := models.UpdateMetadataConfig(metadataProvider, malToken, anilistToken); err != nil {
            return handleError(c, err)
        }
    }
    
    // Update rate limiting configuration
    rateLimitEnabled := c.FormValue("rate_limit_enabled") == "on"
    rateLimitRequestsStr := c.FormValue("rate_limit_requests")
    rateLimitWindowStr := c.FormValue("rate_limit_window")
    var rateLimitRequests, rateLimitWindow int
    if rateLimitRequestsStr != "" {
        if v, err := strconv.Atoi(rateLimitRequestsStr); err == nil && v > 0 {
            rateLimitRequests = v
        } else {
            rateLimitRequests = 100
        }
    } else {
        rateLimitRequests = 100
    }
    if rateLimitWindowStr != "" {
        if v, err := strconv.Atoi(rateLimitWindowStr); err == nil && v > 0 {
            rateLimitWindow = v
        } else {
            rateLimitWindow = 60
        }
    } else {
        rateLimitWindow = 60
    }
    if _, err := models.UpdateRateLimitConfig(rateLimitEnabled, rateLimitRequests, rateLimitWindow); err != nil {
        return handleError(c, err)
    }
    
    // Update bot detection configuration
    botDetectionEnabled := c.FormValue("bot_detection_enabled") == "on"
    botSeriesThresholdStr := c.FormValue("bot_series_threshold")
    botChapterThresholdStr := c.FormValue("bot_chapter_threshold")
    botDetectionWindowStr := c.FormValue("bot_detection_window")
    var botSeriesThreshold, botChapterThreshold, botDetectionWindow int
    if botSeriesThresholdStr != "" {
        if v, err := strconv.Atoi(botSeriesThresholdStr); err == nil && v > 0 {
            botSeriesThreshold = v
        } else {
            botSeriesThreshold = 5
        }
    } else {
        botSeriesThreshold = 5
    }
    if botChapterThresholdStr != "" {
        if v, err := strconv.Atoi(botChapterThresholdStr); err == nil && v > 0 {
            botChapterThreshold = v
        } else {
            botChapterThreshold = 10
        }
    } else {
        botChapterThreshold = 10
    }
    if botDetectionWindowStr != "" {
        if v, err := strconv.Atoi(botDetectionWindowStr); err == nil && v > 0 {
            botDetectionWindow = v
        } else {
            botDetectionWindow = 60
        }
    } else {
        botDetectionWindow = 60
    }
    if _, err := models.UpdateBotDetectionConfig(botDetectionEnabled, botSeriesThreshold, botChapterThreshold, botDetectionWindow); err != nil {
        return handleError(c, err)
    }
    
    // Update compression quality configuration
    readerQualityStr := c.FormValue("reader_compression_quality")
    moderatorQualityStr := c.FormValue("moderator_compression_quality")
    adminQualityStr := c.FormValue("admin_compression_quality")
    premiumQualityStr := c.FormValue("premium_compression_quality")
    anonymousQualityStr := c.FormValue("anonymous_compression_quality")
    processedQualityStr := c.FormValue("processed_image_quality")
    var readerQuality, moderatorQuality, adminQuality, premiumQuality, anonymousQuality, processedQuality int
    if readerQualityStr != "" {
        if v, err := strconv.Atoi(readerQualityStr); err == nil && v >= 0 && v <= 100 {
            readerQuality = v
        } else {
            readerQuality = 70
        }
    } else {
        readerQuality = 70
    }
    if moderatorQualityStr != "" {
        if v, err := strconv.Atoi(moderatorQualityStr); err == nil && v >= 0 && v <= 100 {
            moderatorQuality = v
        } else {
            moderatorQuality = 85
        }
    } else {
        moderatorQuality = 85
    }
    if adminQualityStr != "" {
        if v, err := strconv.Atoi(adminQualityStr); err == nil && v >= 0 && v <= 100 {
            adminQuality = v
        } else {
            adminQuality = 100
        }
    } else {
        adminQuality = 100
    }
    if premiumQualityStr != "" {
        if v, err := strconv.Atoi(premiumQualityStr); err == nil && v >= 0 && v <= 100 {
            premiumQuality = v
        } else {
            premiumQuality = 90
        }
    } else {
        premiumQuality = 90
    }
    if anonymousQualityStr != "" {
        if v, err := strconv.Atoi(anonymousQualityStr); err == nil && v >= 0 && v <= 100 {
            anonymousQuality = v
        } else {
            anonymousQuality = 70
        }
    } else {
        anonymousQuality = 70
    }
    if processedQualityStr != "" {
        if v, err := strconv.Atoi(processedQualityStr); err == nil && v >= 0 && v <= 100 {
            processedQuality = v
        } else {
            processedQuality = 85
        }
    } else {
        processedQuality = 85
    }
    if _, err := models.UpdateCompressionConfig(readerQuality, moderatorQuality, adminQuality, premiumQuality, anonymousQuality, processedQuality); err != nil {
        return handleError(c, err)
    }
    
    // Update image token validity configuration
    imageTokenValidityStr := c.FormValue("image_token_validity_minutes")
    var imageTokenValidity int
    if imageTokenValidityStr != "" {
        if v, err := strconv.Atoi(imageTokenValidityStr); err == nil && v >= 1 && v <= 60 {
            imageTokenValidity = v
        } else {
            imageTokenValidity = 5
        }
    } else {
        imageTokenValidity = 5
    }
    if _, err := models.UpdateImageTokenConfig(imageTokenValidity); err != nil {
        return handleError(c, err)
    }
    
    // Update premium early access configuration
    premiumEarlyAccessStr := c.FormValue("premium_early_access_duration")
    var premiumEarlyAccess int
    if premiumEarlyAccessStr != "" {
        if v, err := strconv.Atoi(premiumEarlyAccessStr); err == nil && v >= 0 {
            premiumEarlyAccess = v
        } else {
            premiumEarlyAccess = 3600
        }
    } else {
        premiumEarlyAccess = 3600
    }
    if _, err := models.UpdatePremiumEarlyAccessConfig(premiumEarlyAccess); err != nil {
        return handleError(c, err)
    }
    
    // Update max premium chapters configuration
    maxPremiumChaptersStr := c.FormValue("max_premium_chapters")
    var maxPremiumChapters int
    if maxPremiumChaptersStr != "" {
        if v, err := strconv.Atoi(maxPremiumChaptersStr); err == nil && v >= 0 {
            maxPremiumChapters = v
        } else {
            maxPremiumChapters = 3
        }
    } else {
        maxPremiumChapters = 3
    }
    if _, err := models.UpdateMaxPremiumChaptersConfig(maxPremiumChapters); err != nil {
        return handleError(c, err)
    }
    
    return HandleView(c, views.ConfigForm())
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
            utils.HandleConsoleLogsWebSocket(conn)
        })(c)
    }
    return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
}
