package handlers

import (
	"github.com/gofiber/fiber/v3"
)

// HandleGetReviews retrieves all reviews for a media - DEPRECATED
func HandleGetReviews(c fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error": "Reviews feature has been deprecated",
	})
}

// HandleCreateReview creates or updates a review - DEPRECATED
func HandleCreateReview(c fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error": "Reviews feature has been deprecated",
	})
}

// HandleGetUserReview gets the current user's review for a media - DEPRECATED
func HandleGetUserReview(c fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error": "Reviews feature has been deprecated",
	})
}

// HandleDeleteReview deletes the user's review for a media - DEPRECATED
func HandleDeleteReview(c fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error": "Reviews feature has been deprecated",
	})
}
