package handlers

import (
	"bytes"
	"context"
	"fmt"
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

func HandleAdmin(c *fiber.Ctx) error {
	return HandleView(c, views.Admin())
}

// New HTMX-specific handlers

func HandleGetLibraries(c *fiber.Ctx) error {
	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return HandleView(c, views.LibrariesTable(libraries))
}

func HandleCreateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if err := models.CreateLibrary(library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Fetch all libraries, including the newly created one
	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Render the updated libraries table
	var buf bytes.Buffer
	err = views.LibrariesTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="libraries-table">%s</div>`, tableContent))
}

func HandleGetLibrary(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid ID")
	}

	library, err := models.GetLibrary(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Library not found")
	}

	return c.JSON(library)
}

func HandleUpdateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if err := models.UpdateLibrary(&library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendString("Library updated successfully")
}

func HandleDeleteLibrary(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid ID")
	}

	if err := models.DeleteLibrary(uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Fetch all libraries, excluding the deleted one
	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Render the updated libraries table
	var buf bytes.Buffer
	err = views.LibrariesTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="libraries-table">%s</div>`, tableContent))
}

func HandleAddFolder(c *fiber.Ctx) error {
	return HandleView(c, views.FolderInput())
}

func HandleRemoveFolder(c *fiber.Ctx) error {
	return c.SendString("")
}
