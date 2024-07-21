package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

func HandleLibraries(c *fiber.Ctx) error {
	libraries, err := models.GetLibraries()
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Libraries(libraries))
}

func HandleLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")

	id, err := models.GetLibraryIDBySlug(slug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	library, err := models.GetLibrary(id)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Library(*library))
}
