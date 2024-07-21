package handlers

import (
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

func HandleMangas(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page <= 0 {
		page = 1
	}

	mangas, count, err := models.SearchMangas("", page, 9, "name", "asc", "", 0)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Mangas(mangas, int(count), page))
}

func HandleManga(c *fiber.Ctx) error {
	slug := c.Params("slug")

	id, err := models.GetMangaIDBySlug(slug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	manga, err := models.GetManga(id)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Manga(*manga))
}
