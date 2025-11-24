package handlers

import (
	"strconv"

	"github.com/alexander-bruun/magi/models"
	fiber "github.com/gofiber/fiber/v2"
)

// HandleGetNotifications retrieves all notifications for the authenticated user
func HandleGetNotifications(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	unreadOnly := c.Query("unread_only", "false") == "true"

	notifications, err := models.GetUserNotifications(userName, unreadOnly)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"notifications": notifications,
	})
}

// HandleGetUnreadCount returns the count of unread notifications for the authenticated user
func HandleGetUnreadCount(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	count, err := models.GetUnreadNotificationCount(userName)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"count": count,
	})
}

// HandleMarkNotificationRead marks a specific notification as read
func HandleMarkNotificationRead(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	idStr := c.Params("id")
	notificationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid notification ID")
	}

	if err := models.MarkNotificationAsRead(notificationID, userName); err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// HandleMarkAllNotificationsRead marks all notifications as read for the authenticated user
func HandleMarkAllNotificationsRead(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	if err := models.MarkAllNotificationsAsRead(userName); err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// HandleDeleteNotification deletes a specific notification
func HandleDeleteNotification(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	idStr := c.Params("id")
	notificationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid notification ID")
	}

	if err := models.DeleteNotification(notificationID, userName); err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// HandleClearReadNotifications clears all read notifications for the authenticated user
func HandleClearReadNotifications(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	if err := models.ClearReadNotifications(userName); err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}
