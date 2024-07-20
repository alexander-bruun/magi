package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/alexander-bruun/magi/models"
)

func CreateMangaHandler(c *fiber.Ctx) error {
	var manga models.Manga
	err := json.Unmarshal(c.Body(), &manga)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	_, err = models.CreateManga(manga)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusCreated)
}

func GetMangaHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	manga, err := models.GetManga(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.JSON(manga)
}

func UpdateMangaHandler(c *fiber.Ctx) error {
	var manga models.Manga
	err := json.Unmarshal(c.Body(), &manga)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	err = models.UpdateManga(&manga)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

func DeleteMangaHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	err = models.DeleteManga(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

func GetMangasHandler(c *fiber.Ctx) error {
	filter := c.Query("filter")
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	sortBy := c.Query("sortBy")
	sortOrder := c.Query("sortOrder")
	filterBy := c.Query("filterBy")
	libraryID, _ := strconv.ParseUint(c.Query("library"), 10, 32)

	mangas, count, err := models.SearchMangas(filter, page, pageSize, sortBy, sortOrder, filterBy, uint(libraryID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("X-Total-Count", strconv.FormatInt(count, 10))
	c.Set("Access-Control-Expose-Headers", "X-Total-Count")

	return c.JSON(mangas)
}
