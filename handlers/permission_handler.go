package handlers

import (
	"fmt"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// PermissionFormData represents form data for creating/updating permissions
type PermissionFormData struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	IsWildcard           bool     `json:"is_wildcard"`
	PremiumChapterAccess bool     `json:"premium_chapter_access"`
	IsEnabled            bool     `json:"is_enabled"`
	Libraries            []string `json:"libraries"`
}

// AssignPermissionToUserFormData represents form data for assigning permissions to users
type AssignPermissionToUserFormData struct {
	Username     string `json:"username"`
	PermissionID string `json:"permission_id"`
}

// AssignPermissionToRoleFormData represents form data for assigning permissions to roles
type AssignPermissionToRoleFormData struct {
	Role         string `json:"role"`
	PermissionID string `json:"permission_id"`
}

// HandleGetPermissions retrieves all permissions and renders the fragment
func HandleGetPermissions(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/permissions")
	}

	permissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		log.Errorf("Failed to get permissions: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.PermissionsList(permissions))
}

// HandleGetPermissionForm renders the create/edit form for a permission
func HandleGetPermissionForm(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/permissions")
	}

	idParam := c.Params("id")

	libraries, err := models.GetLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// If ID is "new", render create form
	if idParam == "new" {
		return handleView(c, views.PermissionForm(nil, libraries))
	}

	// Otherwise, load existing permission for editing
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	permission, err := models.GetPermissionWithLibraries(id)
	if err != nil {
		log.Errorf("Failed to get permission: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	if permission == nil {
		return SendNotFoundError(c, ErrPermissionNotFound)
	}

	return handleView(c, views.PermissionForm(permission, libraries))
}

// HandleCreatePermission creates a new permission
func HandleCreatePermission(c fiber.Ctx) error {
	var formData PermissionFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	name := formData.Name
	description := formData.Description
	isWildcard := formData.IsWildcard
	premiumChapterAccess := formData.PremiumChapterAccess

	if name == "" {
		return SendBadRequestError(c, ErrPermissionNameRequired)
	}

	// Get selected libraries
	var libraries []string
	if !isWildcard {
		libraries = formData.Libraries
	}

	// Create the permission
	permission, err := models.CreatePermission(name, description, isWildcard, premiumChapterAccess)
	if err != nil {
		log.Errorf("Failed to create permission: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Bind to libraries if specified and not wildcard
	if !isWildcard && len(libraries) > 0 {
		err = models.SetLibrariesForPermission(permission.ID, libraries)
		if err != nil {
			log.Errorf("Failed to bind libraries to permission: %v", err)
			return SendInternalServerError(c, ErrPermissionOperationFailed, err)
		}
	}

	// Redirect back to permissions list
	c.Set("HX-Redirect", "/admin/permissions")
	return c.SendStatus(fiber.StatusOK)
}

// HandleUpdatePermission updates an existing permission
func HandleUpdatePermission(c fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	var formData PermissionFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	name := formData.Name
	description := formData.Description
	isWildcard := formData.IsWildcard
	isEnabled := formData.IsEnabled
	premiumChapterAccess := formData.PremiumChapterAccess

	if name == "" {
		return SendBadRequestError(c, ErrPermissionNameRequired)
	}

	// Get selected libraries
	var libraries []string
	if !isWildcard {
		libraries = formData.Libraries
	}

	// Update the permission
	err = models.UpdatePermission(id, name, description, isWildcard, isEnabled, premiumChapterAccess)
	if err != nil {
		log.Errorf("Failed to update permission: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Update library bindings if not wildcard
	if !isWildcard {
		err = models.SetLibrariesForPermission(id, libraries)
		if err != nil {
			log.Errorf("Failed to update library bindings: %v", err)
			return SendInternalServerError(c, ErrPermissionOperationFailed, err)
		}
	} else {
		// If it's wildcard, clear all library bindings
		err = models.SetLibrariesForPermission(id, []string{})
		if err != nil {
			log.Errorf("Failed to clear library bindings: %v", err)
		}
	}

	// Redirect back to permissions list
	c.Set("HX-Redirect", "/admin/permissions")
	return c.SendStatus(fiber.StatusOK)
}

// HandleDeletePermission deletes a permission
func HandleDeletePermission(c fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	err = models.DeletePermission(id)
	if err != nil {
		log.Errorf("Failed to delete permission: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Reload and return updated permissions list
	permissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.PermissionsList(permissions))
}

// HandleAssignPermissionToUser assigns a permission to a user
func HandleAssignPermissionToUser(c fiber.Ctx) error {
	var formData AssignPermissionToUserFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	if formData.Username == "" || formData.PermissionID == "" {
		return SendBadRequestError(c, ErrPermissionUserRequired)
	}

	permissionID, err := strconv.ParseInt(formData.PermissionID, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	// Verify user exists
	user, err := models.FindUserByUsername(formData.Username)
	if err != nil || user == nil {
		return SendNotFoundError(c, ErrUserNotFound)
	}

	// Verify permission exists
	permission, err := models.GetPermission(permissionID)
	if err != nil || permission == nil {
		return SendNotFoundError(c, ErrPermissionNotFound)
	}

	err = models.AssignPermissionToUser(formData.Username, permissionID)
	if err != nil {
		log.Errorf("Failed to assign permission to user: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Return updated user permissions list
	permissions, err := models.GetUserPermissionDetails(formData.Username)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.UserPermissionsList(formData.Username, permissions, allPermissions))
}

// HandleRevokePermissionFromUser removes a permission from a user
func HandleRevokePermissionFromUser(c fiber.Ctx) error {
	username := c.Params("username")
	permissionIDStr := c.Params("permissionId")

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	err = models.RevokePermissionFromUser(username, permissionID)
	if err != nil {
		log.Errorf("Failed to revoke permission from user: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Return updated user permissions list
	permissions, err := models.GetUserPermissionDetails(username)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.UserPermissionsList(username, permissions, allPermissions))
}

// HandleGetUserPermissions retrieves all permissions for a user and renders fragment
func HandleGetUserPermissions(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/permissions")
	}

	username := c.Query("username")
	if username == "" {
		return handleView(c, views.UserPermissionsEmpty())
	}

	// Verify user exists
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return SendNotFoundError(c, ErrUserNotFound)
	}

	permissions, err := models.GetUserPermissionDetails(username)
	if err != nil {
		log.Errorf("Failed to get user permissions: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.UserPermissionsList(username, permissions, allPermissions))
}

// HandleGetBulkAssignForm returns a form for bulk assigning a permission to multiple users
func HandleGetBulkAssignForm(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/permissions")
	}

	permissionID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	permission, err := models.GetPermissionWithLibraries(permissionID)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Get all users
	users, err := models.GetUsers()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Get users who already have this permission
	usersWithPerm, err := models.GetUsersWithPermission(permissionID)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.BulkAssignForm(permission, users, usersWithPerm))
}

// HandleBulkAssignPermission assigns a permission to multiple users at once
func HandleBulkAssignPermission(c fiber.Ctx) error {
	permissionID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Get array of usernames from form
	form, err := c.MultipartForm()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	selectedUsernames := form.Value["usernames[]"]
	if len(selectedUsernames) == 0 {
		// No users selected, just close modal
		triggerCustomNotification(c, "", map[string]any{
			"closeModal": true,
			"showNotification": map[string]string{
				"message": "No users selected",
				"status":  "destructive",
			},
		})
		return c.SendStatus(200)
	}

	successCount := 0
	failCount := 0

	for _, username := range selectedUsernames {
		err := models.AssignPermissionToUser(username, permissionID)
		if err != nil {
			log.Errorf("Failed to assign permission to %s: %v", username, err)
			failCount++
		} else {
			successCount++
		}
	}

	// Close modal and refresh permissions list
	message := fmt.Sprintf("Successfully assigned permission to %d user(s)", successCount)
	if failCount > 0 {
		message += fmt.Sprintf(" (%d failed)", failCount)
	}

	triggerCustomNotification(c, "", map[string]any{
		"closeModal":         true,
		"refreshPermissions": true,
		"showNotification": map[string]string{
			"message": message,
			"status":  "success",
		},
	})
	return c.SendStatus(200)
}

// HandleGetRolePermissions retrieves all permissions for a role and renders fragment
func HandleGetRolePermissions(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !isHTMXRequest(c) {
		return c.Redirect().To("/admin/permissions")
	}

	role := c.Query("role")
	if role == "" {
		return handleView(c, views.RolePermissionsEmpty())
	}

	permissions, err := models.GetRolePermissionDetails(role)
	if err != nil {
		log.Errorf("Failed to get role permissions: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.RolePermissionsList(role, permissions, allPermissions))
}

// HandleAssignPermissionToRole assigns a permission to a role
func HandleAssignPermissionToRole(c fiber.Ctx) error {
	var formData AssignPermissionToRoleFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	if formData.Role == "" || formData.PermissionID == "" {
		return SendBadRequestError(c, ErrPermissionRoleRequired)
	}

	permissionID, err := strconv.ParseInt(formData.PermissionID, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	// Verify permission exists
	permission, err := models.GetPermission(permissionID)
	if err != nil || permission == nil {
		return SendNotFoundError(c, ErrPermissionNotFound)
	}

	err = models.AssignPermissionToRole(formData.Role, permissionID)
	if err != nil {
		log.Errorf("Failed to assign permission to role: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Return updated role permissions list
	permissions, err := models.GetRolePermissionDetails(formData.Role)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.RolePermissionsList(formData.Role, permissions, allPermissions))
}

// HandleRevokePermissionFromRole removes a permission from a role
func HandleRevokePermissionFromRole(c fiber.Ctx) error {
	role := c.Params("role")
	permissionIDStr := c.Params("permissionId")

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return SendBadRequestError(c, ErrPermissionInvalidID)
	}

	err = models.RevokePermissionFromRole(role, permissionID)
	if err != nil {
		log.Errorf("Failed to revoke permission from role: %v", err)
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	// Return updated role permissions list
	permissions, err := models.GetRolePermissionDetails(role)
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return SendInternalServerError(c, ErrPermissionOperationFailed, err)
	}

	return handleView(c, views.RolePermissionsList(role, permissions, allPermissions))
}
