package handlers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// BackupInfo represents information about a backup
type BackupInfo struct {
	Filename string
	Size     int64
	Created  time.Time
}

// HandleBackups renders the backups page
func HandleBackups(c *fiber.Ctx) error {
	backups, err := getBackupList()
	if err != nil {
		log.Errorf("Failed to get backup list: %v", err)
		return handleError(c, err)
	}

	return HandleView(c, views.Backups(backups))
}

// HandleCreateBackup creates a new database backup
func HandleCreateBackup(c *fiber.Ctx) error {
	backupPath, err := createBackup()
	if err != nil {
		log.Errorf("Failed to create backup: %v", err)
		return handleError(c, err)
	}

	log.Infof("Backup created successfully: %s", backupPath)

	// Get updated backup list
	backups, err := getBackupList()
	if err != nil {
		log.Errorf("Failed to get updated backup list: %v", err)
		return handleError(c, err)
	}

	// Return the updated backup list
	return HandleView(c, views.BackupList(backups))
}

// HandleRestoreBackup restores the database from a backup
func HandleRestoreBackup(c *fiber.Ctx) error {
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

	err := restoreBackup(backupPath)
	if err != nil {
		log.Errorf("Failed to restore backup: %v", err)
		return handleError(c, err)
	}

	log.Infof("Backup restored successfully from: %s", backupPath)

	// Return success message
	return c.SendString("Backup restored successfully. The database has been reloaded.")
}

// getBackupList returns a list of available backups
func getBackupList() ([]models.BackupInfo, error) {
	files, err := os.ReadDir(savedBackupDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.BackupInfo{}, nil
		}
		return nil, err
	}

	var backups []models.BackupInfo
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".db") {
			info, err := file.Info()
			if err != nil {
				continue
			}

			backups = append(backups, models.BackupInfo{
				Filename: file.Name(),
				Size:     info.Size(),
				Created:  info.ModTime(),
			})
		}
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Created.After(backups[j].Created)
	})

	return backups, nil
}

// createBackup creates a backup of the current database
func createBackup() (string, error) {
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("magi_backup_%s.db", timestamp)
	backupPath := filepath.Join(savedBackupDirectory, backupFilename)

	// Get database path
	dbPath := filepath.Join(filepath.Dir(savedBackupDirectory), "magi.db") // Assuming backup dir is in data dir

	// Copy the database file
	err := copyFile(dbPath, backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy database: %w", err)
	}

	// Also copy WAL and SHM files if they exist
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	if _, err := os.Stat(walPath); err == nil {
		walBackup := backupPath + "-wal"
		if err := copyFile(walPath, walBackup); err != nil {
			log.Warnf("Failed to backup WAL file: %v", err)
		}
	}

	if _, err := os.Stat(shmPath); err == nil {
		shmBackup := backupPath + "-shm"
		if err := copyFile(shmPath, shmBackup); err != nil {
			log.Warnf("Failed to backup SHM file: %v", err)
		}
	}

	return backupPath, nil
}

// restoreBackup restores the database from a backup
func restoreBackup(backupPath string) error {
	// Get database path
	dbPath := filepath.Join(filepath.Dir(savedBackupDirectory), "magi.db")

	// Close the current database connection
	if err := models.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Copy the backup file back
	err := copyFile(backupPath, dbPath)
	if err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// Also restore WAL and SHM files if they exist
	backupWal := backupPath + "-wal"
	backupShm := backupPath + "-shm"
	dbWal := dbPath + "-wal"
	dbShm := dbPath + "-shm"

	if _, err := os.Stat(backupWal); err == nil {
		if err := copyFile(backupWal, dbWal); err != nil {
			log.Warnf("Failed to restore WAL file: %v", err)
		}
	}

	if _, err := os.Stat(backupShm); err == nil {
		if err := copyFile(backupShm, dbShm); err != nil {
			log.Warnf("Failed to restore SHM file: %v", err)
		}
	}

	// Reinitialize the database connection
	dataDir := filepath.Dir(savedBackupDirectory)
	err = models.Initialize(dataDir)
	if err != nil {
		return fmt.Errorf("failed to reinitialize database: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}