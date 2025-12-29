package handlers

import (
	"fmt"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// HandleReportIssue displays the issue reporting form
func HandleReportIssue(c *fiber.Ctx) error {
	return HandleView(c, views.ReportIssue())
}

// HandleCreateIssue processes the issue report submission
func HandleCreateIssue(c *fiber.Ctx) error {
	// Get user from context (set by auth middleware)
	userName, ok := c.Locals("user_name").(string)
	var userUsername *string
	if ok && userName != "" {
		userUsername = &userName
	}

	var req struct {
		Title       string `json:"title" form:"title"`
		Description string `json:"description" form:"description"`
		Category    string `json:"category" form:"category"`
		Priority    string `json:"priority" form:"priority"`
	}

	if err := c.BodyParser(&req); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	if req.Title == "" || req.Description == "" {
		return sendBadRequestError(c, ErrBadRequest)
	}

	// Validate category
	validCategories := map[string]bool{
		"bug": true, "feature": true, "improvement": true, "question": true,
	}
	if !validCategories[req.Category] {
		req.Category = "bug"
	}

	// Validate priority
	validPriorities := map[string]bool{
		"low": true, "medium": true, "high": true, "critical": true,
	}
	if !validPriorities[req.Priority] {
		req.Priority = "medium"
	}

	issue := models.Issue{
		UserUsername: userUsername,
		Title:        req.Title,
		Description:  req.Description,
		Status:       "open",
		Priority:     req.Priority,
		Category:     req.Category,
		UserAgent:    c.Get("User-Agent"),
		URL:          c.Get("Referer"),
	}

	err := models.CreateIssue(issue)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Create notifications for moderators and admins about the new issue
	moderators, err := models.GetUsersByRole("moderator")
	if err == nil {
		for _, mod := range moderators {
			models.CreateAdminNotification(mod.Username, fmt.Sprintf("New issue reported: %s", issue.Title))
		}
	}
	admins, err := models.GetUsersByRole("admin")
	if err == nil {
		for _, admin := range admins {
			models.CreateAdminNotification(admin.Username, fmt.Sprintf("New issue reported: %s", issue.Title))
		}
	}

	// If HTMX request, return success message
	if c.Get("HX-Request") == "true" {
		return renderComponent(c, views.IssueSuccess())
	}

	// Redirect to success page
	return c.Redirect("/report-issue/success")
}

// HandleReportIssueSuccess displays the success page after reporting an issue
func HandleReportIssueSuccess(c *fiber.Ctx) error {
	return c.Render("report-issue-success", fiber.Map{})
}

// HandleIssuesAdmin displays the admin issues management page
func HandleIssuesAdmin(c *fiber.Ctx) error {
	status := c.Query("status")
	category := c.Query("category")
	limitStr := c.Query("limit", "50")
	offsetStr := c.Query("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	issues, err := models.GetIssues(status, category, limit, offset)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	stats, err := models.GetIssueStats()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Check if this is an HTMX request (for filtering/sorting)
	if c.Get("HX-Request") == "true" {
		return HandleView(c, views.IssuesTable(issues, status, category, limit, offset))
	}

	return HandleView(c, views.Issues(issues, stats, status, category, limit, offset))
}

// HandleUpdateIssueStatus updates the status of an issue
func HandleUpdateIssueStatus(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	status := c.FormValue("status")
	if status == "" {
		return sendBadRequestError(c, ErrBadRequest)
	}

	// Validate status
	validStatuses := map[string]bool{
		"open": true, "in_progress": true, "closed": true,
	}
	if !validStatuses[status] {
		return sendBadRequestError(c, ErrBadRequest)
	}

	err = models.UpdateIssueStatus(id, status)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// If issue is closed, mark related admin notifications as read for all moderators/admins
	if status == "closed" {
		moderators, err := models.GetUsersByRole("moderator")
		if err == nil {
			for _, mod := range moderators {
				// Find and mark admin notifications as read
				notifications, err := models.GetUserNotifications(mod.Username, false)
				if err == nil {
					for _, notif := range notifications {
						if notif.Type == "admin_issue" && !notif.IsRead {
							models.MarkNotificationAsRead(notif.ID, mod.Username)
						}
					}
				}
			}
		}
		admins, err := models.GetUsersByRole("admin")
		if err == nil {
			for _, admin := range admins {
				// Find and mark admin notifications as read
				notifications, err := models.GetUserNotifications(admin.Username, false)
				if err == nil {
					for _, notif := range notifications {
						if notif.Type == "admin_issue" && !notif.IsRead {
							models.MarkNotificationAsRead(notif.ID, admin.Username)
						}
					}
				}
			}
		}
	}

	// If HTMX request, return updated issue row
	if c.Get("HX-Request") == "true" {
		issue, err := models.GetIssueByID(id)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		return renderComponent(c, views.IssueRow(*issue))
	}

	return c.JSON(fiber.Map{"success": true})
}
