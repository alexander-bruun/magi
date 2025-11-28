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

// HandleGetComments retrieves comments for a target (media or chapter)
func HandleGetComments(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	var targetType, targetSlug string
	if chapterSlug != "" {
		targetType = "chapter"
		targetSlug = chapterSlug
	} else {
		targetType = "media"
		targetSlug = mediaSlug
	}

	comments, err := models.GetCommentsByTarget(targetType, targetSlug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve comments",
		})
	}

	return c.JSON(comments)
}

// HandleCreateComment creates a new comment
func HandleCreateComment(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterSlug := c.Params("chapter")

	var targetType, targetSlug string
	if chapterSlug != "" {
		targetType = "chapter"
		targetSlug = chapterSlug
	} else {
		targetType = "media"
		targetSlug = mediaSlug
	}

	// Get user from context (set by auth middleware)
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
		Content string `json:"content"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Content is required",
		})
	}

	comment := models.Comment{
		UserUsername: user.Username,
		TargetType:   targetType,
		TargetSlug:   targetSlug,
		Content:      req.Content,
	}

	err = models.CreateComment(comment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create comment",
		})
	}

	// If HTMX request, return updated comments section
	if c.Get("HX-Request") == "true" {
		// Fetch updated comments
		comments, err := models.GetCommentsByTarget(targetType, targetSlug)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading comments")
		}

		// Get media and chapter for the template
		var media *models.Media
		var chapter *models.Chapter
		if targetType == "chapter" {
			// For chapter comments, get both media and chapter
			chapter, err = models.GetChapter(mediaSlug, targetSlug)
			if err != nil || chapter == nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error loading chapter")
			}
			media, err = models.GetMedia(mediaSlug)
			if err != nil || media == nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error loading media")
			}
		} else {
			// For media comments, get media
			media, err = models.GetMedia(targetSlug)
			if err != nil || media == nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error loading media")
			}
			chapter = &models.Chapter{} // Empty chapter for media comments
		}

		// Render the component to HTML
		var buf bytes.Buffer
		err = views.ChapterCommentsSection(*media, *chapter, comments, user.Role, user.Username).Render(context.Background(), &buf)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error rendering comments section")
		}
		html := buf.String()
		wrapped := fmt.Sprintf(`<div id="comments-section" class="mt-8">%s</div>`, html)
		return c.SendString(wrapped)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Comment created successfully",
	})
}

// HandleDeleteComment deletes a comment (only by author)
func HandleDeleteComment(c *fiber.Ctx) error {
	commentIDStr := c.Params("id")
	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid comment ID",
		})
	}

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

	err = models.DeleteComment(commentID, user.Username)
	if err != nil {
		if err.Error() == "comment not found or not authorized" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Not authorized to delete this comment",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete comment",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Comment deleted successfully",
	})
}