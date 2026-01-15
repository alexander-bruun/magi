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
	hash := c.Params("hash")

	// Get chapter by ID
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return sendNotFoundError(c, ErrChapterNotFound)
	}

	// Get comments for the chapter
	comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// If HTMX request, return HTML
	if c.Get("HX-Request") == "true" {
		// Get media for the template
		media, err := models.GetMedia(chapter.MediaSlug)
		if err != nil || media == nil {
			return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("media not found"))
		}

		// Get user role and name if available
		userRole := ""
		userName := ""
		if userNameLocals, ok := c.Locals("user_name").(string); ok && userNameLocals != "" {
			user, err := models.FindUserByUsername(userNameLocals)
			if err == nil && user != nil {
				userRole = user.Role
				userName = user.Username
			}
		}

		var buf bytes.Buffer
		err = views.CommentsList(*media, *chapter, comments, userRole, userName).Render(context.Background(), &buf)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}

	return c.JSON(comments)
}

// HandleGetCommentsForSeries retrieves comments for a chapter by series slug and chapter number
func HandleGetCommentsForSeries(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterParam := c.Params("chapter")

	chapterSlug := "chapter-" + chapterParam

	// Get chapter by media slug and chapter slug
	chapter, err := models.GetChapter(mediaSlug, "", chapterSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return sendNotFoundError(c, ErrChapterNotFound)
	}

	// Get comments for the chapter
	comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return c.JSON(comments)
}

// HandleCreateComment creates a new comment
func HandleCreateComment(c *fiber.Ctx) error {
	hash := c.Params("hash")

	// Get chapter by ID
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return sendNotFoundError(c, ErrChapterNotFound)
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
		TargetType:   "chapter",
		TargetSlug:   chapter.Slug,
		MediaSlug:    chapter.MediaSlug,
		Content:      req.Content,
	}

	err = models.CreateComment(comment)
	if err != nil {
		return sendInternalServerError(c, ErrCommentCreateFailed, err)
	}

	// If HTMX request, return updated comments section
	if c.Get("HX-Request") == "true" {
		// Fetch updated comments
		comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Get media for the template
		media, err := models.GetMedia(chapter.MediaSlug)
		if err != nil || media == nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
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

// HandleCreateCommentForSeries creates a new comment for a chapter by series slug and chapter number
func HandleCreateCommentForSeries(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	chapterParam := c.Params("chapter")

	chapterSlug := "chapter-" + chapterParam

	// Get chapter by media slug and chapter slug
	chapter, err := models.GetChapter(mediaSlug, "", chapterSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return sendNotFoundError(c, ErrChapterNotFound)
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
		TargetType:   "chapter",
		TargetSlug:   chapter.Slug,
		MediaSlug:    chapter.MediaSlug,
		Content:      req.Content,
	}

	err = models.CreateComment(comment)
	if err != nil {
		return sendInternalServerError(c, ErrCommentCreateFailed, err)
	}

	// If HTMX request, return updated comments section
	if c.Get("HX-Request") == "true" {
		// Fetch updated comments
		comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Get media for the template
		media, err := models.GetMedia(chapter.MediaSlug)
		if err != nil || media == nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
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
