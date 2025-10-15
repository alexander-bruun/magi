package models

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// MangaDuplicate represents when different folders are detected as adding chapters to the same manga
type MangaDuplicate struct {
	ID           int64  `json:"id"`
	MangaSlug    string `json:"manga_slug"`
	MangaName    string `json:"manga_name"`
	LibrarySlug  string `json:"library_slug"`
	LibraryName  string `json:"library_name"`
	FolderPath1  string `json:"folder_path_1"`
	FolderPath2  string `json:"folder_path_2"`
	Dismissed    bool   `json:"dismissed"`
	CreatedAt    int64  `json:"created_at"`
}

// FolderSimilarity represents a potential duplicate folder pair (deprecated - keeping for compatibility)
type FolderSimilarity struct {
	ID              int64   `json:"id"`
	LibrarySlug     string  `json:"library_slug"`
	FolderName1     string  `json:"folder_name_1"`
	FolderName2     string  `json:"folder_name_2"`
	SimilarityScore float64 `json:"similarity_score"`
	Dismissed       bool    `json:"dismissed"`
	CreatedAt       int64   `json:"created_at"`
}

// CreateFolderSimilarity adds a new folder similarity record
func CreateFolderSimilarity(similarity FolderSimilarity) error {
	similarity.CreatedAt = time.Now().Unix()
	
	// Ensure folder names are in consistent order for uniqueness
	if similarity.FolderName1 > similarity.FolderName2 {
		similarity.FolderName1, similarity.FolderName2 = similarity.FolderName2, similarity.FolderName1
	}
	
	query := `
		INSERT OR IGNORE INTO folder_similarities 
		(library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	_, err := db.Exec(query, 
		similarity.LibrarySlug, 
		similarity.FolderName1, 
		similarity.FolderName2, 
		similarity.SimilarityScore,
		0, // dismissed = false
		similarity.CreatedAt,
	)
	
	return err
}

// GetActiveFolderSimilarities returns all non-dismissed similarities
func GetActiveFolderSimilarities() ([]FolderSimilarity, error) {
	query := `
		SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at
		FROM folder_similarities
		WHERE dismissed = 0
		ORDER BY library_slug, similarity_score DESC
	`
	
	rows, err := db.Query(query)
	if err != nil {
		log.Errorf("Failed to get active folder similarities: %v", err)
		return nil, err
	}
	defer rows.Close()
	
	var similarities []FolderSimilarity
	for rows.Next() {
		var s FolderSimilarity
		var dismissed int
		if err := rows.Scan(&s.ID, &s.LibrarySlug, &s.FolderName1, &s.FolderName2, 
			&s.SimilarityScore, &dismissed, &s.CreatedAt); err != nil {
			log.Errorf("Failed to scan folder similarity: %v", err)
			continue
		}
		s.Dismissed = dismissed == 1
		similarities = append(similarities, s)
	}
	
	return similarities, nil
}

// GetActiveFolderSimilaritiesByLibrary returns non-dismissed similarities grouped by library
func GetActiveFolderSimilaritiesByLibrary() (map[string][]FolderSimilarity, error) {
	similarities, err := GetActiveFolderSimilarities()
	if err != nil {
		return nil, err
	}
	
	grouped := make(map[string][]FolderSimilarity)
	for _, s := range similarities {
		grouped[s.LibrarySlug] = append(grouped[s.LibrarySlug], s)
	}
	
	return grouped, nil
}

// DismissFolderSimilarity marks a similarity as dismissed
func DismissFolderSimilarity(id int64) error {
	query := `UPDATE folder_similarities SET dismissed = 1 WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		log.Errorf("Failed to dismiss folder similarity %d: %v", id, err)
	}
	return err
}

// ClearFolderSimilaritiesForLibrary removes all similarities for a library (used when re-indexing)
func ClearFolderSimilaritiesForLibrary(librarySlug string) error {
	query := `DELETE FROM folder_similarities WHERE library_slug = ?`
	_, err := db.Exec(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to clear folder similarities for library %s: %v", librarySlug, err)
	}
	return err
}

// RestoreFolderSimilarity marks a similarity as active again
func RestoreFolderSimilarity(id int64) error {
	query := `UPDATE folder_similarities SET dismissed = 0 WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		log.Errorf("Failed to restore folder similarity %d: %v", id, err)
	}
	return err
}

// GetActiveFolderSimilaritiesWithPagination returns paginated non-dismissed similarities
func GetActiveFolderSimilaritiesWithPagination(page, limit int) ([]FolderSimilarity, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM folder_similarities WHERE dismissed = 0`
	err := db.QueryRow(countQuery).Scan(&total)
	if err != nil {
		log.Errorf("Failed to count active folder similarities: %v", err)
		return nil, 0, err
	}
	
	// Calculate offset
	offset := (page - 1) * limit
	
	// Get paginated results
	query := `
		SELECT id, library_slug, folder_name_1, folder_name_2, similarity_score, dismissed, created_at
		FROM folder_similarities
		WHERE dismissed = 0
		ORDER BY library_slug, similarity_score DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Errorf("Failed to get active folder similarities: %v", err)
		return nil, 0, err
	}
	defer rows.Close()
	
	var similarities []FolderSimilarity
	for rows.Next() {
		var s FolderSimilarity
		var dismissed int
		if err := rows.Scan(&s.ID, &s.LibrarySlug, &s.FolderName1, &s.FolderName2, 
			&s.SimilarityScore, &dismissed, &s.CreatedAt); err != nil {
			log.Errorf("Failed to scan folder similarity: %v", err)
			continue
		}
		s.Dismissed = dismissed == 1
		similarities = append(similarities, s)
	}
	
	return similarities, total, nil
}

// GetActiveFolderSimilaritiesByLibraryWithPagination returns paginated similarities grouped by library
func GetActiveFolderSimilaritiesByLibraryWithPagination(page, limit int) (map[string][]FolderSimilarity, int, error) {
	similarities, total, err := GetActiveFolderSimilaritiesWithPagination(page, limit)
	if err != nil {
		return nil, 0, err
	}
	
	grouped := make(map[string][]FolderSimilarity)
	for _, s := range similarities {
		grouped[s.LibrarySlug] = append(grouped[s.LibrarySlug], s)
	}
	
	return grouped, total, nil
}

// CreateMangaDuplicate adds a new manga duplicate record
func CreateMangaDuplicate(duplicate MangaDuplicate) error {
	duplicate.CreatedAt = time.Now().Unix()
	
	// Ensure folder paths are in consistent order for uniqueness
	if duplicate.FolderPath1 > duplicate.FolderPath2 {
		duplicate.FolderPath1, duplicate.FolderPath2 = duplicate.FolderPath2, duplicate.FolderPath1
	}
	
	query := `
		INSERT OR IGNORE INTO manga_duplicates 
		(manga_slug, library_slug, folder_path_1, folder_path_2, dismissed, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	_, err := db.Exec(query, 
		duplicate.MangaSlug, 
		duplicate.LibrarySlug,
		duplicate.FolderPath1, 
		duplicate.FolderPath2, 
		0, // dismissed = false
		duplicate.CreatedAt,
	)
	
	return err
}

// GetActiveMangaDuplicates returns all non-dismissed manga duplicates with pagination
func GetActiveMangaDuplicates(page, limit int) ([]MangaDuplicate, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM manga_duplicates WHERE dismissed = 0`
	err := db.QueryRow(countQuery).Scan(&total)
	if err != nil {
		log.Errorf("Failed to count active manga duplicates: %v", err)
		return nil, 0, err
	}
	
	// Calculate offset
	offset := (page - 1) * limit
	
	// Get paginated results with joined data
	query := `
		SELECT 
			md.id, 
			md.manga_slug, 
			m.name as manga_name,
			md.library_slug, 
			l.name as library_name,
			md.folder_path_1, 
			md.folder_path_2, 
			md.dismissed, 
			md.created_at
		FROM manga_duplicates md
		LEFT JOIN mangas m ON md.manga_slug = m.slug
		LEFT JOIN libraries l ON md.library_slug = l.slug
		WHERE md.dismissed = 0
		ORDER BY md.created_at DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Errorf("Failed to get active manga duplicates: %v", err)
		return nil, 0, err
	}
	defer rows.Close()
	
	var duplicates []MangaDuplicate
	for rows.Next() {
		var d MangaDuplicate
		var dismissed int
		var mangaName, libraryName sql.NullString
		
		if err := rows.Scan(&d.ID, &d.MangaSlug, &mangaName, &d.LibrarySlug, &libraryName,
			&d.FolderPath1, &d.FolderPath2, &dismissed, &d.CreatedAt); err != nil {
			log.Errorf("Failed to scan manga duplicate: %v", err)
			continue
		}
		
		d.Dismissed = dismissed == 1
		if mangaName.Valid {
			d.MangaName = mangaName.String
		}
		if libraryName.Valid {
			d.LibraryName = libraryName.String
		}
		
		duplicates = append(duplicates, d)
	}
	
	return duplicates, total, nil
}

// DismissMangaDuplicate marks a manga duplicate as dismissed
func DismissMangaDuplicate(id int64) error {
	query := `UPDATE manga_duplicates SET dismissed = 1 WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		log.Errorf("Failed to dismiss manga duplicate %d: %v", id, err)
	}
	return err
}

// ClearMangaDuplicatesForLibrary removes all duplicates for a library (used when re-indexing)
func ClearMangaDuplicatesForLibrary(librarySlug string) error {
	query := `DELETE FROM manga_duplicates WHERE library_slug = ?`
	_, err := db.Exec(query, librarySlug)
	if err != nil {
		log.Errorf("Failed to clear manga duplicates for library %s: %v", librarySlug, err)
	}
	return err
}

// GetMangaDuplicateByFolders checks if a duplicate record exists for the given folders
func GetMangaDuplicateByFolders(mangaSlug, folderPath1, folderPath2 string) (*MangaDuplicate, error) {
	// Ensure consistent order
	if folderPath1 > folderPath2 {
		folderPath1, folderPath2 = folderPath2, folderPath1
	}
	
	query := `
		SELECT id, manga_slug, library_slug, folder_path_1, folder_path_2, dismissed, created_at
		FROM manga_duplicates
		WHERE manga_slug = ? AND folder_path_1 = ? AND folder_path_2 = ?
	`
	
	var d MangaDuplicate
	var dismissed int
	
	err := db.QueryRow(query, mangaSlug, folderPath1, folderPath2).Scan(
		&d.ID, &d.MangaSlug, &d.LibrarySlug, &d.FolderPath1, &d.FolderPath2, &dismissed, &d.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	
	d.Dismissed = dismissed == 1
	return &d, nil
}

// DeleteMangaDuplicateByID deletes a manga duplicate entry by its ID
func DeleteMangaDuplicateByID(id int64) error {
	query := `DELETE FROM manga_duplicates WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		log.Errorf("Failed to delete manga duplicate %d: %v", id, err)
	}
	return err
}

// GetAllMangaDuplicates returns all manga duplicates (including dismissed ones)
func GetAllMangaDuplicates() ([]MangaDuplicate, error) {
	query := `
		SELECT 
			md.id, 
			md.manga_slug, 
			m.name as manga_name,
			md.library_slug, 
			l.name as library_name,
			md.folder_path_1, 
			md.folder_path_2, 
			md.dismissed, 
			md.created_at
		FROM manga_duplicates md
		LEFT JOIN mangas m ON md.manga_slug = m.slug
		LEFT JOIN libraries l ON md.library_slug = l.slug
		ORDER BY md.created_at DESC
	`
	
	rows, err := db.Query(query)
	if err != nil {
		log.Errorf("Failed to get all manga duplicates: %v", err)
		return nil, err
	}
	defer rows.Close()
	
	var duplicates []MangaDuplicate
	for rows.Next() {
		var d MangaDuplicate
		var dismissed int
		var mangaName, libraryName sql.NullString
		
		if err := rows.Scan(&d.ID, &d.MangaSlug, &mangaName, &d.LibrarySlug, &libraryName,
			&d.FolderPath1, &d.FolderPath2, &dismissed, &d.CreatedAt); err != nil {
			log.Errorf("Failed to scan manga duplicate: %v", err)
			continue
		}
		
		d.Dismissed = dismissed == 1
		if mangaName.Valid {
			d.MangaName = mangaName.String
		}
		if libraryName.Valid {
			d.LibraryName = libraryName.String
		}
		
		duplicates = append(duplicates, d)
	}
	
	return duplicates, nil
}

// FolderInfo contains detailed information about a duplicate folder
type FolderInfo struct {
	Path         string `json:"path"`
	BaseName     string `json:"base_name"`
	FileCount    int    `json:"file_count"`
	LastModified int64  `json:"last_modified"`
	Exists       bool   `json:"exists"`
}

// DuplicateFolderInfo contains information about both folders in a duplicate
type DuplicateFolderInfo struct {
	DuplicateID int64      `json:"duplicate_id"`
	MangaSlug   string     `json:"manga_slug"`
	MangaName   string     `json:"manga_name"`
	Folder1     FolderInfo `json:"folder1"`
	Folder2     FolderInfo `json:"folder2"`
}

// GetDuplicateFolderInfo retrieves detailed information about both folders in a duplicate
func GetDuplicateFolderInfo(duplicateID int64) (*DuplicateFolderInfo, error) {
	query := `
		SELECT 
			md.id, 
			md.manga_slug, 
			m.name as manga_name,
			md.folder_path_1, 
			md.folder_path_2
		FROM manga_duplicates md
		LEFT JOIN mangas m ON md.manga_slug = m.slug
		WHERE md.id = ?
	`
	
	var info DuplicateFolderInfo
	var mangaName sql.NullString
	var folderPath1, folderPath2 string
	
	err := db.QueryRow(query, duplicateID).Scan(
		&info.DuplicateID, &info.MangaSlug, &mangaName, &folderPath1, &folderPath2,
	)
	
	if err != nil {
		return nil, err
	}
	
	if mangaName.Valid {
		info.MangaName = mangaName.String
	}
	
	// Get info for folder 1
	info.Folder1 = getFolderInfo(folderPath1)
	
	// Get info for folder 2
	info.Folder2 = getFolderInfo(folderPath2)
	
	return &info, nil
}

// getFolderInfo gets detailed information about a single folder
func getFolderInfo(path string) FolderInfo {
	info := FolderInfo{
		Path:     path,
		BaseName: filepath.Base(path),
		Exists:   false,
	}
	
	// Check if folder exists
	fileInfo, err := os.Stat(path)
	if err == nil {
		info.Exists = true
		info.LastModified = fileInfo.ModTime().Unix()
		
		// Count files in the folder
		fileCount := 0
		filepath.Walk(path, func(p string, f os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !f.IsDir() {
				fileCount++
			}
			return nil
		})
		info.FileCount = fileCount
	}
	
	return info
}

// DeleteDuplicateFolder deletes a folder and removes/updates the duplicate entry
func DeleteDuplicateFolder(duplicateID int64, folderPath string) error {
	// Get the duplicate entry first
	query := `
		SELECT folder_path_1, folder_path_2, manga_slug
		FROM manga_duplicates
		WHERE id = ?
	`
	
	var folder1, folder2, mangaSlug string
	err := db.QueryRow(query, duplicateID).Scan(&folder1, &folder2, &mangaSlug)
	if err != nil {
		return err
	}
	
	// Verify the folder path matches one of the duplicate folders
	if folderPath != folder1 && folderPath != folder2 {
		return fmt.Errorf("folder path does not match duplicate entry")
	}
	
	// Delete the folder from disk
	if err := os.RemoveAll(folderPath); err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	
	log.Infof("Deleted duplicate folder: %s", folderPath)
	
	// After deleting the folder, we should also update the manga's path if needed
	// and delete the duplicate entry since one folder is now gone
	if err := DeleteMangaDuplicateByID(duplicateID); err != nil {
		log.Errorf("Failed to delete duplicate entry after folder deletion: %v", err)
	}
	
	// If we deleted the primary manga path, update it to the remaining folder
	manga, err := GetMangaUnfiltered(mangaSlug)
	if err == nil && manga != nil {
		if manga.Path == folderPath {
			// Update to the other folder
			remainingFolder := folder1
			if folderPath == folder1 {
				remainingFolder = folder2
			}
			manga.Path = remainingFolder
			if err := UpdateManga(manga); err != nil {
				log.Errorf("Failed to update manga path after folder deletion: %v", err)
			}
		}
	}
	
	return nil
}

