package handlers

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// Admin specific handlers
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

	return c.SendString(tableContent)
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

	return c.SendString(tableContent)
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

	return c.SendString(tableContent)
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

func HandleCancelEdit(c *fiber.Ctx) error {
	// Render a fresh LibraryForm
	var buf bytes.Buffer
	err := views.LibraryForm(models.Library{}, "post").Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}
	formContent := buf.String()

	c.Response().Header.Set("Content-Type", "text/html")
	return c.SendString(formContent)
}
