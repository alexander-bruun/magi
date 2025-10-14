package handlers

import (
    "strconv"
    "github.com/alexander-bruun/magi/models"
    "github.com/alexander-bruun/magi/views"
    "github.com/gofiber/fiber/v2"
)

// HandleConfiguration renders the configuration page.
func HandleConfiguration(c *fiber.Ctx) error {
    cfg, err := models.GetAppConfig()
    if err != nil {
        return handleError(c, err)
    }
    count, err := models.CountUsers()
    if err != nil {
        return handleError(c, err)
    }
    return HandleView(c, views.Config(cfg, count))
}

// HandleConfigurationUpdate processes updates to the global configuration.
func HandleConfigurationUpdate(c *fiber.Ctx) error {
    // Checkbox only present when enabled
    allow := c.FormValue("allow_registration") == "on"
    maxUsersStr := c.FormValue("max_users")
    var maxUsers int64
    if maxUsersStr != "" {
        if v, err := strconv.ParseInt(maxUsersStr, 10, 64); err == nil && v >= 0 {
            maxUsers = v
        }
    }
    if _, err := models.UpdateAppConfig(allow, maxUsers); err != nil {
        return handleError(c, err)
    }
    cfg, _ := models.GetAppConfig()
    count, _ := models.CountUsers()
    return HandleView(c, views.Config(cfg, count))
}
