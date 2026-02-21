package models

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CreateBackup creates a database backup in the specified directory.
// Returns the full path to the backup file.
func CreateBackup(dataDir, backupDir string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("magi_backup_%s.db", timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	dbPath := filepath.Join(dataDir, "magi.db")

	if err := copyFile(dbPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to copy database: %w", err)
	}

	// Also copy WAL and SHM files if they exist
	for _, suffix := range []string{"-wal", "-shm"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, backupPath+suffix); err != nil {
				// Non-fatal: log but continue
				fmt.Fprintf(os.Stderr, "Warning: failed to backup %s file: %v\n", suffix, err)
			}
		}
	}

	return backupPath, nil
}

// RestoreBackup restores the database from a backup file.
// It closes the current DB connection, copies files, and reinitializes.
func RestoreBackup(backupPath, dataDir string, reinitWithMigrations bool) error {
	dbPath := filepath.Join(dataDir, "magi.db")

	if err := Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	if err := copyFile(backupPath, dbPath); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// Also restore WAL and SHM files if they exist
	for _, suffix := range []string{"-wal", "-shm"} {
		src := backupPath + suffix
		dst := dbPath + suffix
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to restore %s file: %v\n", suffix, err)
			}
		}
	}

	if reinitWithMigrations {
		return InitializeWithMigration(dataDir, false)
	}
	return Initialize(dataDir)
}

// ListBackups returns available backup files sorted newest-first.
func ListBackups(backupDir string) ([]BackupInfo, error) {
	files, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".db") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			backups = append(backups, BackupInfo{
				Filename: file.Name(),
				Size:     info.Size(),
				Created:  info.ModTime(),
			})
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Created.After(backups[j].Created)
	})

	return backups, nil
}

// copyFile copies a file from src to dst, preserving permissions.
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

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}
