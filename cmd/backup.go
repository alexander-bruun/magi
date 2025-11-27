package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewBackupCmd creates the backup command
func NewBackupCmd(dataDirectory, backupDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Database backup and restore commands",
	}

	cmd.AddCommand(
		newBackupCreateCmd(dataDirectory, backupDirectory),
		newBackupRestoreCmd(dataDirectory, backupDirectory),
		newBackupListCmd(backupDirectory),
	)

	return cmd
}

func newBackupCreateCmd(dataDirectory, backupDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new database backup",
		Run: func(cmd *cobra.Command, args []string) {
			backupDir := *backupDirectory
			if backupDir == "" {
				backupDir = filepath.Join(*dataDirectory, "backups")
			}

			// Ensure backup directory exists
			if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
				cmd.PrintErrf("Failed to create backup directory: %v\n", err)
				os.Exit(1)
			}

			// Initialize database
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			// Create backup
			backupPath, err := createBackupCLI(*dataDirectory, backupDir)
			if err != nil {
				cmd.PrintErrf("Failed to create backup: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Backup created successfully: %s\n", backupPath)
		},
	}
}

func newBackupRestoreCmd(dataDirectory, backupDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "restore [backup-file]",
		Short: "Restore database from a backup file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			backupFile := args[0]

			backupDir := *backupDirectory
			if backupDir == "" {
				backupDir = filepath.Join(*dataDirectory, "backups")
			}

			backupPath := filepath.Join(backupDir, backupFile)

			// Check if backup file exists
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				cmd.PrintErrf("Backup file not found: %s\n", backupPath)
				os.Exit(1)
			}

			// Initialize database
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			// Restore backup
			err = restoreBackupCLI(backupPath, *dataDirectory)
			if err != nil {
				cmd.PrintErrf("Failed to restore backup: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Backup restored successfully from: %s\n", backupPath)
		},
	}
}

func newBackupListCmd(backupDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backup files",
		Run: func(cmd *cobra.Command, args []string) {
			backupDir := *backupDirectory
			if backupDir == "" {
				backupDir = filepath.Join(os.Getenv("HOME"), "magi", "backups") // fallback
			}

			backups, err := listBackupsCLI(backupDir)
			if err != nil {
				cmd.PrintErrf("Failed to list backups: %v\n", err)
				os.Exit(1)
			}

			if len(backups) == 0 {
				cmd.Println("No backups found")
				return
			}

			cmd.Println("Available backups:")
			for _, backup := range backups {
				cmd.Printf("  %s (%s, %s)\n", backup.Filename, formatFileSize(backup.Size), backup.Created.Format("2006-01-02 15:04:05"))
			}
		},
	}
}

// CLI versions of backup functions
func createBackupCLI(dataDir, backupDir string) (string, error) {
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("magi_backup_%s.db", timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	// Get database path
	dbPath := filepath.Join(dataDir, "magi.db")

	// Copy the database file
	err := copyFileCLI(dbPath, backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy database: %w", err)
	}

	// Also copy WAL and SHM files if they exist
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	if _, err := os.Stat(walPath); err == nil {
		walBackup := backupPath + "-wal"
		if err := copyFileCLI(walPath, walBackup); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to backup WAL file: %v\n", err)
		}
	}

	if _, err := os.Stat(shmPath); err == nil {
		shmBackup := backupPath + "-shm"
		if err := copyFileCLI(shmPath, shmBackup); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to backup SHM file: %v\n", err)
		}
	}

	return backupPath, nil
}

func restoreBackupCLI(backupPath, dataDir string) error {
	// Get database path
	dbPath := filepath.Join(dataDir, "magi.db")

	// Close the current database connection
	if err := models.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Copy the backup file back
	err := copyFileCLI(backupPath, dbPath)
	if err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// Also restore WAL and SHM files if they exist
	backupWal := backupPath + "-wal"
	backupShm := backupPath + "-shm"
	dbWal := dbPath + "-wal"
	dbShm := dbPath + "-shm"

	if _, err := os.Stat(backupWal); err == nil {
		if err := copyFileCLI(backupWal, dbWal); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore WAL file: %v\n", err)
		}
	}

	if _, err := os.Stat(backupShm); err == nil {
		if err := copyFileCLI(backupShm, dbShm); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore SHM file: %v\n", err)
		}
	}

	// Reinitialize the database connection
	err = models.InitializeWithMigration(dataDir, false)
	if err != nil {
		return fmt.Errorf("failed to reinitialize database: %w", err)
	}

	return nil
}

func listBackupsCLI(backupDir string) ([]models.BackupInfo, error) {
	files, err := os.ReadDir(backupDir)
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

func copyFileCLI(src, dst string) error {
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

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}