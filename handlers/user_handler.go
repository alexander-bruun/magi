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

func HandleUserBan(c *fiber.Ctx) error {
	username := c.Params("username")

	models.UpdateUserRole(username, "reader")
	models.BanUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

func HandleUserUnban(c *fiber.Ctx) error {
	username := c.Params("username")

	models.UnbanUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

func HandleUserPromote(c *fiber.Ctx) error {
	username := c.Params("username")

	models.PromoteUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

func HandleUserDemote(c *fiber.Ctx) error {
	username := c.Params("username")

	models.DemoteUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}
