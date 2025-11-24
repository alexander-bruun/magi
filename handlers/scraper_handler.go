package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
)

// (execution/cancel tracking moved to `executor` package)

// extractVariablesFromForm extracts variable key-value pairs from form data
// Variables should be submitted as: variable_name=<key> and variable_value=<value> (paired in order)
func extractVariablesFromForm(c *fiber.Ctx) map[string]string {
	variables := make(map[string]string)
	
	// Get all variable names and values
	names := c.Request().PostArgs().PeekMulti("variable_name")
	values := c.Request().PostArgs().PeekMulti("variable_value")
	
	// Pair them up
	for i := 0; i < len(names); i++ {
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
func extractPackagesFromForm(c *fiber.Ctx) []string {
	var packages []string
	
	// Get all package names
	pkgNames := c.Request().PostArgs().PeekMulti("package")
	
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
		return handleError(c, err)
	}
	return HandleView(c, views.Scraper(scripts))
}

// HandleScraperScriptDetail renders a specific script for editing
func HandleScraperScriptDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return handleError(c, err)
	}
	if script == nil {
		return handleError(c, fmt.Errorf("script not found"))
	}

	return HandleView(c, views.ScraperScriptEditor(script))
}

// HandleScraperScriptCreate creates a new script
func HandleScraperScriptCreate(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	scriptContent := c.FormValue("script")
	language := c.FormValue("language")
	schedule := strings.TrimSpace(c.FormValue("schedule"))

	// Validate input
	if name == "" {
		return handleError(c, fmt.Errorf("script name is required"))
	}

	if err := models.ValidateScript(scriptContent, language); err != nil {
		return handleError(c, err)
	}

	if schedule == "" {
		schedule = "0 0 * * *" // Default to daily at midnight
	}

	// Extract variables from form
	variables := extractVariablesFromForm(c)

	// Extract packages from form
	packages := extractPackagesFromForm(c)

	script, err := models.CreateScraperScript(name, scriptContent, language, schedule, variables, packages)
	if err != nil {
		return handleError(c, err)
	}

	// Return the new script in editor view
	return HandleView(c, views.ScraperScriptEditor(script))
}

// HandleScraperScriptUpdate updates an existing script
func HandleScraperScriptUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	name := strings.TrimSpace(c.FormValue("name"))
	scriptContent := c.FormValue("script")
	language := c.FormValue("language")
	schedule := strings.TrimSpace(c.FormValue("schedule"))

	// Validate input
	if name == "" {
		return handleError(c, fmt.Errorf("script name is required"))
	}

	if err := models.ValidateScript(scriptContent, language); err != nil {
		return handleError(c, err)
	}

	if schedule == "" {
		schedule = "0 0 * * *"
	}

	// Extract variables from form
	variables := extractVariablesFromForm(c)

	// Extract packages from form
	packages := extractPackagesFromForm(c)

	script, err := models.UpdateScraperScript(id, name, scriptContent, language, schedule, variables, packages)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ScraperScriptEditor(script))
}

// HandleScraperScriptDelete deletes a script
func HandleScraperScriptDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	if err := models.DeleteScraperScript(id); err != nil {
		return handleError(c, err)
	}

	// Redirect to scraper page to refresh the entire page
	c.Set("HX-Redirect", "/admin/scraper")
	return c.SendStatus(fiber.StatusNoContent)
}

// HandleScraperScriptCancel cancels a running script
func HandleScraperScriptCancel(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}
	if err := executor.CancelScriptExecution(id); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusOK)
}

// HandleScraperScriptToggle enables or disables a script
func HandleScraperScriptToggle(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return handleError(c, err)
	}
	if script == nil {
		return handleError(c, fmt.Errorf("script not found"))
	}

	if err := models.EnableScraperScript(id, !script.Enabled); err != nil {
		return handleError(c, err)
	}

	// Return updated script
	updated, err := models.GetScraperScript(id)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ScraperScriptRow(updated))
}

// HandleScraperScriptRun manually runs a script
func HandleScraperScriptRun(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	script, err := models.GetScraperScript(id)
	if err != nil {
		return handleError(c, err)
	}
	if script == nil {
		return handleError(c, fmt.Errorf("script not found"))
	}

	// Start with stored variables, then override with form values
	variables := make(map[string]string)
	for k, v := range script.Variables {
		variables[k] = v
	}
	
	// Override with form values if provided
	formVariables := extractVariablesFromForm(c)
	for k, v := range formVariables {
		variables[k] = v
	}

	// Start execution via shared executor (creates DB log and streams logs)
	if _, err := executor.StartScriptExecution(script, variables, true); err != nil {
		return handleError(c, err)
	}

	// Return updated script with output
	updated, err := models.GetScraperScript(id)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ScraperScriptEditor(updated))
}

// Execution implementation moved to `executor` package; handlers call into that shared package.

// Note: Go template execution was deprecated; only bash scripts are supported.

// HandleScraperScriptsList returns the list of scripts as a fragment (for HTMX updates)
func HandleScraperScriptsList(c *fiber.Ctx) error {
	scripts, err := models.ListScraperScripts(false)
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.ScraperScriptsList(scripts))
}

// HandleScraperNewForm returns the form for creating a new script
func HandleScraperNewForm(c *fiber.Ctx) error {
	return HandleView(c, views.ScraperScriptForm(nil))
}

// HandleScraperLogs returns the logs view for a script
func HandleScraperLogs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid script id"), fiber.StatusBadRequest)
	}

	logs, err := models.ListScraperLogs(id, 50)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ScraperLogsPanel(logs))
}

// HandleScraperVariableAdd returns an empty variable input row for HTMX inserts
func HandleScraperVariableAdd(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return HandleView(c, views.Variable("", "", false))
}

// HandleScraperVariableRemove acknowledges variable removal requests without returning content
func HandleScraperVariableRemove(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return c.SendString("")
}

// HandleScraperPackageAdd returns an empty package input row for HTMX inserts
func HandleScraperPackageAdd(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return HandleView(c, views.Package("", false))
}

// HandleScraperPackageRemove acknowledges package removal requests without returning content
func HandleScraperPackageRemove(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the scraper page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/scraper")
	}

	return c.SendString("")
}
