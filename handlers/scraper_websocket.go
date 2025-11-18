package handlers

import (
	"fmt"
	"strconv"

	"github.com/alexander-bruun/magi/executor"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// HandleScraperLogsWebSocketUpgrade upgrades the connection to WebSocket and extracts the script ID
func HandleScraperLogsWebSocketUpgrade(c *fiber.Ctx) error {
	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(c) {
		// Extract script ID from route parameter
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("invalid script id: %v", err))
		}

		// Upgrade to WebSocket with the extracted script ID
		return websocket.New(func(conn *websocket.Conn) {
			executor.HandleLogsWebSocket(conn, id)
		})(c)
	}
	return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
}
