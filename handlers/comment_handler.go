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

	var comments []models.Comment
	var err error

	if targetType == "chapter" {
		// For chapter comments, filter by media_slug too
		comments, err = models.GetCommentsByTargetAndMedia(targetType, targetSlug, mediaSlug)
	} else {
		// For media comments, just use target
		comments, err = models.GetCommentsByTarget(targetType, targetSlug)
	}

	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := c.BodyParser(&req); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	if req.Content == "" {
		return sendBadRequestError(c, ErrEmptyComment)
	}

	comment := models.Comment{
		UserUsername: user.Username,
		TargetType:   targetType,
		TargetSlug:   targetSlug,
		MediaSlug:    mediaSlug,
		Content:      req.Content,
	}

	err = models.CreateComment(comment)
	if err != nil {
		return sendInternalServerError(c, ErrCommentCreateFailed, err)
	}

	// If HTMX request, return updated comments section
	if c.Get("HX-Request") == "true" {
		// Fetch updated comments
		var comments []models.Comment
		var err error

		if targetType == "chapter" {
			// For chapter comments, filter by media_slug too
			comments, err = models.GetCommentsByTargetAndMedia(targetType, targetSlug, mediaSlug)
		} else {
			// For media comments, just use target
			comments, err = models.GetCommentsByTarget(targetType, targetSlug)
		}

		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Get media and chapter for the template
		var media *models.Media
		var chapter *models.Chapter
		if targetType == "chapter" {
			// For chapter comments, get both media and chapter
			chapter, err = models.GetChapter(mediaSlug, targetSlug)
			if err != nil || chapter == nil {
				return sendInternalServerError(c, ErrInternalServerError, err)
			}
			media, err = models.GetMedia(mediaSlug)
			if err != nil || media == nil {
				return sendInternalServerError(c, ErrInternalServerError, err)
			}
		} else {
			// For media comments, get media
			media, err = models.GetMedia(targetSlug)
			if err != nil || media == nil {
				return sendInternalServerError(c, ErrInternalServerError, err)
			}
			chapter = &models.Chapter{} // Empty chapter for media comments
		}

		// Render the component to HTML
		var buf bytes.Buffer
		err = views.ChapterCommentsSection(*media, *chapter, comments, user.Role, user.Username).Render(context.Background(), &buf)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		html := buf.String()
		wrapped := fmt.Sprintf(`<div id="comments-section" class="mt-8">%s</div>`, html)

		// Add success notification for HTMX requests
		triggerNotification(c, "Comment posted successfully", "success")

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
		return sendBadRequestError(c, "Invalid comment ID")
	}

	userName, ok := c.Locals("user_name").(string)
	if !ok || userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	err = models.DeleteComment(commentID, user.Username)
	if err != nil {
		if err.Error() == "comment not found or not authorized" {
			return sendForbiddenError(c, "You don't have permission to delete this comment")
		}
		return sendInternalServerError(c, ErrCommentDeleteFailed, err)
	}

	return c.JSON(fiber.Map{
		"message": "Comment deleted successfully",
	})
}
