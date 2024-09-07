package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

func HandleUsers(c *fiber.Ctx) error {
	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Users(users))
}
