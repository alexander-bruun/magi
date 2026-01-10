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
	Script           string `json:"script" form:"script"`
	Language         string `json:"language" form:"language"`
	Schedule         string `json:"schedule" form:"schedule"`
	SharedScript     string `json:"shared_script" form:"shared_script"`
	IndexLibrarySlug string `json:"index_library_slug" form:"index_library_slug"`
	VariableName     any    `json:"variable_name" form:"variable_name"`
	VariableValue    any    `json:"variable_value" form:"variable_value"`
	Package          any    `json:"package" form:"package"`
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

// extractPackagesFromForm extracts package list from form data
// Packages should be submitted as: package=<package_name> (multiple)
func extractPackagesFromForm(formData ScraperFormData) []string {
	var packages []string

	// Normalize the interface{} field to []string
	pkgNames := normalizeToStringSlice(formData.Package)

	for _, pkg := range pkgNames {
		pkgName := strings.TrimSpace(string(pkg))
		if pkgName != "" {
			packages = append(packages, pkgName)
		}
	}

	return packages
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

	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return handleView(c, views.ScraperEditorWithUpdatedTabs(script, scripts))
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
		formData.Script = c.FormValue("script")
		formData.Language = c.FormValue("language")
		formData.Schedule = c.FormValue("schedule")
		formData.SharedScript = c.FormValue("shared_script")
		formData.IndexLibrarySlug = c.FormValue("index_library_slug")

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

		packages := c.Request().PostArgs().PeekMulti("package")
		if len(packages) > 0 {
			pkgs := make([]string, len(packages))
			for i, v := range packages {
				pkgs[i] = string(v)
			}
			formData.Package = pkgs
		}
	}

	name := strings.TrimSpace(formData.Name)
	scriptContent := formData.Script
	language := formData.Language
	schedule := strings.TrimSpace(formData.Schedule)
	sharedScriptContent := formData.SharedScript

	// Validate input
	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	if err := models.ValidateScript(scriptContent, language); err != nil {
		return sendValidationError(c, ErrScraperScriptInvalid)
	}

	if schedule == "" {
		schedule = "0 0 * * *" // Default to daily at midnight
	}

	// Extract variables from form
	variables := extractVariablesFromForm(formData)

	// Extract packages from form
	extractedPackages := extractPackagesFromForm(formData)

	// Handle shared script
	var sharedScript *string
	if sharedScriptContent != "" {
		sharedScript = &sharedScriptContent
	}

	// Handle index library slug
	var indexLibrarySlug *string
	if strings.TrimSpace(formData.IndexLibrarySlug) != "" {
		indexLibrarySlug = &formData.IndexLibrarySlug
	}

	script, err := models.CreateScraperScript(name, scriptContent, language, schedule, variables, extractedPackages, sharedScript, indexLibrarySlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get all scripts to update the tabs
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Return the new script in editor view with updated tabs
	return handleView(c, views.ScraperEditorWithUpdatedTabs(script, scripts))
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
		formData.Script = c.FormValue("script")
		formData.Language = c.FormValue("language")
		formData.Schedule = c.FormValue("schedule")
		formData.SharedScript = c.FormValue("shared_script")
		formData.IndexLibrarySlug = c.FormValue("index_library_slug")

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

		packages := c.Request().PostArgs().PeekMulti("package")
		if len(packages) > 0 {
			pkgs := make([]string, len(packages))
			for i, v := range packages {
				pkgs[i] = string(v)
			}
			formData.Package = pkgs
		}
	}

	name := strings.TrimSpace(formData.Name)
	scriptContent := formData.Script
	language := formData.Language
	schedule := strings.TrimSpace(formData.Schedule)
	sharedScriptContent := formData.SharedScript

	// Validate input
	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	if err := models.ValidateScript(scriptContent, language); err != nil {
		return sendValidationError(c, ErrScraperScriptInvalid)
	}

	if schedule == "" {
		schedule = "0 0 * * *"
	}

	// Extract variables from form
	variables := extractVariablesFromForm(formData)

	// Extract packages from form
	extractedPackages := extractPackagesFromForm(formData)

	// Handle shared script
	var sharedScript *string
	if sharedScriptContent != "" {
		sharedScript = &sharedScriptContent
	}

	// Handle index library slug
	var indexLibrarySlug *string
	if strings.TrimSpace(formData.IndexLibrarySlug) != "" {
		indexLibrarySlug = &formData.IndexLibrarySlug
	}

	script, err := models.UpdateScraperScript(id, name, scriptContent, language, schedule, variables, extractedPackages, sharedScript, indexLibrarySlug)
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

	// Redirect to scraper page to refresh the entire page
	c.Set("HX-Redirect", "/admin/scraper")
	return c.SendStatus(fiber.StatusNoContent)
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

// HandleScraperScriptToggle enables or disables a script
func HandleScraperScriptToggle(c *fiber.Ctx) error {
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

	if err := models.EnableScraperScript(id, !script.Enabled); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// For HTMX requests, return a simple success response instead of re-rendering the form
	if c.Get("HX-Request") == "true" {
		newState := !script.Enabled
		message := "Script disabled"
		if newState {
			message = "Script enabled"
		}
		triggerNotification(c, message, "success")
		return c.SendStatus(fiber.StatusOK)
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
		Script:    "",
		Language:  "bash", // Default to bash for new scripts
		Schedule:  "0 0 * * *",
		Variables: make(map[string]string),
		Packages:  []string{},
		Enabled:   true,
	}
	return handleView(c, views.ScraperScriptEditor(emptyScript))
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
	perPage := 5
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

	return handleView(c, views.ScraperLogsPanelWithPagination(logs, pagination))
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

// HandleScraperPackageAdd returns an empty package input row for HTMX inserts
func HandleScraperPackageAdd(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return handleView(c, views.Package("", false))
}

// HandleScraperPackageRemove acknowledges package removal requests without returning content
func HandleScraperPackageRemove(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return c.SendString("")
}

// HandleScraperUpdateLanguage returns updated language-dependent sections for HTMX requests
func HandleScraperUpdateLanguage(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	// Get the language from the request
	language := c.Query("language", "python")
	if language == "" {
		language = "python"
	}

	// Create a mock script object with the selected language
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
