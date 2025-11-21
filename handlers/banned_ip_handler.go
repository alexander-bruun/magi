package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// HandleBannedIPs displays the list of banned IPs
func HandleBannedIPs(c *fiber.Ctx) error {
	return HandleView(c, views.BannedIPs())
}

// HandleUnbanIP removes an IP from the banned list
func HandleUnbanIP(c *fiber.Ctx) error {
	ip := c.Params("ip")
	if ip == "" {
		return c.Status(fiber.StatusBadRequest).SendString("IP address is required")
	}

	err := models.UnbanIP(ip)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to unban IP")
	}

	bannedIPs, err := models.GetBannedIPs()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.BannedIPsTable(bannedIPs))
}