-- Create the users table
CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('reader', 'moderator', 'admin')),
    banned BOOLEAN NOT NULL DEFAULT FALSE
);
