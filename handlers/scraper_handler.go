package handlers

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
)

// ScraperFormData represents form data for creating/updating scraper scripts
type ScraperFormData struct {
	Name             string `json:"name" form:"name"`
	Language         string `json:"language" form:"language"`
	Schedule         string `json:"schedule" form:"schedule"`
	IndexLibrarySlug string `json:"index_library_slug" form:"index_library_slug"`
	VariableName     any    `json:"variable_name" form:"variable_name"`
	VariableValue    any    `json:"variable_value" form:"variable_value"`
	ScriptPath       string `json:"script_path" form:"script_path"`
	RequirementsPath string `json:"requirements_path" form:"requirements_path"`
}

// normalizeToStringSlice converts interface{} to []string, handling both single values and arrays
func normalizeToStringSlice(data any) []string {
	if data == nil {
		return []string{}
	}

	switch v := data.(type) {
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result
	case []string:
		return v
	case string:
		return []string{v}
	default:
		return []string{}
	}
}

// extractVariablesFromForm extracts variable key-value pairs from form data
// Variables should be submitted as: variable_name=<key> and variable_value=<value> (paired in order)
func extractVariablesFromForm(formData ScraperFormData) map[string]string {
	variables := make(map[string]string)

	// Normalize the interface{} fields to []string
	names := normalizeToStringSlice(formData.VariableName)
	values := normalizeToStringSlice(formData.VariableValue)

	// Pair them up
	for i := range names {
		name := string(names[i])
		name = strings.TrimSpace(name)
		if name != "" {
			value := ""
			if i < len(values) {
				value = string(values[i])
			}
			variables[name] = value
		}
	}

	return variables
}

// HandleScraper renders the scraper page with all scripts
func HandleScraper(c *fiber.Ctx) error {
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	activeID := int64(0)
	if len(scripts) > 0 {
		activeID = scripts[0].ID
	}
	return handleView(c, views.Scraper(scripts, activeID))
}

// HandleScraperScriptDetail renders a specific script for editing
func HandleScraperScriptDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if script == nil {
		return sendNotFoundError(c, ErrScraperScriptNotFound)
	}

	return handleView(c, views.ScraperForm(script, "put", true))
}

// HandleScraperScriptCreate creates a new script
func HandleScraperScriptCreate(c *fiber.Ctx) error {
	var formData ScraperFormData

	// Check if this is a JSON request (HTMX form-json)
	contentType := c.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse as JSON first
		if err := c.BodyParser(&formData); err != nil {
			return sendBadRequestError(c, ErrBadRequest)
		}
	} else {
		// Parse as traditional form data
		formData.Name = c.FormValue("name")
		formData.Language = c.FormValue("language")
		formData.Schedule = c.FormValue("schedule")
		formData.IndexLibrarySlug = c.FormValue("index_library_slug")
		formData.ScriptPath = c.FormValue("script_path")
		formData.RequirementsPath = c.FormValue("requirements_path")

		// For traditional form data, we need to manually collect arrays
		varNames := c.Request().PostArgs().PeekMulti("variable_name")
		if len(varNames) > 0 {
			names := make([]string, len(varNames))
			for i, v := range varNames {
				names[i] = string(v)
			}
			formData.VariableName = names
		}

		varValues := c.Request().PostArgs().PeekMulti("variable_value")
		if len(varValues) > 0 {
			values := make([]string, len(varValues))
			for i, v := range varValues {
				values[i] = string(v)
			}
			formData.VariableValue = values
		}
	}

	name := strings.TrimSpace(formData.Name)
	language := formData.Language
	schedule := strings.TrimSpace(formData.Schedule)

	// Validate input
	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	// Validate that script path is provided
	if formData.ScriptPath == "" {
		return sendValidationError(c, ErrRequiredField) // TODO: define proper error
	}

	// Infer language from script path extension if not provided
	if language == "" {
		if strings.HasSuffix(formData.ScriptPath, ".py") {
			language = "python"
		} else if strings.HasSuffix(formData.ScriptPath, ".sh") {
			language = "bash"
		} else {
			return sendValidationError(c, ErrScraperScriptInvalid) // Unsupported file extension
		}
	}

	if schedule == "" {
		schedule = "0 0 * * *" // Default to daily at midnight
	}

	// Extract variables from form
	variables := extractVariablesFromForm(formData)

	// Handle index library slug
	var indexLibrarySlug *string
	if strings.TrimSpace(formData.IndexLibrarySlug) != "" {
		indexLibrarySlug = &formData.IndexLibrarySlug
	}

	// Handle script path
	var scriptPath *string
	if strings.TrimSpace(formData.ScriptPath) != "" {
		scriptPath = &formData.ScriptPath
	}

	// Handle requirements path
	var requirementsPath *string
	if strings.TrimSpace(formData.RequirementsPath) != "" {
		requirementsPath = &formData.RequirementsPath
	}

	if _, err := models.CreateScraperScript(name, language, schedule, variables, indexLibrarySlug, scriptPath, requirementsPath); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get all scripts to update the table
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Return the updated table
	return handleView(c, views.ScraperTable(scripts))
}

// HandleScraperScriptUpdate updates an existing script
func HandleScraperScriptUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	var formData ScraperFormData

	// Check if this is a JSON request (HTMX form-json)
	contentType := c.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Parse as JSON first
		if err := c.BodyParser(&formData); err != nil {
			return sendBadRequestError(c, ErrBadRequest)
		}
	} else {
		// Parse as traditional form data
		formData.Name = c.FormValue("name")
		formData.Language = c.FormValue("language")
		formData.Schedule = c.FormValue("schedule")
		formData.IndexLibrarySlug = c.FormValue("index_library_slug")
		formData.ScriptPath = c.FormValue("script_path")
		formData.RequirementsPath = c.FormValue("requirements_path")

		// For traditional form data, we need to manually collect arrays
		varNames := c.Request().PostArgs().PeekMulti("variable_name")
		if len(varNames) > 0 {
			names := make([]string, len(varNames))
			for i, v := range varNames {
				names[i] = string(v)
			}
			formData.VariableName = names
		}

		varValues := c.Request().PostArgs().PeekMulti("variable_value")
		if len(varValues) > 0 {
			values := make([]string, len(varValues))
			for i, v := range varValues {
				values[i] = string(v)
			}
			formData.VariableValue = values
		}
	}

	name := strings.TrimSpace(formData.Name)
	language := formData.Language
	schedule := strings.TrimSpace(formData.Schedule)

	// Validate input
	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	// Validate that script path is provided
	if formData.ScriptPath == "" {
		return sendValidationError(c, ErrRequiredField) // TODO: define proper error
	}

	// Infer language from script path extension if not provided
	if language == "" {
		if strings.HasSuffix(formData.ScriptPath, ".py") {
			language = "python"
		} else if strings.HasSuffix(formData.ScriptPath, ".sh") {
			language = "bash"
		} else {
			return sendValidationError(c, ErrScraperScriptInvalid) // Unsupported file extension
		}
	}

	if schedule == "" {
		schedule = "0 0 * * *"
	}

	// Extract variables from form
	variables := extractVariablesFromForm(formData)

	// Handle index library slug
	var indexLibrarySlug *string
	if strings.TrimSpace(formData.IndexLibrarySlug) != "" {
		indexLibrarySlug = &formData.IndexLibrarySlug
	}

	// Handle script path
	var scriptPath *string
	if strings.TrimSpace(formData.ScriptPath) != "" {
		scriptPath = &formData.ScriptPath
	}

	// Handle requirements path
	var requirementsPath *string
	if strings.TrimSpace(formData.RequirementsPath) != "" {
		requirementsPath = &formData.RequirementsPath
	}

	script, err := models.UpdateScraperScript(id, name, language, schedule, variables, indexLibrarySlug, scriptPath, requirementsPath)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// For HTMX requests, return a simple success response instead of re-rendering the form
	if c.Get("HX-Request") == "true" {
		triggerNotification(c, "Script saved successfully", "success")
		return c.SendStatus(fiber.StatusOK)
	}

	return handleView(c, views.ScraperScriptEditor(script))
}

// HandleScraperScriptDelete deletes a script
func HandleScraperScriptDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	if err := models.DeleteScraperScript(id); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get updated scripts list and return the table
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return handleView(c, views.ScraperTable(scripts))
}

// HandleScraperScriptCancel cancels a running script
func HandleScraperScriptCancel(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}
	if err := scheduler.CancelScriptExecution(id); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	return c.SendStatus(fiber.StatusOK)
}

// HandleScraperScriptDisable disables a script
func HandleScraperScriptDisable(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if script == nil {
		return sendNotFoundError(c, ErrScraperScriptNotFound)
	}

	if err := models.EnableScraperScript(id, false); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// For HTMX requests, return the updated toggle button
	if c.Get("HX-Request") == "true" {
		triggerNotification(c, "Script disabled", "success")

		// Get the updated script with new state
		updated, err := models.GetScraperScript(id)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		return handleView(c, views.ScraperTableRow(updated))
	}

	// Return updated script
	updated, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return handleView(c, views.ScraperScriptRow(updated))
}

// HandleScraperScriptEnable enables a script
func HandleScraperScriptEnable(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if script == nil {
		return sendNotFoundError(c, ErrScraperScriptNotFound)
	}

	if err := models.EnableScraperScript(id, true); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// For HTMX requests, return the updated toggle button
	if c.Get("HX-Request") == "true" {
		triggerNotification(c, "Script enabled", "success")

		// Get the updated script with new state
		updated, err := models.GetScraperScript(id)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		return handleView(c, views.ScraperTableRow(updated))
	}

	// Return updated script
	updated, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return handleView(c, views.ScraperScriptRow(updated))
}

// HandleScraperScriptRun manually runs a script
func HandleScraperScriptRun(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if script == nil {
		return sendNotFoundError(c, ErrScraperScriptNotFound)
	}

	// Parse form data to get variable overrides
	var formData ScraperFormData

	// Try to parse as form data first
	varNames := c.Request().PostArgs().PeekMulti("variable_name")
	if len(varNames) > 0 {
		names := make([]string, len(varNames))
		for i, v := range varNames {
			names[i] = string(v)
		}
		formData.VariableName = names
	}

	varValues := c.Request().PostArgs().PeekMulti("variable_value")
	if len(varValues) > 0 {
		values := make([]string, len(varValues))
		for i, v := range varValues {
			values[i] = string(v)
		}
		formData.VariableValue = values
	}

	// If no variables found, try JSON parsing
	namesSlice := normalizeToStringSlice(formData.VariableName)
	if len(namesSlice) == 0 {
		if err := c.BodyParser(&formData); err == nil {
			// JSON parsing worked, use it
		}
	}

	// Start with stored variables, then override with form values
	variables := make(map[string]string)
	maps.Copy(variables, script.Variables)

	// Override with form values if provided
	formVariables := extractVariablesFromForm(formData)
	maps.Copy(variables, formVariables)

	// Start execution via shared executor (creates DB log and streams logs)
	if _, err := scheduler.StartScriptExecution(script, variables, true); err != nil {
		return sendInternalServerError(c, ErrScraperExecutionFailed, err)
	}

	// For HTMX requests, return a simple success response instead of re-rendering the form
	if c.Get("HX-Request") == "true" {
		triggerNotification(c, "Script execution started", "success")
		return c.SendStatus(fiber.StatusOK)
	}

	// Return updated script with output
	updated, err := models.GetScraperScript(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return handleView(c, views.ScraperScriptEditor(updated))
}

// Execution implementation moved to `executor` package; handlers call into that shared package.

// Note: Go template execution was deprecated; only bash scripts are supported.

// HandleScraperScriptsList returns the list of scripts as a fragment (for HTMX updates)
func HandleScraperScriptsList(c *fiber.Ctx) error {
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	return handleView(c, views.ScraperScriptsList(scripts))
}

// HandleScraperNewForm returns the form for creating a new script
func HandleScraperNewForm(c *fiber.Ctx) error {
	// Create an empty script for the form
	emptyScript := &models.ScraperScript{
		ID:        0,
		Name:      "",
		Language:  "", // Will be inferred from script path
		Schedule:  "0 0 * * *",
		Variables: make(map[string]string),
		Enabled:   true,
	}
	return handleView(c, views.ScraperForm(emptyScript, "post", false))
}

// HandleScraperLogs returns the logs view for a script
func HandleScraperLogs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidScriptID)
	}

	// Get pagination parameters
	pageStr := c.Query("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	perPage := 10 // Increased for modal view
	offset := (page - 1) * perPage

	// Get logs for current page
	logs, err := models.ListScraperLogs(id, perPage, offset)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get total count for pagination
	totalCount, err := models.CountScraperLogs(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Calculate pagination info
	totalPages := (totalCount + perPage - 1) / perPage // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := map[string]any{
		"current_page": page,
		"total_pages":  totalPages,
		"per_page":     perPage,
		"total_count":  totalCount,
		"has_prev":     page > 1,
		"has_next":     page < totalPages,
		"prev_page":    page - 1,
		"next_page":    page + 1,
		"script_id":    id,
	}

	// Return just the logs panel for modal
	return handleView(c, views.ScraperLogsModalContent(logs, pagination, id))
}

// HandleScraperVariableAdd returns an empty variable input row for HTMX inserts
func HandleScraperVariableAdd(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return handleView(c, views.Variable("", "", false))
}

// HandleScraperVariableRemove acknowledges variable removal requests without returning content
func HandleScraperVariableRemove(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return c.SendString("")
}

// HandleScraperCancelEdit resets the scraper form to its default state.
func HandleScraperCancelEdit(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return handleView(c, views.ScraperForm(&models.ScraperScript{}, "post", false))
}

// HandleScraperUpdateScriptPath returns updated language-dependent sections based on script path extension
func HandleScraperUpdateScriptPath(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	// Get the script path from the request
	scriptPath := c.Query("script_path", "")

	// Infer language from script path extension
	language := ""
	if strings.HasSuffix(scriptPath, ".py") {
		language = "python"
	} else if strings.HasSuffix(scriptPath, ".sh") {
		language = "bash"
	}

	// Create a mock script object with the inferred language
	script := &models.ScraperScript{
		Language: language,
	}

	// Return the updated language-dependent sections
	return handleView(c, views.LanguageDependentSections(script))
}

// HandleScraperLogDelete deletes a specific execution log
func HandleScraperLogDelete(c *fiber.Ctx) error {
	logID, err := strconv.ParseInt(c.Params("logId"), 10, 64)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidLogID)
	}

	// Get the log to verify it exists and get the script ID
	logEntry, err := models.GetScraperLog(logID)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Delete the log
	if err := models.DeleteScraperLog(logID); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// If this is an HTMX request, return a success notification
	if isHTMXRequest(c) {
		triggerNotification(c, "Log deleted successfully", "success")
		return c.SendString("")
	}

	// Otherwise, redirect back to the logs page
	return c.Redirect(fmt.Sprintf("/admin/scraper/%d/logs", logEntry.ScriptID))
}
