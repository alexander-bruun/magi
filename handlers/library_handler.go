package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/robfig/cron/v3"
)

// LibraryFormData holds form data for creating/updating libraries
type LibraryFormData struct {
	Name             string   `form:"name"`
	Description      string   `form:"description"`
	Cron             string   `form:"cron"`
	Folders          []string `form:"folders"`
	MetadataProvider string   `form:"metadata_provider"`
	Enabled          bool     `form:"enabled"`
}

// validateLibraryFormData validates the parsed library form data
func validateLibraryFormData(formData LibraryFormData, currentSlug string) error {
	// Validate cron expression
	if formData.Cron != "" {
		_, err := cron.ParseStandard(formData.Cron)
		if err != nil {
			return fmt.Errorf("Invalid cron expression (must be 5 fields: minute hour day month weekday)")
		}
	}

	// Validate folders exist
	for _, folder := range formData.Folders {
		if folder == "" {
			continue // Skip empty folders
		}
		info, err := os.Stat(folder)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Folder does not exist: %s", folder)
			}
			return fmt.Errorf("Cannot access folder %s: %v", folder, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("Path is not a directory: %s", folder)
		}
	}

	// Check for duplicate folders across libraries
	if err := models.CheckDuplicateFolders(formData.Folders, currentSlug); err != nil {
		return err
	}

	return nil
}

// CreateLibrary creates a new library and returns all libraries
func CreateLibrary(library models.Library) ([]models.Library, error) {
	if err := models.CreateLibrary(library); err != nil {
		return nil, err
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return nil, err
	}

	return libraries, nil
}

// GetLibraries retrieves all libraries
func GetLibraries() ([]models.Library, error) {
	return models.GetLibraries()
}

// HandleLibraries renders the libraries dashboard with the current library list.
func HandleLibraries(c *fiber.Ctx) error {
	return handleView(c, views.Libraries())
}

func renderLibraryTable(libraries []models.Library) (string, error) {
	var buf bytes.Buffer
	err := views.LibraryTable(libraries).Render(context.Background(), &buf)
	if err != nil {
		return "", fmt.Errorf("error rendering table: %w", err)
	}
	return buf.String(), nil
}

func setCommonHeaders(c *fiber.Ctx) {
	c.Response().Header.Set("Content-Type", "text/html")
}

// HandleCreateLibrary persists a new library and returns the refreshed table markup.
func HandleCreateLibrary(c *fiber.Ctx) error {
	var formData LibraryFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	// Validate the form data
	if err := validateLibraryFormData(formData, ""); err != nil {
		return sendValidationError(c, err.Error())
	}

	// Convert form data to Library model
	library := models.Library{
		Name:        formData.Name,
		Description: formData.Description,
		Cron:        formData.Cron,
		Folders:     formData.Folders,
	}

	// Handle metadata_provider: empty string means use global (NULL in DB)
	if formData.MetadataProvider != "" {
		library.MetadataProvider = sql.NullString{String: formData.MetadataProvider, Valid: true}
	} else {
		library.MetadataProvider = sql.NullString{Valid: false}
	}

	libraries, err := CreateLibrary(library)
	if err != nil {
		return sendInternalServerError(c, ErrLibraryCreateFailed, err)
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Render a fresh form
	var formBuf bytes.Buffer
	emptyLibrary := models.Library{}
	err = views.LibraryForm(emptyLibrary, "post", false).Render(context.Background(), &formBuf)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	formContent := formBuf.String()

	// Combine table and form with OOB swap for form
	responseContent := tableContent + `<div id="library-form" hx-swap-oob="innerHTML">` + formContent + `</div>`

	triggerNotification(c, "Library created successfully", "success")
	setCommonHeaders(c)
	return c.SendString(responseContent)
}

// HandleDeleteLibrary removes an existing library and responds with the updated table fragment.
func HandleDeleteLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	// Delete the library asynchronously to avoid blocking the response
	go func() {
		if err := models.DeleteLibrary(slug); err != nil {
			log.Errorf("Error deleting library %s: %v", slug, err)
		}
	}()

	// Immediately return the updated table without the deleted library
	libraries, err := models.GetLibraries()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Filter out the library being deleted from the response
	filteredLibraries := make([]models.Library, 0, len(libraries))
	for _, lib := range libraries {
		if lib.Slug != slug {
			filteredLibraries = append(filteredLibraries, lib)
		}
	}

	tableContent, err := renderLibraryTable(filteredLibraries)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Library deleted successfully", "success")
	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleUpdateLibrary updates library information and returns the refreshed listing.
func HandleUpdateLibrary(c *fiber.Ctx) error {
	var formData LibraryFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	slug := c.Params("slug")

	// Validate the form data
	if err := validateLibraryFormData(formData, slug); err != nil {
		return sendValidationError(c, err.Error())
	}

	// Convert form data to Library model
	library := models.Library{
		Name:        formData.Name,
		Description: formData.Description,
		Cron:        formData.Cron,
		Folders:     formData.Folders,
	}

	// Handle metadata_provider: empty string means use global (NULL in DB)
	if formData.MetadataProvider != "" {
		library.MetadataProvider = sql.NullString{String: formData.MetadataProvider, Valid: true}
	} else {
		library.MetadataProvider = sql.NullString{Valid: false}
	}

	library.Slug = slug
	log.Infof("Updating library: %s", library.Slug)

	if err := models.UpdateLibrary(&library); err != nil {
		return sendInternalServerError(c, ErrLibraryUpdateFailed, err)
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerCustomNotification(c, "showNotification", map[string]interface{}{
		"message": "Library updated successfully",
		"status":  "success",
	})
	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleEditLibrary renders the inline edit form for the requested library.
func HandleEditLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return sendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return sendNotFoundError(c, ErrLibraryNotFound)
	}

	var buf bytes.Buffer
	err = views.LibraryForm(*library, "put", true).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	setCommonHeaders(c)
	return c.SendString(fmt.Sprintf(`<div id="library-form">%s</div>`, buf.String()))
}

// HandleScanLibrary triggers an immediate indexing pass for the specified library.
func HandleScanLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return sendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return sendNotFoundError(c, ErrLibraryNotFound)
	}

	// Create a temporary Indexer for this library and run the job so we
	// preserve the same logging and lifecycle as scheduled jobs.
	idx := scheduler.NewIndexer(*library)
	// RunIndexingJob will process all folders for the library.
	if ran := idx.RunIndexingJob(); !ran {
		triggerNotification(c, "Indexing already in progress for this library", "warning")
		return c.SendString("")
	}

	return c.SendString(`<uk-icon icon="RefreshCw"></uk-icon>`)
}

// HandleAddFolder returns an empty folder form fragment for HTMX inserts.
func HandleAddFolder(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/libraries")
	}

	return handleView(c, views.Folder(""))
}

// HandleRemoveFolder acknowledges folder removal requests without returning content.
func HandleRemoveFolder(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/libraries")
	}

	return c.SendString("")
}

// HandleCancelEdit resets the library form to its default state.
func HandleCancelEdit(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect("/admin/libraries")
	}

	var buf bytes.Buffer
	err := views.LibraryForm(models.Library{}, "post", false).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	c.Response().Header.Set("Content-Type", "text/html")
	return c.SendString(buf.String())
}

// getSubdirectories returns the names of all subdirectories in a given path
func getSubdirectories(path string) ([]string, error) {
	var subdirs []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, entry.Name())
		}
	}

	return subdirs, nil
}

// FileEntry represents a file or directory entry
type FileEntry struct {
	Name  string
	IsDir bool
	Path  string
}

// HandleBrowseDirectory returns a list of files and directories for the file explorer
func HandleBrowseDirectory(c *fiber.Ctx) error {
	path := c.Query("path", "/")
	if path == "" {
		path = "/"
	}

	// For security, we might want to restrict to certain directories, but for now allow all
	entries, err := os.ReadDir(path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var fileEntries []FileEntry
	for _, entry := range entries {
		fileEntries = append(fileEntries, FileEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Path:  filepath.Join(path, entry.Name()),
		})
	}

	return c.JSON(fileEntries)
}
