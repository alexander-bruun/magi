package handlers

import (
	"bytes"
	"context"
	"fmt"

	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

func HandleLibraries(c *fiber.Ctx) error {
	libraries, err := models.GetLibraries()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Libraries(libraries))
}

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
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
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
	log.Infof("Updating library: %s", library.Slug)

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
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	var buf bytes.Buffer
	err = views.LibraryForm(*library, "put", true).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	setCommonHeaders(c)
	return c.SendString(fmt.Sprintf(`<div id="library-form">%s</div>`, buf.String()))
}

func HandleScanLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	log.Infof("Starting manual scan for library: %s", library.Name)
	// Create a temporary Indexer for this library and run the job so we
	// preserve the same logging and lifecycle as scheduled jobs.
	idx := indexer.NewIndexer(*library)
	// RunIndexingJob will process all folders for the library.
	idx.RunIndexingJob()

	return c.SendString(`<uk-icon icon="Check"></uk-icon>`)
}

func HandleAddFolder(c *fiber.Ctx) error {
	return HandleView(c, views.Folder(""))
}

func HandleRemoveFolder(c *fiber.Ctx) error {
	return c.SendString("")
}

func HandleCancelEdit(c *fiber.Ctx) error {
	var buf bytes.Buffer
	err := views.LibraryForm(models.Library{}, "post", false).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	c.Response().Header.Set("Content-Type", "text/html")
	return c.SendString(buf.String())
}
