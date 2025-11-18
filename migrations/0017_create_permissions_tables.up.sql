-- Create the permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_wildcard BOOLEAN NOT NULL DEFAULT FALSE,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Create the library_permissions table (links permissions to libraries)
CREATE TABLE IF NOT EXISTS library_permissions (
    permission_id INTEGER NOT NULL,
    library_slug TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (permission_id, library_slug),
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE,
    FOREIGN KEY (library_slug) REFERENCES libraries(slug) ON DELETE CASCADE
);

-- Create the user_permissions table (assigns permissions to users)
CREATE TABLE IF NOT EXISTS user_permissions (
    username TEXT NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (username, permission_id),
    FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

-- Create the role_permissions table (assigns permissions to roles)
CREATE TABLE IF NOT EXISTS role_permissions (
    role TEXT NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (role, permission_id),
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

-- Create a default wildcard permission for read-all access
INSERT INTO permissions (name, description, is_wildcard, is_enabled, created_at, updated_at)
VALUES ('all', 'Access to all libraries', TRUE, TRUE, strftime('%s', 'now'), strftime('%s', 'now'));

-- Assign the wildcard 'all' permission to the anonymous role
-- This grants unauthenticated users access to all libraries by default
INSERT INTO role_permissions (role, permission_id, created_at)
SELECT 'anonymous', id, strftime('%s', 'now')
FROM permissions
WHERE name = 'all' AND is_wildcard = TRUE;

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_library_permissions_library ON library_permissions(library_slug);
CREATE INDEX IF NOT EXISTS idx_user_permissions_username ON user_permissions(username);
CREATE INDEX IF NOT EXISTS idx_permissions_enabled ON permissions(is_enabled);
