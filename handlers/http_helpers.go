package handlers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/adaptor/v2"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

const (
	sessionTokenCookie = "session_token"
)

// QueryParams holds parsed query parameters for media listings
type QueryParams struct {
	Page         int
	Sort         string
	Order        string
	Tags         []string
	TagMode      string
	Types        []string
	LibrarySlug  string
	SearchFilter string
}

// ParseQueryParams extracts and normalizes query parameters from the request
func ParseQueryParams(c *fiber.Ctx) QueryParams {
	params := QueryParams{
		Page:    getPageNumber(c.Query("page")),
		TagMode: strings.ToLower(c.Query("tag_mode")),
	}

	// Parse sorting parameters - handle duplicates from HTMX includes
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		if valsMap, err := url.ParseQuery(raw); err == nil {
			if vals, ok := valsMap["sort"]; ok && len(vals) > 0 {
				params.Sort = vals[0]
			}
			if vals, ok := valsMap["order"]; ok && len(vals) > 0 {
				params.Order = vals[0]
			}
			// Parse tags from repeated params
			if vals, ok := valsMap["tags"]; ok {
				for _, v := range vals {
					for _, t := range strings.Split(v, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							params.Tags = append(params.Tags, t)
						}
					}
				}
			}
			// Parse types from repeated params
			if vals, ok := valsMap["types"]; ok {
				for _, v := range vals {
					for _, t := range strings.Split(v, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							params.Types = append(params.Types, t)
						}
					}
				}
			}
		}
	}

	// Fallback to simple query params
	if params.Sort == "" {
		params.Sort = c.Query("sort")
	}
	if params.Order == "" {
		params.Order = c.Query("order")
	}

	// Fallback for comma-separated tags
	if len(params.Tags) == 0 {
		if raw := c.Query("tags"); raw != "" {
			for _, t := range strings.Split(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					params.Tags = append(params.Tags, t)
				}
			}
		}
	}

	// Fallback for comma-separated types
	if len(params.Types) == 0 {
		if raw := c.Query("types"); raw != "" {
			for _, t := range strings.Split(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					params.Types = append(params.Types, t)
				}
			}
		}
	}

	// Normalize defaults
	if params.Sort == "" && params.Order == "" {
		params.Sort, params.Order = "name", "asc"
	} else {
		if params.Order != "asc" && params.Order != "desc" {
			params.Order = "asc"
		}
	}

	if params.TagMode != "any" {
		params.TagMode = "all"
	}

	params.LibrarySlug = c.Query("library")
	params.SearchFilter = c.Query("search")

	return params
}

// getPageNumber extracts and validates the page number
func getPageNumber(pageStr string) int {
	if pageStr == "" {
		return defaultPage
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return defaultPage
	}
	return page
}

// CalculateTotalPages computes total pages from count and page size
func CalculateTotalPages(count int64, pageSize int) int {
	totalPages := int((count + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		return 1
	}
	return totalPages
}

// GetUserContext extracts username from fiber context locals
func GetUserContext(c *fiber.Ctx) string {
	if userName, ok := c.Locals("user_name").(string); ok && userName != "" {
		return userName
	}
	return ""
}

// isHTMXRequest checks if the request is from HTMX
func isHTMXRequest(c *fiber.Ctx) bool {
	return c.Get("HX-Request") == "true"
}

// GetHTMXTarget returns the HTMX target ID
func GetHTMXTarget(c *fiber.Ctx) string {
	return c.Get("HX-Target")
}

// triggerNotification triggers an HTMX notification if the request is HTMX
func triggerNotification(c *fiber.Ctx, message string, status string) {
	if isHTMXRequest(c) {
		notification := map[string]interface{}{
			"showNotification": map[string]string{
				"message": message,
				"status":  status,
			},
		}
		jsonBytes, _ := json.Marshal(notification)
		c.Set("HX-Trigger", string(jsonBytes))
	}
}

// triggerCustomNotification triggers a custom HTMX notification with any event name
func triggerCustomNotification(c *fiber.Ctx, eventName string, data map[string]interface{}) {
	if isHTMXRequest(c) {
		var notification map[string]interface{}
		if eventName == "" {
			notification = data
		} else {
			notification = map[string]interface{}{
				eventName: data,
			}
		}
		jsonBytes, _ := json.Marshal(notification)
		c.Set("HX-Trigger", string(jsonBytes))
	}
}

// sendValidationError sends a validation error with HX-Trigger notification for HTMX requests
func sendValidationError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusUnprocessableEntity).SendString("")
	}
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"error": message,
	})
}

// sendUnauthorizedError sends an unauthorized error with HX-Trigger notification for HTMX requests
func sendUnauthorizedError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("")
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": message,
	})
}

// sendForbiddenError sends a forbidden error with HX-Trigger notification for HTMX requests
func sendForbiddenError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusForbidden).SendString("")
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": message,
	})
}

// sendConflictError sends a conflict error with HX-Trigger notification for HTMX requests
func sendConflictError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusConflict).SendString("")
	}
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"error": message,
	})
}

// sendNotFoundError sends a not found error with HX-Trigger notification for HTMX requests
func sendNotFoundError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusNotFound).SendString("")
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": message,
	})
}

// sendInternalServerError sends an internal server error with HX-Trigger notification for HTMX requests
func sendInternalServerError(c *fiber.Ctx, message string, err error) error {
	// Log the actual error for debugging
	log.Errorf("Internal server error: %v", err)

	triggerNotification(c, message, "destructive")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusInternalServerError).SendString("")
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// sendBadRequestError sends a bad request error with HX-Trigger notification for HTMX requests
func sendBadRequestError(c *fiber.Ctx, message string) error {
	triggerNotification(c, message, "warning")
	if isHTMXRequest(c) {
		return c.Status(fiber.StatusBadRequest).SendString("")
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": message,
	})
}

// handleView wraps a page component with the layout unless the request is an HTMX fragment.
func handleView(c *fiber.Ctx, content templ.Component, unreadCountAndNotifications ...interface{}) error {
	// Return partial content if HTMX request
	if isHTMXRequest(c) {
		return renderComponent(c, content)
	}

	userRole, err := getUserRole(c)
	if err != nil {
		// Log the error, but continue with an empty user role
		// This allows the page to render for non-authenticated users
		log.Errorf("Error getting user role (clearing session cookie): %v", err)
		// Clear the session cookie if the token is invalid
		if err.Error() == "invalid session token" {
			ClearSessionCookie(c)
		}
	}

	unreadCount := 0
	notifications := []models.UserNotification{}
	if len(unreadCountAndNotifications) > 0 {
		unreadCount = unreadCountAndNotifications[0].(int)
	}
	if len(unreadCountAndNotifications) > 1 {
		notifications = unreadCountAndNotifications[1].([]models.UserNotification)
	}

	// pass current request path so templates can mark active nav items
	base := views.Layout(content, userRole, c.Path(), unreadCount, notifications)
	return renderComponent(c, base)
}

func renderComponent(c *fiber.Ctx, component templ.Component) error {
	// Preserve the status code if it was already set
	statusCode := c.Response().StatusCode()
	if statusCode == 0 {
		statusCode = fiber.StatusOK
	}

	handler := adaptor.HTTPHandler(templ.Handler(component))
	err := handler(c)

	// Restore the status code after rendering
	c.Status(statusCode)
	return err
}

func getUserRole(c *fiber.Ctx) (string, error) {
	sessionToken := c.Cookies(sessionTokenCookie)
	if sessionToken == "" {
		return "", nil
	}

	userName, err := models.ValidateSessionToken(sessionToken)
	if err != nil {
		return "", fmt.Errorf("invalid session token")
	}

	user, err := models.FindUserByUsername(userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %s", userName)
	}
	if user == nil {
		return "", fmt.Errorf("user not found: %s", userName)
	}

	return user.Role, nil
}

// handleErrorWithStatus renders an error view with a custom HTTP status code
func handleErrorWithStatus(c *fiber.Ctx, err error, status int) error {
	c.Status(status)
	return handleView(c, views.ErrorWithStatus(status, err.Error()))
}

// filterMediaByAccessibleLibraries filters a slice of media to only include those with chapters in accessible libraries
func filterMediaByAccessibleLibraries(media []models.Media, librarySet map[string]struct{}) []models.Media {
	if len(librarySet) == 0 {
		return []models.Media{}
	}

	filtered := make([]models.Media, 0, len(media))
	for _, m := range media {
		// Check if media has chapters in accessible libraries
		hasAccess := false
		chapters, err := models.GetChapters(m.Slug)
		if err == nil {
			for _, chapter := range chapters {
				if _, ok := librarySet[chapter.LibrarySlug]; ok {
					hasAccess = true
					break
				}
			}
		}
		if hasAccess {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// NotificationStatus represents the status levels for notifications
type NotificationStatus string

const (
	// StatusSuccess indicates a successful operation
	StatusSuccess NotificationStatus = "success"
	// StatusWarning indicates a warning or non-critical error
	StatusWarning NotificationStatus = "warning"
	// StatusDestructive indicates a critical error or failure
	StatusDestructive NotificationStatus = "destructive"
	// StatusInfo indicates informational messages
	StatusInfo NotificationStatus = "info"
)

// GetNotificationStatusForHTTPStatus maps HTTP status codes to notification statuses
func GetNotificationStatusForHTTPStatus(statusCode int) NotificationStatus {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return StatusSuccess
	case statusCode >= 400 && statusCode < 500:
		if statusCode == 401 || statusCode == 403 {
			return StatusDestructive
		}
		return StatusWarning
	case statusCode >= 500:
		return StatusDestructive
	default:
		return StatusInfo
	}
}

// GetNotificationStatusForError maps common error types to notification statuses
func GetNotificationStatusForError(err error) NotificationStatus {
	if err == nil {
		return StatusSuccess
	}

	// This could be expanded to check for specific error types
	// For now, default to destructive for any error
	return StatusDestructive
}
