-- Drop indexes
DROP INDEX IF EXISTS idx_permissions_enabled;
DROP INDEX IF EXISTS idx_user_permissions_username;
DROP INDEX IF EXISTS idx_library_permissions_library;

-- Drop tables in reverse order
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS library_permissions;
DROP TABLE IF EXISTS permissions;
