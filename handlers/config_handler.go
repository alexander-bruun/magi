package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils/text"
	"github.com/alexander-bruun/magi/views"
	websocket "github.com/gofiber/contrib/v3/websocket"
	fiber "github.com/gofiber/fiber/v3"
)

// HandleConfiguration renders the configuration page.
func HandleConfiguration(c fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrConfigLoadFailed, err)
	}
	return handleView(c, views.Config(cfg))
}

// HandleConfigurationUpdate processes updates to the global configuration.
func HandleConfigurationUpdate(c fiber.Ctx) error {
	var cfg models.AppConfig
	if err := c.Bind().Body(&cfg); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	cfg.Validate()

	if _, err := models.SaveFullConfig(cfg); err != nil {
		return SendInternalServerError(c, ErrConfigUpdateFailed, err)
	}

	return handleView(c, views.ConfigForm())
}

// HandleConsoleLogsWebSocketUpgrade upgrades the connection to WebSocket for console logs
func HandleConsoleLogsWebSocketUpgrade(c fiber.Ctx) error {
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
