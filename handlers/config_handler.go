package handlers

import (
    "strconv"
    "github.com/alexander-bruun/magi/models"
    "github.com/alexander-bruun/magi/views"
    "github.com/gofiber/fiber/v2"
)

// HandleConfiguration renders the configuration page.
func HandleConfiguration(c *fiber.Ctx) error {
    return HandleView(c, views.Config())
}

// HandleConfigurationUpdate processes updates to the global configuration.
func HandleConfigurationUpdate(c *fiber.Ctx) error {
    // Checkbox only present when enabled
    allow := c.FormValue("allow_registration") == "on"
    requireLogin := c.FormValue("require_login_for_content") == "on"
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
    if _, err := models.UpdateAppConfig(allow, maxUsers, contentRatingLimit, requireLogin); err != nil {
        return handleError(c, err)
    }
    return HandleView(c, views.Config())
}
