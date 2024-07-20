package handlers

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func HandleView(c *fiber.Ctx, content templ.Component) error {
	if c.Get("HX-Request") != "" {
		handler := adaptor.HTTPHandler(templ.Handler(content))
		return handler(c)
	}
	base := views.Layout(content)
	handler := adaptor.HTTPHandler(templ.Handler(base))
	return handler(c)
}

func HandleHome(c *fiber.Ctx) error {
	mangas, _, _ := models.SearchMangas("", 1, 10, "created_at", "desc", "", 0)
	return HandleView(c, views.Home(mangas))
}

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

func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

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
