package handlers

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// HandleLibraries renders the libraries dashboard with the current library list.
func HandleLibraries(c *fiber.Ctx) error {
	return HandleView(c, views.Libraries())
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
	c.Response().Header.Set("HX-Trigger", "reset-form")
	c.Response().Header.Set("Content-Type", "text/html")
}

// HandleCreateLibrary persists a new library and returns the refreshed table markup.
func HandleCreateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if err := models.CreateLibrary(library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleDeleteLibrary removes an existing library and responds with the updated table fragment.
func HandleDeleteLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	if err := models.DeleteLibrary(slug); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleUpdateLibrary updates library information and returns the refreshed listing.
func HandleUpdateLibrary(c *fiber.Ctx) error {
	var library models.Library
	if err := c.BodyParser(&library); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	library.Slug = c.Params("slug")
	log.Infof("Updating library: %s", library.Slug)

	if err := models.UpdateLibrary(&library); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	libraries, err := models.GetLibraries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	tableContent, err := renderLibraryTable(libraries)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setCommonHeaders(c)
	return c.SendString(tableContent)
}

// HandleEditLibrary renders the inline edit form for the requested library.
func HandleEditLibrary(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
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
		return c.Status(fiber.StatusBadRequest).SendString("Slug cannot be empty")
	}

	library, err := models.GetLibrary(slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	log.Infof("Starting manual scan for library: %s", library.Name)
	// Create a temporary Indexer for this library and run the job so we
	// preserve the same logging and lifecycle as scheduled jobs.
	idx := indexer.NewIndexer(*library)
	// RunIndexingJob will process all folders for the library.
	idx.RunIndexingJob()

	return c.SendString(`<uk-icon icon="Check"></uk-icon>`)
}

// HandleAddFolder returns an empty folder form fragment for HTMX inserts.
func HandleAddFolder(c *fiber.Ctx) error {
	return HandleView(c, views.Folder(""))
}

// HandleRemoveFolder acknowledges folder removal requests without returning content.
func HandleRemoveFolder(c *fiber.Ctx) error {
	return c.SendString("")
}

// HandleCancelEdit resets the library form to its default state.
func HandleCancelEdit(c *fiber.Ctx) error {
	var buf bytes.Buffer
	err := views.LibraryForm(models.Library{}, "post", false).Render(context.Background(), &buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering form")
	}

	c.Response().Header.Set("Content-Type", "text/html")
	return c.SendString(buf.String())
}

// HandleBetter shows potential duplicate folders across all libraries
func HandleBetter(c *fiber.Ctx) error {
	// Get page parameter, default to 1
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	
	return HandleView(c, views.Better(page))
}

// HandleDismissDuplicate dismisses a manga duplicate entry
func HandleDismissDuplicate(c *fiber.Ctx) error {
	// Get duplicate ID from URL params
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).SendString("Invalid duplicate ID")
	}
	
	// Dismiss the duplicate
	if err := models.DismissMangaDuplicate(int64(id)); err != nil {
		log.Errorf("Failed to dismiss duplicate %d: %v", id, err)
		return c.Status(500).SendString("Failed to dismiss duplicate")
	}
	
	// Return empty response to remove the row
	return c.SendString("")
}

// HandleGetDuplicateFolderInfo returns folder information for a duplicate entry
func HandleGetDuplicateFolderInfo(c *fiber.Ctx) error {
	// Get duplicate ID from URL params
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid duplicate ID"})
	}
	
	// Get folder info
	folderInfo, err := models.GetDuplicateFolderInfo(int64(id))
	if err != nil {
		log.Errorf("Failed to get folder info for duplicate %d: %v", id, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get folder info"})
	}
	
	return c.JSON(folderInfo)
}

// HandleDeleteDuplicateFolder deletes a specific folder from a duplicate entry
func HandleDeleteDuplicateFolder(c *fiber.Ctx) error {
	// Get duplicate ID and folder path from request
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid duplicate ID"})
	}
	
	type DeleteRequest struct {
		FolderPath string `json:"folder_path"`
	}
	
	var req DeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	
	// Delete the folder
	if err := models.DeleteDuplicateFolder(int64(id), req.FolderPath); err != nil {
		log.Errorf("Failed to delete folder %s for duplicate %d: %v", req.FolderPath, id, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete folder"})
	}
	
	return c.JSON(fiber.Map{"success": true})
}


// findDuplicatesInLibrary finds similar folders within a library's directories
func findDuplicatesInLibrary(library models.Library, threshold float64) [][]models.DuplicateFolder {
	var allFolders []string
	
	// Collect all subdirectories from all library folders
	for _, folder := range library.Folders {
		subdirs, err := getSubdirectories(folder)
		if err != nil {
			log.Errorf("Error reading directory %s: %v", folder, err)
			continue
		}
		allFolders = append(allFolders, subdirs...)
	}

	if len(allFolders) < 2 {
		return nil
	}

	// Track which folders we've already grouped
	grouped := make(map[int]bool)
	var duplicateGroups [][]models.DuplicateFolder

	// Compare each folder with all others
	for i := 0; i < len(allFolders); i++ {
		if grouped[i] {
			continue
		}

		var group []models.DuplicateFolder
		group = append(group, models.DuplicateFolder{
			Name:       allFolders[i],
			Similarity: 1.0,
		})

		for j := i + 1; j < len(allFolders); j++ {
			if grouped[j] {
				continue
			}

			similarity := utils.SimilarityRatio(allFolders[i], allFolders[j])
			if similarity >= threshold {
				group = append(group, models.DuplicateFolder{
					Name:       allFolders[j],
					Similarity: similarity,
				})
				grouped[j] = true
			}
		}

		// Only add groups with more than one folder
		if len(group) > 1 {
			duplicateGroups = append(duplicateGroups, group)
			grouped[i] = true
		}
	}

	return duplicateGroups
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

