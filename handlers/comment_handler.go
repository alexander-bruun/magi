package handlers

import (
	"bytes"
	"context"
	"fmt"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v3"
)

// getChapterByHash extracts the "hash" param and returns the chapter or an error response.
func getChapterByHash(c fiber.Ctx) (*models.Chapter, error) {
	hash := c.Params("hash")
	chapter, err := models.GetChapterByID(hash)
	if err != nil {
		return nil, SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return nil, SendNotFoundError(c, ErrChapterNotFound)
	}
	return chapter, nil
}

// getChapterBySeriesParams extracts "media" and "chapter" params and returns the chapter or an error response.
func getChapterBySeriesParams(c fiber.Ctx) (*models.Chapter, error) {
	mediaSlug := c.Params("media")
	chapterSlug := "chapter-" + c.Params("chapter")
	chapter, err := models.GetChapter(mediaSlug, "", chapterSlug)
	if err != nil {
		return nil, SendInternalServerError(c, ErrInternalServerError, err)
	}
	if chapter == nil {
		return nil, SendNotFoundError(c, ErrChapterNotFound)
	}
	return chapter, nil
}

// requireAuthenticatedUser extracts and validates the authenticated user or returns an error response.
func requireAuthenticatedUser(c fiber.Ctx) (*models.User, error) {
	userName, ok := c.Locals("user_name").(string)
	if !ok || userName == "" {
		return nil, SendUnauthorizedError(c, ErrUnauthorized)
	}
	user, err := models.FindUserByUsername(userName)
	if err != nil || user == nil {
		return nil, SendUnauthorizedError(c, ErrUnauthorized)
	}
	return user, nil
}

// parseCommentContent parses and validates the comment content from the request body.
func parseCommentContent(c fiber.Ctx) (string, error) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return "", SendBadRequestError(c, ErrBadRequest)
	}
	if req.Content == "" {
		return "", SendBadRequestError(c, ErrEmptyComment)
	}
	return req.Content, nil
}

// renderHTMXCommentCreatedResponse renders the HTMX response after a comment is created.
func renderHTMXCommentCreatedResponse(c fiber.Ctx, chapter *models.Chapter, user *models.User) (bool, error) {
	if c.Get("HX-Request") != "true" {
		return false, nil
	}
	comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return true, SendInternalServerError(c, ErrInternalServerError, err)
	}
	media, err := models.GetMedia(chapter.MediaSlug)
	if err != nil || media == nil {
		return true, SendInternalServerError(c, ErrInternalServerError, err)
	}
	var buf bytes.Buffer
	err = views.ChapterCommentsSection(*media, *chapter, comments, user.Role, user.Username).Render(context.Background(), &buf)
	if err != nil {
		return true, SendInternalServerError(c, ErrInternalServerError, err)
	}
	wrapped := fmt.Sprintf(`<div id="comments-section" class="mt-8">%s</div>`, buf.String())
	triggerNotification(c, "Comment posted successfully", "success")
	return true, c.SendString(wrapped)
}

// handleCreateCommentCore is the shared logic for creating a comment on a chapter.
func handleCreateCommentCore(c fiber.Ctx, chapter *models.Chapter) error {
	user, err := requireAuthenticatedUser(c)
	if err != nil {
		return err
	}

	content, err := parseCommentContent(c)
	if err != nil {
		return err
	}

	comment := models.Comment{
		UserUsername: user.Username,
		TargetType:  "chapter",
		TargetSlug:  chapter.Slug,
		MediaSlug:   chapter.MediaSlug,
		Content:     content,
	}
	if err := models.CreateComment(comment); err != nil {
		return SendInternalServerError(c, ErrCommentCreateFailed, err)
	}

	if responded, err := renderHTMXCommentCreatedResponse(c, chapter, user); responded {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Comment created successfully",
	})
}

// HandleGetComments retrieves comments for a target (media or chapter)
func HandleGetComments(c fiber.Ctx) error {
	chapter, err := getChapterByHash(c)
	if err != nil {
		return err
	}

	// Get comments for the chapter
	comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// If HTMX request, return HTML
	if c.Get("HX-Request") == "true" {
		// Get media for the template
		media, err := models.GetMedia(chapter.MediaSlug)
		if err != nil || media == nil {
			return SendInternalServerError(c, ErrInternalServerError, fmt.Errorf("media not found"))
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
			return SendInternalServerError(c, ErrInternalServerError, err)
		}

		c.Set("Content-Type", "text/html")
		return c.Send(buf.Bytes())
	}

	return c.JSON(comments)
}

// HandleGetCommentsForSeries retrieves comments for a chapter by series slug and chapter number
func HandleGetCommentsForSeries(c fiber.Ctx) error {
	chapter, err := getChapterBySeriesParams(c)
	if err != nil {
		return err
	}

	// Get comments for the chapter
	comments, err := models.GetCommentsByTargetAndMedia("chapter", chapter.Slug, chapter.MediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	return c.JSON(comments)
}

// HandleCreateComment creates a new comment
func HandleCreateComment(c fiber.Ctx) error {
	chapter, err := getChapterByHash(c)
	if err != nil {
		return err
	}
	return handleCreateCommentCore(c, chapter)
}

// HandleCreateCommentForSeries creates a new comment for a chapter by series slug and chapter number
func HandleCreateCommentForSeries(c fiber.Ctx) error {
	chapter, err := getChapterBySeriesParams(c)
	if err != nil {
		return err
	}
	return handleCreateCommentCore(c, chapter)
}

// HandleDeleteComment deletes a comment (only by author)
func HandleDeleteComment(c fiber.Ctx) error {
	commentID, err := ParseIntParam(c, "id", "Invalid comment ID")
	if err != nil {
		return err
	}

	user, err := requireAuthenticatedUser(c)
	if err != nil {
		return err
	}

	err = models.DeleteComment(commentID, user.Username)
	if err != nil {
		if err.Error() == "comment not found or not authorized" {
			return SendForbiddenError(c, "You don't have permission to delete this comment")
		}
		return SendInternalServerError(c, ErrCommentDeleteFailed, err)
	}

	return c.JSON(fiber.Map{
		"message": "Comment deleted successfully",
	})
}
