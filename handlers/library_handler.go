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
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/robfig/cron/v3"
)

// LibraryFormData holds form data for creating/updating libraries
type LibraryFormData struct {
	Name             string   `form:"name"`
	Description      string   `form:"description"`
	Cron             string   `form:"cron"`
	Folders          []string `form:"folders"`
	MetadataProvider string   `form:"metadata_provider"`
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
func HandleLibraries(c fiber.Ctx) error {
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

func setCommonHeaders(c fiber.Ctx) {
	c.Response().Header.Set("Content-Type", "text/html")
}

// HandleCreateLibrary persists a new library and returns the refreshed table markup.
func HandleCreateLibrary(c fiber.Ctx) error {
	var formData LibraryFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	// Validate the form data
	if err := validateLibraryFormData(formData, ""); err != nil {
		return SendValidationError(c, err.Error())
	}

	// Convert form data to Library model
	library := models.Library{
		Name:        formData.Name,
		Description: formData.Description,
		Cron:        formData.Cron,
		Folders:     formData.Folders,
		Enabled:     true,
	}

	// Handle metadata_provider: empty string means use global (NULL in DB)
	if formData.MetadataProvider != "" {
		library.MetadataProvider = sql.NullString{String: formData.MetadataProvider, Valid: true}
	} else {
		library.MetadataProvider = sql.NullString{Valid: false}
	}

	libraries, err := CreateLibrary(library)
	if err != nil {
		return SendInternalServerError(c, ErrLibraryCreateFailed, err)
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// Render a fresh form
	var formBuf bytes.Buffer
	emptyLibrary := models.Library{}
	err = views.LibraryForm(emptyLibrary, "post", false).Render(context.Background(), &formBuf)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	formContent := formBuf.String()

	// Combine table and form with OOB swap for form
	responseContent := tableContent + `<div id="library-form" hx-swap-oob="innerHTML">` + formContent + `</div>`

	triggerNotification(c, "Library created successfully", "success")
	setCommonHeaders(c)
	return c.SendString(responseContent)
}

// HandleDeleteLibrary removes an existing library and responds with the updated table fragment.
func HandleDeleteLibrary(c fiber.Ctx) error {
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
		return SendInternalServerError(c, ErrInternalServerError, err)
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
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Library deleted successfully", "success")
	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleToggleLibrary toggles the enabled status of a library
func HandleToggleLibrary(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return c.Status(fiber.StatusNotFound).SendString("Library not found")
	}

	// Toggle the enabled status
	library.Enabled = !library.Enabled

	if err := models.UpdateLibrary(library); err != nil {
		return SendInternalServerError(c, ErrLibraryUpdateFailed, err)
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	status := "disabled"
	if library.Enabled {
		status = "enabled"
	}
	triggerNotification(c, fmt.Sprintf("Library %s successfully", status), "success")
	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleUpdateLibrary updates library information and returns the refreshed listing.
func HandleUpdateLibrary(c fiber.Ctx) error {
	var formData LibraryFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	slug := c.Params("slug")

	// Validate the form data
	if err := validateLibraryFormData(formData, slug); err != nil {
		return SendValidationError(c, err.Error())
	}

	// Get the existing library to preserve the Enabled state
	existingLibrary, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// Convert form data to Library model
	library := models.Library{
		Name:        formData.Name,
		Description: formData.Description,
		Cron:        formData.Cron,
		Folders:     formData.Folders,
		Enabled:     existingLibrary.Enabled, // Preserve existing enabled state
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
		return SendInternalServerError(c, ErrLibraryUpdateFailed, err)
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerCustomNotification(c, "showNotification", map[string]interface{}{
		"message": "Library updated successfully",
		"status":  "success",
	})
	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleEditLibrary renders the inline edit form for the requested library.
func HandleEditLibrary(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return SendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return SendNotFoundError(c, ErrLibraryNotFound)
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
func HandleScanLibrary(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return SendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return SendNotFoundError(c, ErrLibraryNotFound)
	}

	// Create a temporary Indexer for this library and run the job so we
	// preserve the same logging and lifecycle as scheduled jobs.
	idx := scheduler.NewIndexer(*library, fileStore)
	// RunIndexingJob will process all folders for the library.
	if ran := idx.RunIndexingJob(); !ran {
		triggerNotification(c, "Indexing already in progress for this library", "warning")
		return c.SendString("")
	}

	return c.SendString(`<uk-icon icon="RefreshCw"></uk-icon>`)
}

// HandleIndexPosters re-indexes posters for all media in the library
func HandleIndexPosters(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return SendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return SendNotFoundError(c, ErrLibraryNotFound)
	}

	log.Debugf("Starting poster re-indexing for library '%s'", library.Name)

	dataBackend := GetFileStore()
	if dataBackend == nil {
		return SendInternalServerError(c, "Data backend not available", nil)
	}

	medias, err := models.GetMediasByLibrarySlug(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	log.Debugf("Processing %d medias for poster re-indexing", len(medias))

	// Candidate poster file names to look for
	posterCandidates := []string{"poster.webp", "poster.jpg", "poster.jpeg", "poster.png", "thumbnail.webp", "thumbnail.jpg", "thumbnail.jpeg", "thumbnail.png", "cover.webp", "cover.jpg", "cover.jpeg", "cover.png"}

	for _, media := range medias {
		log.Debugf("Processing poster for media '%s'", media.Slug)

		chapters, err := models.GetChapters(media.Slug)
		if err != nil || len(chapters) == 0 {
			log.Debugf("Skipping media '%s': no chapters found", media.Slug)
			continue
		}

		lib, err := models.GetLibrary(chapters[0].LibrarySlug)
		if err != nil {
			log.Warnf("Failed to get library for media '%s': %v", media.Slug, err)
			continue
		}

		if len(lib.Folders) == 0 {
			log.Debugf("Skipping media '%s': no folders in library", media.Slug)
			continue
		}

		path := filepath.Join(lib.Folders[0], chapters[0].File)
		mediaDir := filepath.Dir(path)

		log.Debugf("Checking for local posters in '%s' for media '%s'", mediaDir, media.Slug)

		var posterURL string
		var usedLocal bool
		var skipMedia bool

		// Check for local poster files
		for _, candidate := range posterCandidates {
			posterPath := filepath.Join(mediaDir, candidate)
			if stat, err := os.Stat(posterPath); err == nil {
				localSize := stat.Size()

				// Get current poster size if exists
				currentSize := int64(-1)
				if currentData, err := dataBackend.Load("posters/" + media.Slug + ".webp"); err == nil {
					currentSize = int64(len(currentData))
				}

				// Use local poster if sizes differ or no current poster
				if currentSize == -1 || localSize != currentSize {
					log.Debugf("Using local poster '%s' for media '%s' (local size: %d, current size: %d)", posterPath, media.Slug, localSize, currentSize)
					posterURL, err = files.ProcessLocalImageWithThumbnails(posterPath, media.Slug, dataBackend, true)
					if err != nil {
						log.Warnf("Failed to process local poster '%s' for media '%s': %v", posterPath, media.Slug, err)
						continue
					}
					usedLocal = true
					break
				} else {
					log.Debugf("Skipping media '%s': local poster '%s' has same size as current (%d)", media.Slug, posterPath, localSize)
					skipMedia = true
					break
				}
			}
		}

		// Skip this media if local poster exists and size matches
		if skipMedia {
			continue
		}

		// If no local poster was used, try downloading from potential poster URLs
		if !usedLocal {
			// Get media to check for potential poster URLs
			m, err := models.GetMediaUnfiltered(media.Slug)
			if err != nil {
				log.Warnf("Failed to get media '%s': %v", media.Slug, err)
			} else if len(m.PotentialPosterURLs) > 0 {
				log.Debugf("Trying %d potential poster URLs for media '%s'", len(m.PotentialPosterURLs), media.Slug)
				for _, url := range m.PotentialPosterURLs {
					log.Debugf("Attempting to download poster from '%s' for media '%s'", url, media.Slug)
					downloadedURL, err := files.DownloadPosterImage(url, media.Slug, dataBackend, true)
					if err != nil {
						log.Debugf("Failed to download poster from '%s' for media '%s': %v", url, media.Slug, err)
						continue
					}
					posterURL = downloadedURL
					usedLocal = true
					log.Debugf("Downloaded poster from URL for media '%s'", media.Slug)
					break
				}
			}
		}

		// If no local poster or URL was used, extract from archive
		if !usedLocal {
			log.Debugf("Extracting poster from archive for media '%s'", media.Slug)
			posterURL, err = files.ExtractPosterImage(path, media.Slug, dataBackend, true)
			if err != nil {
				log.Warnf("Failed to extract poster for media '%s': %v", media.Slug, err)
				continue
			}
		}

		// Update media cover art
		m, err := models.GetMediaUnfiltered(media.Slug)
		if err != nil {
			log.Warnf("Failed to get media '%s': %v", media.Slug, err)
			continue
		}
		m.CoverArtURL = posterURL
		err = models.UpdateMedia(m)
		if err != nil {
			log.Warnf("Failed to update media '%s': %v", media.Slug, err)
		} else {
			log.Debugf("Updated poster for media '%s'", media.Slug)
		}
	}

	log.Infof("Completed poster re-indexing for library '%s'", library.Name)

	return c.SendString(`<uk-icon icon="Image"></uk-icon>`)
}

// HandleIndexMetadata re-indexes metadata for all media in the library
func HandleIndexMetadata(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return SendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return SendNotFoundError(c, ErrLibraryNotFound)
	}

	log.Debugf("Starting metadata re-indexing for library '%s'", library.Name)

	// For now, trigger full index as metadata re-indexing is complex
	idx := scheduler.NewIndexer(*library, fileStore)
	if ran := idx.RunIndexingJob(); !ran {
		triggerNotification(c, "Indexing already in progress for this library", "warning")
		return c.SendString("")
	}

	return c.SendString(`<uk-icon icon="FileText"></uk-icon>`)
}

// HandleIndexChapters re-indexes chapters for all media in the library
func HandleIndexChapters(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return SendBadRequestError(c, "Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if library == nil {
		return SendNotFoundError(c, ErrLibraryNotFound)
	}

	log.Debugf("Starting chapter re-indexing for library '%s'", library.Name)

	medias, err := models.GetMediasByLibrarySlug(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	for _, media := range medias {
		chapters, err := models.GetChapters(media.Slug)
		if err != nil || len(chapters) == 0 {
			continue
		}

		lib, err := models.GetLibrary(chapters[0].LibrarySlug)
		if err != nil {
			continue
		}

		if len(lib.Folders) == 0 {
			continue
		}

		path := filepath.Dir(filepath.Join(lib.Folders[0], chapters[0].File))

		_, _, _, _, err = scheduler.IndexChapters(media.Slug, path, slug, false)
		if err != nil {
			log.Warnf("Failed to index chapters for media '%s': %v", media.Slug, err)
		}
	}

	return c.SendString(`<uk-icon icon="BookOpen"></uk-icon>`)
}

// HandleAddFolder returns an empty folder form fragment for HTMX inserts.
func HandleAddFolder(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/libraries")
	}

	return handleView(c, views.Folder(""))
}

// HandleRemoveFolder acknowledges folder removal requests without returning content.
func HandleRemoveFolder(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/libraries")
	}

	return c.SendString("")
}

// HandleCancelEdit resets the library form to its default state.
func HandleCancelEdit(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the libraries page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/libraries")
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
func HandleBrowseDirectory(c fiber.Ctx) error {
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
