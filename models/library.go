package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"github.com/gofiber/fiber/v2/log"
)

type Library struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Cron        string   `json:"cron"`
	Folders     []string `json:"folders"`
	CreatedAt   int64    `json:"created_at"` // Unix timestamp
	UpdatedAt   int64    `json:"updated_at"` // Unix timestamp
}

// GetFolderNames returns a comma-separated string of folder names
func (l *Library) GetFolderNames() string {
	return strings.Join(l.Folders, ", ")
}

// Validate checks if the Library has valid values
func (l *Library) Validate() error {
	if l.Name == "" {
		return errors.New("library name cannot be empty")
	}
	if l.Description == "" {
		return errors.New("library description cannot be empty")
	}
	if l.Cron == "" {
		return errors.New("library cron cannot be empty")
	}
	l.Slug = utils.Sluggify(l.Name)
	return nil
}

// CreateLibrary adds a new Library to the database
func CreateLibrary(library Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	exists, err := LibraryExists(library.Slug)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("library already exists")
	}

	// Set CreatedAt and UpdatedAt fields to current time
	now := time.Now().Unix()
	library.CreatedAt = now
	library.UpdatedAt = now

	foldersJson, err := json.Marshal(library.Folders)
	if err != nil {
		return fmt.Errorf("failed to marshal folders: %w", err)
	}

	query := `
	INSERT INTO libraries (slug, name, description, cron, folders, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, library.Slug, library.Name, library.Description, library.Cron, foldersJson, library.CreatedAt, library.UpdatedAt)
	if err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_created", Payload: library})
	return nil
}

// GetLibraries retrieves all Libraries from the database
func GetLibraries() ([]Library, error) {
	query := `SELECT slug, name, description, cron, folders, created_at, updated_at FROM libraries`

	rows, err := db.Query(query)
	if err != nil {
		log.Errorf("Failed to get all libraries: %v", err)
		return nil, err
	}
	defer rows.Close()

	var libraries []Library
	for rows.Next() {
		var library Library
		var foldersJson string
		if err := rows.Scan(&library.Slug, &library.Name, &library.Description, &library.Cron, &foldersJson, &library.CreatedAt, &library.UpdatedAt); err != nil {
			log.Errorf("Failed to scan library row: %v", err)
			continue
		}
		if err := json.Unmarshal([]byte(foldersJson), &library.Folders); err != nil {
			log.Errorf("Failed to unmarshal folders JSON: %v", err)
			continue
		}
		libraries = append(libraries, library)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return libraries, nil
}

// GetLibrary retrieves a single Library by slug
func GetLibrary(slug string) (*Library, error) {
	query := `
	SELECT slug, name, description, cron, folders, created_at, updated_at
	FROM libraries
	WHERE slug = ?
	`
	row := db.QueryRow(query, slug)

	var library Library
	var foldersJson string
	if err := row.Scan(&library.Slug, &library.Name, &library.Description, &library.Cron, &foldersJson, &library.CreatedAt, &library.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("library with slug %s not found", slug)
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(foldersJson), &library.Folders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal folders JSON: %w", err)
	}
	return &library, nil
}

// UpdateLibrary modifies an existing Library
func UpdateLibrary(library *Library) error {
	if err := library.Validate(); err != nil {
		return err
	}
	library.UpdatedAt = time.Now().Unix() // Update the timestamp

	foldersJson, err := json.Marshal(library.Folders)
	if err != nil {
		return fmt.Errorf("failed to marshal folders: %w", err)
	}

	query := `
	UPDATE libraries
	SET name = ?, description = ?, cron = ?, folders = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err = db.Exec(query, library.Name, library.Description, library.Cron, foldersJson, library.UpdatedAt, library.Slug)
	if err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_updated", Payload: *library})
	return nil
}

// DeleteLibrary removes a Library and its associated mangas
func DeleteLibrary(slug string) error {
	library, err := GetLibrary(slug)
	if err != nil {
		return err
	}

	query := `DELETE FROM libraries WHERE slug = ?`

	_, err = db.Exec(query, slug)
	if err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_deleted", Payload: *library})
	if err := DeleteMangasByLibrarySlug(slug); err != nil {
		return err
	}
	return nil
}

// LibraryExists checks if a Library exists by slug
func LibraryExists(slug string) (bool, error) {
	query := `SELECT 1 FROM libraries WHERE slug = ?`
	row := db.QueryRow(query, slug)
	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
