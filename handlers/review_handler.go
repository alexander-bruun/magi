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

// HandleGetReviews retrieves all reviews for a media
func HandleGetReviews(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")

	reviews, err := models.GetReviewsByMedia(mediaSlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve reviews",
		})
	}

	return c.JSON(reviews)
}

// HandleCreateReview creates or updates a review
func HandleCreateReview(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")

	userName, ok := c.Locals("user_name").(string)
	if !ok || userName == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	var req struct {
		ReviewId int    `json:"reviewId"`
		Rating   int    `json:"rating"`
		Content  string `json:"content,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Rating < 1 || req.Rating > 10 {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Trigger", `{"showNotification": {"message": "Please select a star rating between 1 and 10", "status": "warning"}}`)
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Rating must be between 1 and 10",
		})
	}

	if req.ReviewId != 0 {
		// Update existing review
		existingReview, err := models.GetReviewByID(req.ReviewId)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to retrieve review",
			})
		}
		if existingReview == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Review not found",
			})
		}
		if existingReview.UserUsername != user.Username && user.Role != "admin" && user.Role != "moderator" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden",
			})
		}
		err = models.UpdateReviewByID(req.ReviewId, req.Rating, req.Content)
	} else {
		// Create or update user's review
		review := models.Review{
			UserUsername: user.Username,
			MediaSlug:    mediaSlug,
			Rating:       req.Rating,
			Content:      req.Content,
		}
		err = models.CreateReview(review)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save review",
		})
	}

	// If HTMX request, return updated reviews section
	if c.Get("HX-Request") == "true" {
		// Fetch updated reviews and user review
		reviews, err := models.GetReviewsByMedia(mediaSlug)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading reviews")
		}
		
		userReview, err := models.GetReviewByUserAndMedia(user.Username, mediaSlug)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading user review")
		}

		// Get media for the template
		media, err := models.GetMedia(mediaSlug)
		if err != nil || media == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading media")
		}

		// Render the component to HTML
		var buf bytes.Buffer
		err = views.MediaReviewsSection(*media, reviews, userReview, user.Role, user.Username).Render(context.Background(), &buf)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error rendering reviews section")
		}
		html := buf.String()
		wrapped := fmt.Sprintf(`<div id="reviews-section" class="mt-8">%s</div>`, html)
		return c.SendString(wrapped)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Review created successfully",
	})
}

// HandleGetUserReview gets the current user's review for a media
func HandleGetUserReview(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")

	userName, ok := c.Locals("user_name").(string)
	if !ok || userName == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	review, err := models.GetReviewByUserAndMedia(user.Username, mediaSlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve review",
		})
	}

	if review == nil {
		return c.JSON(fiber.Map{})
	}

	return c.JSON(review)
}

// HandleDeleteReview deletes the user's review for a media or a specific review by ID for mods
func HandleDeleteReview(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	reviewIdStr := c.Params("reviewId")

	userName, ok := c.Locals("user_name").(string)
	if !ok || userName == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	if reviewIdStr != "" {
		// Delete by ID, check if mod
		reviewId, err := strconv.Atoi(reviewIdStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid review ID",
			})
		}
		if user.Role != "admin" && user.Role != "moderator" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden",
			})
		}
		err = models.DeleteReviewByID(reviewId)
	} else {
		// Delete own review
		err = models.DeleteReview(user.Username, mediaSlug)
	}

	if err != nil {
		if err.Error() == "review not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Review not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete review",
		})
	}

	// If HTMX request, return updated reviews section
	if c.Get("HX-Request") == "true" {
		// Fetch updated reviews and user review (should be nil now)
		reviews, err := models.GetReviewsByMedia(mediaSlug)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading reviews")
		}

		// Get media for the template
		media, err := models.GetMedia(mediaSlug)
		if err != nil || media == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading media")
		}

		return HandleView(c, views.MediaReviewsSection(*media, reviews, nil, user.Role, user.Username))
	}

	return c.JSON(fiber.Map{
		"message": "Review deleted successfully",
	})
}