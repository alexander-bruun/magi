package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
		newBackupListCmd(dataDirectory, backupDirectory),
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
			if err := os.MkdirAll(backupDir, 0750); err != nil {
				cmd.PrintErrf("Failed to create backup directory: %v\n", err)
				os.Exit(1)
			}

			withDB(dataDirectory, cmd, func() error {
				backupPath, err := models.CreateBackup(*dataDirectory, backupDir)
				if err != nil {
					return fmt.Errorf("Failed to create backup: %w", err)
				}

				cmd.Printf("Backup created successfully: %s\n", backupPath)
				return nil
			})
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

			withDB(dataDirectory, cmd, func() error {
				if err := models.RestoreBackup(backupPath, *dataDirectory, true); err != nil {
					return fmt.Errorf("Failed to restore backup: %w", err)
				}

				cmd.Printf("Backup restored successfully from: %s\n", backupPath)
				return nil
			})
		},
	}
}

func newBackupListCmd(dataDirectory, backupDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backup files",
		Run: func(cmd *cobra.Command, args []string) {
			backupDir := *backupDirectory
			if backupDir == "" {
				backupDir = filepath.Join(*dataDirectory, "backups")
			}

			backups, err := models.ListBackups(backupDir)
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
