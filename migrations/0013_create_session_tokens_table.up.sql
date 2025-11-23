-- Create the session_tokens table
CREATE TABLE IF NOT EXISTS session_tokens (
    token TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    last_used_at DATETIME NOT NULL,
    FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
);

-- Create index for faster lookups by username
CREATE INDEX IF NOT EXISTS idx_session_tokens_username ON session_tokens(username);

-- Create index for faster cleanup of expired tokens
CREATE INDEX IF NOT EXISTS idx_session_tokens_expires_at ON session_tokens(expires_at);
