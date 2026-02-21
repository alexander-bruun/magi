package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils/text"
	"github.com/gofiber/fiber/v3/log"
)

type Library struct {
	Slug             string         `json:"slug"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Cron             string         `json:"cron"`
	Folders          []string       `json:"folders"`
	MetadataProvider sql.NullString `json:"metadata_provider"` // Optional: mangadex, anilist, jikan
	Enabled          bool           `json:"enabled"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
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
	l.Slug = text.Sluggify(l.Name)
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
	now := time.Now()
	library.CreatedAt = now
	library.UpdatedAt = now

	foldersJson, err := json.Marshal(library.Folders)
	if err != nil {
		return fmt.Errorf("failed to marshal folders: %w", err)
	}

	query := `
	INSERT INTO libraries (slug, name, description, cron, folders, metadata_provider, enabled, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, library.Slug, library.Name, library.Description, library.Cron, foldersJson, library.MetadataProvider, library.Enabled, library.CreatedAt.Unix(), library.UpdatedAt.Unix())
	if err != nil {
		return err
	}

	NotifyListeners(Notification{Type: "library_created", Payload: library})
	return nil
}

// GetLibraries retrieves all Libraries from the database
func GetLibraries() ([]Library, error) {
	query := `SELECT slug, name, description, cron, folders, metadata_provider, enabled, created_at, updated_at FROM libraries ORDER BY name ASC`

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
		var createdAt, updatedAt int64
		if err := rows.Scan(&library.Slug, &library.Name, &library.Description, &library.Cron, &foldersJson, &library.MetadataProvider, &library.Enabled, &createdAt, &updatedAt); err != nil {
			log.Errorf("Failed to scan library row: %v", err)
			continue
		}
		library.CreatedAt = time.Unix(createdAt, 0)
		library.UpdatedAt = time.Unix(updatedAt, 0)
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
	SELECT slug, name, description, cron, folders, metadata_provider, enabled, created_at, updated_at
	FROM libraries
	WHERE slug = ?
	`
	row := db.QueryRow(query, slug)

	var library Library
	var foldersJson string
	var createdAt, updatedAt int64
	if err := row.Scan(&library.Slug, &library.Name, &library.Description, &library.Cron, &foldersJson, &library.MetadataProvider, &library.Enabled, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("library with slug %s not found", slug)
		}
		return nil, err
	}
	library.CreatedAt = time.Unix(createdAt, 0)
	library.UpdatedAt = time.Unix(updatedAt, 0)
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
	library.UpdatedAt = time.Now() // Update the timestamp

	foldersJson, err := json.Marshal(library.Folders)
	if err != nil {
		return fmt.Errorf("failed to marshal folders: %w", err)
	}

	query := `
	UPDATE libraries
	SET name = ?, description = ?, cron = ?, folders = ?, metadata_provider = ?, enabled = ?, updated_at = ?
	WHERE slug = ?
	`

	_, err = db.Exec(query, library.Name, library.Description, library.Cron, foldersJson, library.MetadataProvider, library.Enabled, library.UpdatedAt.Unix(), library.Slug)
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

	// Notify listeners to stop the indexer (synchronous — indexer is stopped when this returns)
	NotifyListeners(Notification{Type: "library_deleted", Payload: *library})

	// Now delete all mangas associated with this library
	if err := DeleteMediasByLibrarySlug(slug); err != nil {
		return err
	}

	return nil
}

// LibraryExists checks if a Library exists by slug
func LibraryExists(slug string) (bool, error) {
	return ExistsChecker(`SELECT 1 FROM libraries WHERE slug = ?`, slug)
}

// DuplicateFolder represents a folder with its similarity score
type DuplicateFolder struct {
	Name       string
	Similarity float64
}

// LibraryDuplicates represents duplicates found in a library
type LibraryDuplicates struct {
	Library    Library
	Duplicates [][]DuplicateFolder // Each slice represents a group of similar folders
}

// CheckDuplicateFolders checks if any of the given folders are already used by other libraries
// excludeSlug: if not empty, exclude this library from the check (for updates)
func CheckDuplicateFolders(folders []string, excludeSlug string) error {
	libraries, err := GetLibraries()
	if err != nil {
		return err
	}

	// Create a map of folder to library slug
	folderToLibrary := make(map[string]string)
	for _, lib := range libraries {
		if excludeSlug != "" && lib.Slug == excludeSlug {
			continue
		}
		for _, folder := range lib.Folders {
			folderToLibrary[folder] = lib.Slug
		}
	}

	// Check if any new folder is already used
	for _, folder := range folders {
		if folder == "" {
			continue
		}
		if libSlug, exists := folderToLibrary[folder]; exists {
			return fmt.Errorf("folder '%s' is already used by library '%s'", folder, libSlug)
		}
	}

	return nil
}
