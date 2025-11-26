package handlers

import (
	"fmt"

	"github.com/alexander-bruun/magi/models"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// DeleteMedia deletes a media and all associated data
func DeleteMedia(mediaSlug string) error {
	existingMedia, err := models.GetMediaUnfiltered(mediaSlug)
	if err != nil {
		return err
	}
	if existingMedia == nil {
		return nil // Not found
	}

	// Delete the media (chapters and tags will be cascade deleted)
	return models.DeleteMedia(existingMedia.Slug)
}

// HandleDeleteMedia deletes a media and all associated data
func HandleDeleteMedia(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("media slug can't be empty"))
	}

	if err := DeleteMedia(mangaSlug); err != nil {
		log.Errorf("Failed to delete media '%s': %v", mangaSlug, err)
		return handleError(c, fmt.Errorf("failed to delete media: %w", err))
	}

	log.Infof("Successfully deleted media '%s'", mangaSlug)

	// Redirect to media list
	c.Set("HX-Redirect", "/series")
	return c.SendStatus(fiber.StatusOK)
}
