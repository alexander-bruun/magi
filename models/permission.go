package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Permission represents a permission that can be assigned to users
type Permission struct {
	ID                   int64  `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	IsWildcard           bool   `json:"is_wildcard"`
	IsEnabled            bool   `json:"is_enabled"`
	PremiumChapterAccess bool   `json:"premium_chapter_access"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
}

// LibraryPermission links a permission to a library
type LibraryPermission struct {
	PermissionID int64  `json:"permission_id"`
	LibrarySlug  string `json:"library_slug"`
	CreatedAt    int64  `json:"created_at"`
}

// UserPermission assigns a permission to a user
type UserPermission struct {
	Username     string `json:"username"`
	PermissionID int64  `json:"permission_id"`
	CreatedAt    int64  `json:"created_at"`
}

// RolePermission assigns a permission to a role
type RolePermission struct {
	Role         string `json:"role"`
	PermissionID int64  `json:"permission_id"`
	CreatedAt    int64  `json:"created_at"`
}

// PermissionWithLibraries includes the permission and its associated libraries
type PermissionWithLibraries struct {
	Permission
	Libraries []string `json:"libraries"`
}

// CreatePermission creates a new permission
func CreatePermission(name, description string, isWildcard, premiumChapterAccess bool) (*Permission, error) {
	if name == "" {
		return nil, fmt.Errorf("permission name cannot be empty")
	}

	now := time.Now().Unix()
	query := `
	INSERT INTO permissions (name, description, is_wildcard, is_enabled, premium_chapter_access, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query, name, description, isWildcard, true, premiumChapterAccess, now, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Permission{
		ID:                   id,
		Name:                 name,
		Description:          description,
		IsWildcard:           isWildcard,
		IsEnabled:            true,
		PremiumChapterAccess: premiumChapterAccess,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

// GetPermissions retrieves all permissions
func GetPermissions() ([]Permission, error) {
	query := `SELECT id, name, description, is_wildcard, is_enabled, premium_chapter_access, created_at, updated_at FROM permissions ORDER BY name`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.IsWildcard, &p.IsEnabled, &p.PremiumChapterAccess, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		permissions = append(permissions, p)
	}

	return permissions, rows.Err()
}

// GetPermission retrieves a single permission by ID
func GetPermission(id int64) (*Permission, error) {
	query := `SELECT id, name, description, is_wildcard, is_enabled, premium_chapter_access, created_at, updated_at FROM permissions WHERE id = ?`

	var p Permission
	err := db.QueryRow(query, id).Scan(&p.ID, &p.Name, &p.Description, &p.IsWildcard, &p.IsEnabled, &p.PremiumChapterAccess, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &p, nil
}

// GetPermissionWithLibraries retrieves a permission with its associated libraries
func GetPermissionWithLibraries(id int64) (*PermissionWithLibraries, error) {
	permission, err := GetPermission(id)
	if err != nil || permission == nil {
		return nil, err
	}

	libraries, err := GetLibrariesForPermission(id)
	if err != nil {
		return nil, err
	}

	return &PermissionWithLibraries{
		Permission: *permission,
		Libraries:  libraries,
	}, nil
}

// GetAllPermissionsWithLibraries retrieves all permissions with their associated libraries
func GetAllPermissionsWithLibraries() ([]PermissionWithLibraries, error) {
	permissions, err := GetPermissions()
	if err != nil {
		return nil, err
	}

	result := make([]PermissionWithLibraries, 0, len(permissions))
	for _, p := range permissions {
		libraries, err := GetLibrariesForPermission(p.ID)
		if err != nil {
			return nil, err
		}

		result = append(result, PermissionWithLibraries{
			Permission: p,
			Libraries:  libraries,
		})
	}

	return result, nil
}

// UpdatePermission updates an existing permission
func UpdatePermission(id int64, name, description string, isWildcard, isEnabled, premiumChapterAccess bool) error {
	if name == "" {
		return fmt.Errorf("permission name cannot be empty")
	}

	now := time.Now().Unix()
	query := `
	UPDATE permissions
	SET name = ?, description = ?, is_wildcard = ?, is_enabled = ?, premium_chapter_access = ?, updated_at = ?
	WHERE id = ?
	`

	_, err := db.Exec(query, name, description, isWildcard, isEnabled, premiumChapterAccess, now, id)
	return err
}

// DeletePermission deletes a permission and all its associations
func DeletePermission(id int64) error {
	query := `DELETE FROM permissions WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

// BindPermissionToLibrary creates a link between a permission and a library
func BindPermissionToLibrary(permissionID int64, librarySlug string) error {
	now := time.Now().Unix()
	query := `
	INSERT OR IGNORE INTO library_permissions (permission_id, library_slug, created_at)
	VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, permissionID, librarySlug, now)
	return err
}

// UnbindPermissionFromLibrary removes the link between a permission and a library
func UnbindPermissionFromLibrary(permissionID int64, librarySlug string) error {
	query := `DELETE FROM library_permissions WHERE permission_id = ? AND library_slug = ?`
	_, err := db.Exec(query, permissionID, librarySlug)
	return err
}

// GetLibrariesForPermission retrieves all library slugs associated with a permission
func GetLibrariesForPermission(permissionID int64) ([]string, error) {
	query := `SELECT library_slug FROM library_permissions WHERE permission_id = ? ORDER BY library_slug`

	rows, err := db.Query(query, permissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libraries []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		libraries = append(libraries, slug)
	}

	return libraries, rows.Err()
}

// AssignPermissionToUser assigns a permission to a user
func AssignPermissionToUser(username string, permissionID int64) error {
	now := time.Now().Unix()
	query := `
	INSERT OR IGNORE INTO user_permissions (username, permission_id, created_at)
	VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, username, permissionID, now)
	return err
}

// RevokePermissionFromUser removes a permission from a user
func RevokePermissionFromUser(username string, permissionID int64) error {
	query := `DELETE FROM user_permissions WHERE username = ? AND permission_id = ?`
	_, err := db.Exec(query, username, permissionID)
	return err
}

// GetUserPermissions retrieves all permission IDs assigned to a user
func GetUserPermissions(username string) ([]int64, error) {
	query := `SELECT permission_id FROM user_permissions WHERE username = ? ORDER BY permission_id`

	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		permissionIDs = append(permissionIDs, id)
	}

	return permissionIDs, rows.Err()
}

// GetUsersWithPermission retrieves all usernames that have a specific permission
func GetUsersWithPermission(permissionID int64) ([]string, error) {
	query := `SELECT username FROM user_permissions WHERE permission_id = ? ORDER BY username`

	rows, err := db.Query(query, permissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}

	return usernames, rows.Err()
}

// UserHasLibraryAccess checks if a user has access to a specific library
// Returns true if:
// 1. User has an enabled wildcard permission (direct or role-based)
// 2. User has an enabled permission specifically bound to the library (direct or role-based)
func UserHasLibraryAccess(username, librarySlug string) (bool, error) {
	// Get user's role
	user, err := FindUserByUsername(username)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	// Check direct user permissions
	query := `
	SELECT COUNT(*)
	FROM user_permissions up
	JOIN permissions p ON up.permission_id = p.id
	LEFT JOIN library_permissions lp ON p.id = lp.permission_id
	LEFT JOIN libraries l ON lp.library_slug = l.slug
	WHERE up.username = ?
	  AND p.is_enabled = 1
	  AND (
	    p.is_wildcard = 1
	    OR (lp.library_slug = ?)
	  )
	`

	var directCount int
	err = db.QueryRow(query, username, librarySlug).Scan(&directCount)
	if err != nil {
		return false, err
	}

	if directCount > 0 {
		return true, nil
	}

	// Check role-based permissions
	roleQuery := `
	SELECT COUNT(*)
	FROM role_permissions rp
	JOIN permissions p ON rp.permission_id = p.id
	LEFT JOIN library_permissions lp ON p.id = lp.permission_id
	LEFT JOIN libraries l ON lp.library_slug = l.slug
	WHERE rp.role = ?
	  AND p.is_enabled = 1
	  AND (
	    p.is_wildcard = 1
	    OR (lp.library_slug = ?)
	  )
	`

	var roleCount int
	err = db.QueryRow(roleQuery, user.Role, librarySlug).Scan(&roleCount)
	if err != nil {
		return false, err
	}

	return roleCount > 0, nil
}

// GetAccessibleLibrariesForUser retrieves all library slugs that a user can access
// Based on their assigned permissions (direct and role-based)
func GetAccessibleLibrariesForUser(username string) ([]string, error) {
	// Get user's role
	user, err := FindUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Admins and moderators have access to all libraries
	if user.Role == "admin" || user.Role == "moderator" {
		libraries, err := GetLibraries()
		if err != nil {
			return nil, err
		}

		slugs := make([]string, 0, len(libraries))
		for _, lib := range libraries {
			if lib.Enabled {
				slugs = append(slugs, lib.Slug)
			}
		}
		return slugs, nil
	}

	// First check if user has wildcard permission (direct or role-based)
	hasWildcard, err := userOrRoleHasWildcardPermission(username, user.Role)
	if err != nil {
		return nil, err
	}

	// If user has wildcard, return all enabled libraries
	if hasWildcard {
		libraries, err := GetLibraries()
		if err != nil {
			return nil, err
		}

		slugs := make([]string, 0, len(libraries))
		for _, lib := range libraries {
			if lib.Enabled {
				slugs = append(slugs, lib.Slug)
			}
		}
		return slugs, nil
	}

	// Otherwise, return libraries from both user and role permissions
	query := `
	SELECT DISTINCT lp.library_slug
	FROM (
		SELECT permission_id FROM user_permissions WHERE username = ?
		UNION
		SELECT permission_id FROM role_permissions WHERE role = ?
	) AS combined
	JOIN permissions p ON combined.permission_id = p.id
	JOIN library_permissions lp ON p.id = lp.permission_id
	JOIN libraries l ON lp.library_slug = l.slug
	WHERE p.is_enabled = 1 AND l.enabled = true
	ORDER BY lp.library_slug
	`

	rows, err := db.Query(query, username, user.Role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libraries []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		libraries = append(libraries, slug)
	}

	return libraries, rows.Err()
}

// userOrRoleHasWildcardPermission checks if a user or their role has any enabled wildcard permission
func userOrRoleHasWildcardPermission(username, role string) (bool, error) {
	query := `
	SELECT COUNT(*)
	FROM (
		SELECT permission_id FROM user_permissions WHERE username = ?
		UNION
		SELECT permission_id FROM role_permissions WHERE role = ?
	) AS combined
	JOIN permissions p ON combined.permission_id = p.id
	WHERE p.is_wildcard = 1
	  AND p.is_enabled = 1
	`

	var count int
	err := db.QueryRow(query, username, role).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// SetLibrariesForPermission replaces all library bindings for a permission
func SetLibrariesForPermission(permissionID int64, librarySlugs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove all existing bindings
	_, err = tx.Exec(`DELETE FROM library_permissions WHERE permission_id = ?`, permissionID)
	if err != nil {
		return err
	}

	// Add new bindings
	now := time.Now().Unix()
	stmt, err := tx.Prepare(`INSERT INTO library_permissions (permission_id, library_slug, created_at) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, slug := range librarySlugs {
		_, err = stmt.Exec(permissionID, slug, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetUserPermissionDetails retrieves detailed permission information for a user
func GetUserPermissionDetails(username string) ([]PermissionWithLibraries, error) {
	permissionIDs, err := GetUserPermissions(username)
	if err != nil {
		return nil, err
	}

	result := make([]PermissionWithLibraries, 0, len(permissionIDs))
	for _, id := range permissionIDs {
		permWithLibs, err := GetPermissionWithLibraries(id)
		if err != nil {
			return nil, err
		}
		if permWithLibs != nil {
			result = append(result, *permWithLibs)
		}
	}

	return result, nil
}

// AssignPermissionToRole assigns a permission to a role
func AssignPermissionToRole(role string, permissionID int64) error {
	now := time.Now().Unix()
	query := `
	INSERT OR IGNORE INTO role_permissions (role, permission_id, created_at)
	VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, role, permissionID, now)
	return err
}

// RevokePermissionFromRole removes a permission from a role
func RevokePermissionFromRole(role string, permissionID int64) error {
	query := `DELETE FROM role_permissions WHERE role = ? AND permission_id = ?`
	_, err := db.Exec(query, role, permissionID)
	return err
}

// GetRolePermissions retrieves all permission IDs assigned to a role
func GetRolePermissions(role string) ([]int64, error) {
	query := `SELECT permission_id FROM role_permissions WHERE role = ? ORDER BY permission_id`

	rows, err := db.Query(query, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		permissionIDs = append(permissionIDs, id)
	}

	return permissionIDs, rows.Err()
}

// GetRolePermissionDetails retrieves detailed permission information for a role
func GetRolePermissionDetails(role string) ([]PermissionWithLibraries, error) {
	permissionIDs, err := GetRolePermissions(role)
	if err != nil {
		return nil, err
	}

	result := make([]PermissionWithLibraries, 0, len(permissionIDs))
	for _, id := range permissionIDs {
		permWithLibs, err := GetPermissionWithLibraries(id)
		if err != nil {
			return nil, err
		}
		if permWithLibs != nil {
			result = append(result, *permWithLibs)
		}
	}

	return result, nil
}

// GetRolesWithPermission retrieves all roles that have a specific permission
func GetRolesWithPermission(permissionID int64) ([]string, error) {
	query := `SELECT role FROM role_permissions WHERE permission_id = ? ORDER BY role`

	rows, err := db.Query(query, permissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

// AnonymousHasLibraryAccess checks if anonymous (unauthenticated) users have access to a specific library
// Returns true if the 'anonymous' role has:
// 1. An enabled wildcard permission
// 2. An enabled permission specifically bound to the library
func AnonymousHasLibraryAccess(librarySlug string) (bool, error) {
	query := `
	SELECT COUNT(*)
	FROM role_permissions rp
	JOIN permissions p ON rp.permission_id = p.id
	LEFT JOIN library_permissions lp ON p.id = lp.permission_id
	LEFT JOIN libraries l ON lp.library_slug = l.slug
	WHERE rp.role = 'anonymous'
	  AND p.is_enabled = 1
	  AND (
	    p.is_wildcard = 1
	    OR (lp.library_slug = ?)
	  )
	`

	var count int
	err := db.QueryRow(query, librarySlug).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetAccessibleLibrariesForAnonymous retrieves all library slugs that anonymous users can access
// Based on permissions assigned to the 'anonymous' role
func GetAccessibleLibrariesForAnonymous() ([]string, error) {
	// First check if anonymous role has wildcard permission
	hasWildcard, err := roleHasWildcardPermission("anonymous")
	if err != nil {
		return nil, err
	}

	// If anonymous has wildcard, return all libraries
	if hasWildcard {
		libraries, err := GetLibraries()
		if err != nil {
			return nil, err
		}

		slugs := make([]string, 0, len(libraries))
		for _, lib := range libraries {
			if lib.Enabled {
				slugs = append(slugs, lib.Slug)
			}
		}
		return slugs, nil
	}

	// Otherwise, return libraries from anonymous role permissions
	query := `
	SELECT DISTINCT lp.library_slug
	FROM role_permissions rp
	JOIN permissions p ON rp.permission_id = p.id
	JOIN library_permissions lp ON p.id = lp.permission_id
	JOIN libraries l ON lp.library_slug = l.slug
	WHERE rp.role = 'anonymous'
	  AND p.is_enabled = 1 AND l.enabled = true
	ORDER BY lp.library_slug
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libraries []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		libraries = append(libraries, slug)
	}

	// If no specific permissions found for anonymous, allow access to all libraries
	if len(libraries) == 0 {
		allLibraries, err := GetLibraries()
		if err != nil {
			return nil, err
		}

		for _, lib := range allLibraries {
			if lib.Enabled {
				libraries = append(libraries, lib.Slug)
			}
		}
	}

	return libraries, rows.Err()
}

// roleHasWildcardPermission checks if a role has any wildcard permission
func roleHasWildcardPermission(role string) (bool, error) {
	query := `
	SELECT COUNT(*)
	FROM role_permissions rp
	JOIN permissions p ON rp.permission_id = p.id
	WHERE rp.role = ?
	  AND p.is_wildcard = 1
	`

	var count int
	err := db.QueryRow(query, role).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RoleHasWildcardPermission checks if a role has any wildcard permission
func RoleHasWildcardPermission(role string) (bool, error) {
	return roleHasWildcardPermission(role)
}

// RoleHasAccess checks if a role has access to premium chapters
// Returns true if the role has any permission with premium_chapter_access = true
func RoleHasAccess(role string) (bool, error) {
	query := `
	SELECT COUNT(*)
	FROM role_permissions rp
	JOIN permissions p ON rp.permission_id = p.id
	WHERE rp.role = ?
	  AND p.premium_chapter_access = 1
	`

	var count int
	err := db.QueryRow(query, role).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// UserHasPremiumChapterAccess checks if a user has access to premium chapters
// Returns true if:
// 1. User has an permission with premium_chapter_access = true (direct or role-based)
func UserHasPremiumChapterAccess(username string) (bool, error) {
	// Get user's role
	user, err := FindUserByUsername(username)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	// Check direct user permissions
	query := `
	SELECT COUNT(*)
	FROM user_permissions up
	JOIN permissions p ON up.permission_id = p.id
	WHERE up.username = ?
	  AND p.premium_chapter_access = 1
	`

	var directCount int
	err = db.QueryRow(query, username).Scan(&directCount)
	if err != nil {
		return false, err
	}

	if directCount > 0 {
		return true, nil
	}

	// Check role-based permissions
	return RoleHasAccess(user.Role)
}
