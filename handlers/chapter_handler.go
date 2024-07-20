package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/alexander-bruun/magi/models"
)

// CreateChapterHandler handles creating a new chapter
func CreateChapterHandler(c *fiber.Ctx) error {
	var chapter models.Chapter
	err := c.BodyParser(&chapter)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	err = models.CreateChapter(chapter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusCreated)
}

// GetChapterHandler handles retrieving a chapter by ID
func GetChapterHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter ID")
	}

	chapter, err := models.GetChapter(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.JSON(chapter)
}

// UpdateChapterHandler handles updating an existing chapter
func UpdateChapterHandler(c *fiber.Ctx) error {
	var chapter models.Chapter
	err := c.BodyParser(&chapter)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	err = models.UpdateChapter(&chapter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// DeleteChapterHandler handles deleting a chapter by ID
func DeleteChapterHandler(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter ID")
	}

	err = models.DeleteChapter(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// SearchChaptersHandler handles searching chapters
func SearchChaptersHandler(c *fiber.Ctx) error {
	query := c.Query("keyword")
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	sortBy := c.Query("sortBy")
	sortOrder := c.Query("sortOrder")

	chapters, err := models.SearchChapters(query, page, pageSize, sortBy, sortOrder)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(chapters)
}
