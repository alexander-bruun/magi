package handlers

import (
	"bytes"
	"context"
	"fmt"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

func renderLibraryTable(libraries []models.Library) (string, error) {
	var buf bytes.Buffer
	err := views.LibraryTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return "", fmt.Errorf("error rendering table: %w", err)
	}
	return buf.String(), nil
}

func setCommonHeaders(c *fiber.Ctx) {
	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")
}

func HandleCreateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if err := models.CreateLibrary(library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

func HandleDeleteLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty.")
	}

	if err := models.DeleteLibrary(slug); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

func HandleUpdateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	library.Slug = c.Params("slug")

	if err := models.UpdateLibrary(&library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

func HandleEditLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty.")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	var buf bytes.Buffer
	err = views.LibraryForm(*library, "put").Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	setCommonHeaders(c)
	return c.SendString(fmt.Sprintf(`<div id="library-form">%s</div>`, buf.String()))
}

func HandleAddFolder(c *fiber.Ctx) error {
	return HandleView(c, views.Folder(""))
}

func HandleRemoveFolder(c *fiber.Ctx) error {
	return c.SendString("")
}

func HandleCancelEdit(c *fiber.Ctx) error {
	var buf bytes.Buffer
	err := views.LibraryForm(models.Library{}, "post").Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	c.Response().Header.Set("Content-Type", "text/html")
	return c.SendString(buf.String())
}
