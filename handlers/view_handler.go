package handlers

import (
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

func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

func HandleAdmin(c *fiber.Ctx) error {
	libraries, _ := models.GetLibraries()
	return HandleView(c, views.Admin(libraries))
}
