package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
)

func CreateLibraryHandler(c *fiber.Ctx) error {
	var library models.Library
	err := json.Unmarshal(c.Body(), &library)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	err = models.CreateLibrary(library)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusCreated)
}

func GetLibrariesHandler(c *fiber.Ctx) error {
	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(libraries)
}

func GetLibraryHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid library ID")
	}

	library, err := models.GetLibrary(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.JSON(library)
}

func UpdateLibraryHandler(c *fiber.Ctx) error {
	var library models.Library
	err := json.Unmarshal(c.Body(), &library)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	err = models.UpdateLibrary(&library)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

func DeleteLibraryHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid library ID")
	}

	err = models.DeleteLibrary(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

func SearchLibrariesHandler(c *fiber.Ctx) error {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	sortBy := c.Query("sortBy")
	sortOrder := c.Query("sortOrder")

	libraries, err := models.SearchLibraries(keyword, page, pageSize, sortBy, sortOrder)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(libraries)
}
