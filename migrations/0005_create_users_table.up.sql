-- Create the users table
CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    refresh_token_version INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL CHECK (role IN ('reader', 'moderator', 'admin')),
    banned BOOLEAN NOT NULL DEFAULT FALSE
);
