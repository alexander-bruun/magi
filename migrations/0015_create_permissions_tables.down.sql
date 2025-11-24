-- Remove the inserted data
DELETE FROM role_permissions WHERE role = 'anonymous' AND permission_id IN (SELECT id FROM permissions WHERE name = 'all' AND is_wildcard = TRUE);
DELETE FROM permissions WHERE name = 'all' AND is_wildcard = TRUE;

-- Drop the permissions tables
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS library_permissions;
DROP TABLE IF EXISTS permissions;