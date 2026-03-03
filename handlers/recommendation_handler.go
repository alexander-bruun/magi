package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v3"
)

// HandleMediaRecommendations returns recommended media for a given series slug
func HandleMediaRecommendations(c fiber.Ctx) error {
	slug := c.Params("media")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing media slug"})
	}

	media, err := models.GetMedia(slug)
	if err != nil || media == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Media not found"})
	}

	recs, err := models.GetRecommendedMedia(media)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(recs)
}
