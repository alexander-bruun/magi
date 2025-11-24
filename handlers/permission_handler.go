package handlers

import (
	"fmt"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// HandleGetPermissions retrieves all permissions and renders the fragment
func HandleGetPermissions(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/permissions")
	}

	permissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		log.Errorf("Failed to get permissions: %v", err)
		return handleError(c, err)
	}

	return HandleView(c, views.PermissionsList(permissions))
}

// HandleGetPermissionForm renders the create/edit form for a permission
func HandleGetPermissionForm(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/permissions")
	}

	idParam := c.Params("id")
	
	libraries, err := models.GetLibraries()
	if err != nil {
		return handleError(c, err)
	}
	
	// If ID is "new", render create form
	if idParam == "new" {
		return HandleView(c, views.PermissionForm(nil, libraries))
	}
	
	// Otherwise, load existing permission for editing
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	permission, err := models.GetPermissionWithLibraries(id)
	if err != nil {
		log.Errorf("Failed to get permission: %v", err)
		return handleError(c, err)
	}

	if permission == nil {
		return handleErrorWithStatus(c, fmt.Errorf("permission not found"), fiber.StatusNotFound)
	}

	return HandleView(c, views.PermissionForm(permission, libraries))
}

// HandleCreatePermission creates a new permission
func HandleCreatePermission(c *fiber.Ctx) error {
	name := c.FormValue("name")
	description := c.FormValue("description")
	isWildcard := c.FormValue("is_wildcard") == "on"
	
	if name == "" {
		return handleErrorWithStatus(c, fmt.Errorf("permission name is required"), fiber.StatusBadRequest)
	}

	// Get selected libraries
	var libraries []string
	if !isWildcard {
		libraryValues := c.Request().PostArgs().PeekMulti("libraries[]")
		for _, lib := range libraryValues {
			libraries = append(libraries, string(lib))
		}
	}

	// Create the permission
	permission, err := models.CreatePermission(name, description, isWildcard)
	if err != nil {
		log.Errorf("Failed to create permission: %v", err)
		return handleError(c, err)
	}

	// Bind to libraries if specified and not wildcard
	if !isWildcard && len(libraries) > 0 {
		err = models.SetLibrariesForPermission(permission.ID, libraries)
		if err != nil {
			log.Errorf("Failed to bind libraries to permission: %v", err)
			return handleError(c, err)
		}
	}

	// Redirect back to permissions list
	c.Set("HX-Redirect", "/admin/permissions")
	return c.SendStatus(fiber.StatusOK)
}

// HandleUpdatePermission updates an existing permission
func HandleUpdatePermission(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	name := c.FormValue("name")
	description := c.FormValue("description")
	isWildcard := c.FormValue("is_wildcard") == "on"
	isEnabled := c.FormValue("is_enabled") == "on"
	
	if name == "" {
		return handleErrorWithStatus(c, fmt.Errorf("permission name is required"), fiber.StatusBadRequest)
	}

	// Get selected libraries
	var libraries []string
	if !isWildcard {
		libraryValues := c.Request().PostArgs().PeekMulti("libraries[]")
		for _, lib := range libraryValues {
			libraries = append(libraries, string(lib))
		}
	}

	// Update the permission
	err = models.UpdatePermission(id, name, description, isWildcard, isEnabled)
	if err != nil {
		log.Errorf("Failed to update permission: %v", err)
		return handleError(c, err)
	}

	// Update library bindings if not wildcard
	if !isWildcard {
		err = models.SetLibrariesForPermission(id, libraries)
		if err != nil {
			log.Errorf("Failed to update library bindings: %v", err)
			return handleError(c, err)
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
func HandleDeletePermission(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	err = models.DeletePermission(id)
	if err != nil {
		log.Errorf("Failed to delete permission: %v", err)
		return handleError(c, err)
	}

	// Reload and return updated permissions list
	permissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.PermissionsList(permissions))
}

// HandleAssignPermissionToUser assigns a permission to a user
func HandleAssignPermissionToUser(c *fiber.Ctx) error {
	username := c.FormValue("username")
	permissionIDStr := c.FormValue("permission_id")
	
	if username == "" || permissionIDStr == "" {
		return handleErrorWithStatus(c, fmt.Errorf("username and permission are required"), fiber.StatusBadRequest)
	}

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	// Verify user exists
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return handleErrorWithStatus(c, fmt.Errorf("user not found"), fiber.StatusNotFound)
	}

	// Verify permission exists
	permission, err := models.GetPermission(permissionID)
	if err != nil || permission == nil {
		return handleErrorWithStatus(c, fmt.Errorf("permission not found"), fiber.StatusNotFound)
	}

	err = models.AssignPermissionToUser(username, permissionID)
	if err != nil {
		log.Errorf("Failed to assign permission to user: %v", err)
		return handleError(c, err)
	}

	// Return updated user permissions list
	permissions, err := models.GetUserPermissionDetails(username)
	if err != nil {
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UserPermissionsList(username, permissions, allPermissions))
}

// HandleRevokePermissionFromUser removes a permission from a user
func HandleRevokePermissionFromUser(c *fiber.Ctx) error {
	username := c.Params("username")
	permissionIDStr := c.Params("permissionId")

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	err = models.RevokePermissionFromUser(username, permissionID)
	if err != nil {
		log.Errorf("Failed to revoke permission from user: %v", err)
		return handleError(c, err)
	}

	// Return updated user permissions list
	permissions, err := models.GetUserPermissionDetails(username)
	if err != nil {
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UserPermissionsList(username, permissions, allPermissions))
}

// HandleGetUserPermissions retrieves all permissions for a user and renders fragment
func HandleGetUserPermissions(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/permissions")
	}

	username := c.Query("username")
	if username == "" {
		return HandleView(c, views.UserPermissionsEmpty())
	}

	// Verify user exists
	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil {
		return handleErrorWithStatus(c, fmt.Errorf("user not found"), fiber.StatusNotFound)
	}

	permissions, err := models.GetUserPermissionDetails(username)
	if err != nil {
		log.Errorf("Failed to get user permissions: %v", err)
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UserPermissionsList(username, permissions, allPermissions))
}

// HandleGetBulkAssignForm returns a form for bulk assigning a permission to multiple users
func HandleGetBulkAssignForm(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/permissions")
	}

	permissionID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleError(c, err)
	}

	permission, err := models.GetPermissionWithLibraries(permissionID)
	if err != nil {
		return handleError(c, err)
	}

	// Get all users
	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	// Get users who already have this permission
	usersWithPerm, err := models.GetUsersWithPermission(permissionID)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.BulkAssignForm(permission, users, usersWithPerm))
}

// HandleBulkAssignPermission assigns a permission to multiple users at once
func HandleBulkAssignPermission(c *fiber.Ctx) error {
	permissionID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return handleError(c, err)
	}

	// Get array of usernames from form
	form, err := c.MultipartForm()
	if err != nil {
		return handleError(c, err)
	}

	selectedUsernames := form.Value["usernames[]"]
	if len(selectedUsernames) == 0 {
		// No users selected, just close modal
		c.Set("HX-Trigger", `{"closeModal": true, "showNotification": {"message": "No users selected", "type": "warning"}}`)
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

	c.Set("HX-Trigger", `{"closeModal": true, "refreshPermissions": true, "showNotification": {"message": "`+message+`", "type": "success"}}`)
	return c.SendStatus(200)
}

// HandleGetRolePermissions retrieves all permissions for a role and renders fragment
func HandleGetRolePermissions(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the permissions management page
	if !IsHTMXRequest(c) {
		return c.Redirect("/admin/permissions")
	}

	role := c.Query("role")
	if role == "" {
		return HandleView(c, views.RolePermissionsEmpty())
	}

	permissions, err := models.GetRolePermissionDetails(role)
	if err != nil {
		log.Errorf("Failed to get role permissions: %v", err)
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.RolePermissionsList(role, permissions, allPermissions))
}

// HandleAssignPermissionToRole assigns a permission to a role
func HandleAssignPermissionToRole(c *fiber.Ctx) error {
	role := c.FormValue("role")
	permissionIDStr := c.FormValue("permission_id")

	if role == "" || permissionIDStr == "" {
		return handleErrorWithStatus(c, fmt.Errorf("role and permission are required"), fiber.StatusBadRequest)
	}

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	// Verify permission exists
	permission, err := models.GetPermission(permissionID)
	if err != nil || permission == nil {
		return handleErrorWithStatus(c, fmt.Errorf("permission not found"), fiber.StatusNotFound)
	}

	err = models.AssignPermissionToRole(role, permissionID)
	if err != nil {
		log.Errorf("Failed to assign permission to role: %v", err)
		return handleError(c, err)
	}

	// Return updated role permissions list
	permissions, err := models.GetRolePermissionDetails(role)
	if err != nil {
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.RolePermissionsList(role, permissions, allPermissions))
}

// HandleRevokePermissionFromRole removes a permission from a role
func HandleRevokePermissionFromRole(c *fiber.Ctx) error {
	role := c.Params("role")
	permissionIDStr := c.Params("permissionId")

	permissionID, err := strconv.ParseInt(permissionIDStr, 10, 64)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("invalid permission ID"), fiber.StatusBadRequest)
	}

	err = models.RevokePermissionFromRole(role, permissionID)
	if err != nil {
		log.Errorf("Failed to revoke permission from role: %v", err)
		return handleError(c, err)
	}

	// Return updated role permissions list
	permissions, err := models.GetRolePermissionDetails(role)
	if err != nil {
		return handleError(c, err)
	}

	allPermissions, err := models.GetAllPermissionsWithLibraries()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.RolePermissionsList(role, permissions, allPermissions))
}
