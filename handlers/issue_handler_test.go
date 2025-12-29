package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandleReportIssue(t *testing.T) {
	app := fiber.New()
	app.Get("/report-issue", HandleReportIssue)

	req := httptest.NewRequest("GET", "/report-issue", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleCreateIssue(t *testing.T) {
	app := fiber.New()
	app.Post("/report-issue", HandleCreateIssue)

	// Test with HTMX headers
	req := httptest.NewRequest("POST", "/report-issue", strings.NewReader("title=Test+Issue&description=Test+description&category=bug"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleIssuesAdmin(t *testing.T) {
	app := fiber.New()
	app.Get("/admin/issues", HandleIssuesAdmin)

	// Test regular request (should return full page)
	req := httptest.NewRequest("GET", "/admin/issues", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Test HTMX request with status filter
	req2 := httptest.NewRequest("GET", "/admin/issues?status=open", nil)
	req2.Header.Set("HX-Request", "true")
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)

	// Test HTMX request with category filter
	req3 := httptest.NewRequest("GET", "/admin/issues?category=bug", nil)
	req3.Header.Set("HX-Request", "true")
	resp3, err := app.Test(req3)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp3.StatusCode)

	// Test HTMX request with both status and category filters
	req4 := httptest.NewRequest("GET", "/admin/issues?status=open&category=bug", nil)
	req4.Header.Set("HX-Request", "true")
	resp4, err := app.Test(req4)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp4.StatusCode)
}

func TestIssueModelOperations(t *testing.T) {
	// Test creating an issue
	issue := models.Issue{
		Title:       "Test Issue",
		Description: "Test description",
		Status:      "open",
		Priority:    "medium",
		Category:    "bug",
		UserAgent:   "Test Agent",
		URL:         "https://example.com",
	}

	err := models.CreateIssue(issue)
	assert.NoError(t, err)

	// Test retrieving issues
	issues, err := models.GetIssues("", "", 10, 0)
	assert.NoError(t, err)
	assert.Greater(t, len(issues), 0)

	// Verify the timestamps are properly converted
	found := false
	for _, i := range issues {
		if i.Title == "Test Issue" {
			found = true
			assert.NotZero(t, i.CreatedAt)
			assert.NotZero(t, i.UpdatedAt)
			assert.Equal(t, "Test Issue", i.Title)
			assert.Equal(t, "Test description", i.Description)
			assert.Equal(t, "open", i.Status)
			assert.Equal(t, "medium", i.Priority)
			assert.Equal(t, "bug", i.Category)
			break
		}
	}
	assert.True(t, found, "Created issue should be found in the list")

	// Test getting issue by ID
	if len(issues) > 0 {
		issue, err := models.GetIssueByID(issues[0].ID)
		assert.NoError(t, err)
		assert.NotNil(t, issue)
		assert.NotZero(t, issue.CreatedAt)
		assert.NotZero(t, issue.UpdatedAt)
	}

	// Test updating issue status
	if len(issues) > 0 {
		err := models.UpdateIssueStatus(issues[0].ID, "closed")
		assert.NoError(t, err)

		// Verify the status was updated
		updatedIssue, err := models.GetIssueByID(issues[0].ID)
		assert.NoError(t, err)
		assert.Equal(t, "closed", updatedIssue.Status)
		assert.NotNil(t, updatedIssue.ResolvedAt)
	}
}
