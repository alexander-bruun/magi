package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v3"
)

// HandleMediaRecommendationsSlider renders the recommendations slider HTML fragment for a given series slug
func HandleMediaRecommendationsSlider(c fiber.Ctx) error {
	slug := c.Params("media")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing media slug")
	}

	media, err := models.GetMedia(slug)
	if err != nil || media == nil {
		return c.Status(fiber.StatusNotFound).SendString("Media not found")
	}

	recs, err := models.GetRecommendedMedia(media)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return views.RecommendationsSlider(recs).Render(c.Context(), c.Response().BodyWriter())
}
