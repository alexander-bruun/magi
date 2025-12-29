-- +migrate Up
CREATE TABLE IF NOT EXISTS user_external_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT NOT NULL,
    service_name TEXT NOT NULL, -- e.g., 'toraka', 'anilist', 'mal', 'mangadex'
    external_user_id TEXT,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_name, service_name)
);
