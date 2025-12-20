package handlers

import (
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
		return sendBadRequestError(c, "Media slug cannot be empty")
	}

	if err := DeleteMedia(mangaSlug); err != nil {
		log.Errorf("Failed to delete media '%s': %v", mangaSlug, err)
		return sendInternalServerError(c, ErrMediaDeleteFailed, err)
	}

	log.Infof("Successfully deleted media '%s'", mangaSlug)

	// Add success notification for HTMX requests
	triggerNotification(c, "Media deleted successfully", "success")

	// Redirect to media list
	c.Set("HX-Redirect", "/series")
	return c.SendStatus(fiber.StatusOK)
}
