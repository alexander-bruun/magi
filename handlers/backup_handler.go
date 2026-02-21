package handlers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// handleBackups renders the backups page
func handleBackups(c fiber.Ctx) error {
	backups, err := models.ListBackups(savedBackupDirectory)
	if err != nil {
		log.Errorf("Failed to get backup list: %v", err)
		return SendInternalServerError(c, ErrBackupListFailed, err)
	}

	return handleView(c, views.Backups(backups))
}

// handleCreateBackup creates a new database backup
func handleCreateBackup(c fiber.Ctx) error {
	dataDir := filepath.Dir(savedBackupDirectory)
	backupPath, err := models.CreateBackup(dataDir, savedBackupDirectory)
	if err != nil {
		log.Errorf("Failed to create backup: %v", err)
		return SendInternalServerError(c, ErrBackupCreateFailed, err)
	}

	log.Infof("Backup created successfully: %s", backupPath)

	// Get updated backup list
	backups, err := models.ListBackups(savedBackupDirectory)
	if err != nil {
		log.Errorf("Failed to get updated backup list: %v", err)
		return SendInternalServerError(c, ErrBackupListFailed, err)
	}

	// Return the updated backup list
	return handleView(c, views.BackupList(backups))
}

// handleRestoreBackup restores the database from a backup
func handleRestoreBackup(c fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Backup filename is required")
	}

	// Validate filename to prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid backup filename")
	}

	backupPath := filepath.Join(savedBackupDirectory, filename)

	// Check if backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("Backup file not found")
	}

	dataDir := filepath.Dir(savedBackupDirectory)
	err := models.RestoreBackup(backupPath, dataDir, false)
	if err != nil {
		log.Errorf("Failed to restore backup: %v", err)
		return SendInternalServerError(c, ErrBackupRestoreFailed, err)
	}

	log.Infof("Backup restored successfully from: %s", backupPath)

	// Return success message
	return c.SendString("Backup restored successfully. The database has been reloaded.")
}
