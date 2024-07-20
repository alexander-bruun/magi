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

func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

func HandleAdmin(c *fiber.Ctx) error {
	libraries, _ := models.GetLibraries()
	return HandleView(c, views.Admin(libraries))
}

// Admin HTMX endpoints
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
	err = views.LibraryTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="libraries-table">%s</div>`, tableContent))
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
	err = views.LibraryTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="libraries-table">%s</div>`, tableContent))
}

func HandleUpdateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	id, _ := strconv.ParseUint(c.Params("id"), 10, 64)
	library.ID = uint(id)

	if err := models.UpdateLibrary(&library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Fetch all libraries, including the newly created one
	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Render the updated libraries table
	var buf bytes.Buffer
	err = views.LibraryTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="libraries-table">%s</div>`, tableContent))
}

func HandleEditLibrary(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid ID")
	}

	library, err := models.GetLibrary(uint(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Render the updated libraries table
	var buf bytes.Buffer
	err = views.LibraryForm(*library, "put").Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering table")
	}
	tableContent := buf.String()

	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")

	return c.SendString(fmt.Sprintf(`<div id="library-form">%s</div>`, tableContent))
}

func HandleAddFolder(c *fiber.Ctx) error {
	return HandleView(c, views.Folder(""))
}

func HandleRemoveFolder(c *fiber.Ctx) error {
	return c.SendString("")
}
