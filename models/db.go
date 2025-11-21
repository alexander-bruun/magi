package models

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"

	"database/sql"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

var db *sql.DB

// Initialize connects to the SQLite database and applies necessary migrations
func Initialize(cacheDirectory string) error {
	start := time.Now()
	defer utils.LogDuration("Initialize", start)

	databasePath := filepath.Join(cacheDirectory, "magi.db")

	var err error
	db, err = sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}

	// Enable WAL mode for better concurrency
	if _, err = db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err = db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err = db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Initialize schema_migrations table if it doesn't exist
	err = initializeSchemaMigrationsTable()
	if err != nil {
		return err
	}

	// Apply migrations from the "migrations" folder
	err = applyMigrations("migrations")
	if err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func Close() error {
	start := time.Now()
	defer utils.LogDuration("Close", start)

	if db != nil {
		return db.Close()
	}
	return nil
}

// initializeSchemaMigrationsTable ensures that the schema_migrations table exists
func initializeSchemaMigrationsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY
	);
	`
	_, err := db.Exec(query)
	return err
}

// applyMigrations reads and applies all new migrations from the specified folder
func applyMigrations(migrationsDir string) error {
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Filter out only the .up.sql files and sort them by version
	var migrationFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".up.sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Get the latest applied migration version
	var currentVersion int
	err = db.QueryRow("SELECT IFNULL(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return err
	}

	for _, fileName := range migrationFiles {
		version, err := extractVersion(fileName)
		if err != nil {
			return err
		}
		if version > currentVersion {
			// Apply the migration
			err := applyMigration(migrationsDir, fileName, version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// applyMigration reads and applies a single migration file
func applyMigration(migrationsDir, fileName string, version int) error {
	migrationPath := filepath.Join(migrationsDir, fileName)
	query, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
	}

	// Execute the migration
	_, err = db.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
	}

	// Record the applied migration version
	_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version)
	if err != nil {
		return err
	}

	log.Infof("Migration %s (version %d) applied successfully.\n", fileName, version)
	return nil
}

// extractVersion extracts the version number from the migration file name
func extractVersion(fileName string) (int, error) {
	parts := strings.Split(fileName, "_")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid migration file name: %s", fileName)
	}
	var version int
	_, err := fmt.Sscanf(parts[0], "%d", &version)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version from file name: %w", err)
	}
	return version, nil
}

// rollbackMigration rolls back a migration by applying its .down.sql file
func rollbackMigration(migrationsDir string, version int) error {
	downFileName := fmt.Sprintf("%03d*.down.sql", version) // Example: 001*.down.sql
	files, err := filepath.Glob(filepath.Join(migrationsDir, downFileName))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no rollback file found for version %d", version)
	}

	// Read and apply the .down.sql file
	query, err := os.ReadFile(files[0])
	if err != nil {
		return fmt.Errorf("failed to read rollback file: %w", err)
	}

	_, err = db.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed to execute rollback: %w", err)
	}

	// Remove the migration version from schema_migrations
	_, err = db.Exec("DELETE FROM schema_migrations WHERE version = ?", version)
	if err != nil {
		return fmt.Errorf("failed to remove migration version: %w", err)
	}

	log.Infof("Rollback version %d applied successfully.\n", version)
	return nil
}

// ExistsChecker is a generic function to check if a record exists
func ExistsChecker(query string, args ...interface{}) (bool, error) {
	var exists int
	err := db.QueryRow(query, args...).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TimestampPair holds creation and update timestamps
type TimestampPair struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTimestamps creates a new timestamp pair with current time
func NewTimestamps() TimestampPair {
	now := time.Now()
	return TimestampPair{CreatedAt: now, UpdatedAt: now}
}

// UpdateTimestamp updates only the UpdatedAt field
func (t *TimestampPair) UpdateTimestamp() {
	t.UpdatedAt = time.Now()
}

// UnixTimestamps returns both timestamps as Unix seconds
func (t *TimestampPair) UnixTimestamps() (int64, int64) {
	return t.CreatedAt.Unix(), t.UpdatedAt.Unix()
}

// FromUnixTimestamps populates timestamps from Unix seconds
func (t *TimestampPair) FromUnixTimestamps(createdAt, updatedAt int64) {
	t.CreatedAt = time.Unix(createdAt, 0)
	t.UpdatedAt = time.Unix(updatedAt, 0)
}

// CountRecords returns count from a query
func CountRecords(query string, args ...interface{}) (int64, error) {
	var count int64
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// DeleteRecord executes a delete query
func DeleteRecord(query string, args ...interface{}) error {
	_, err := db.Exec(query, args...)
	return err
}
