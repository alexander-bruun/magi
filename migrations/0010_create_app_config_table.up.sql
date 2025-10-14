-- Migration 0010: Create app_config table for global application settings
CREATE TABLE IF NOT EXISTS app_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_registration INTEGER NOT NULL DEFAULT 1, -- 1 = true, 0 = false
    max_users INTEGER NOT NULL DEFAULT 0           -- 0 = unlimited
);

-- Ensure exactly one row exists (id = 1)
INSERT INTO app_config (id, allow_registration, max_users)
SELECT 1, 1, 0
WHERE NOT EXISTS (SELECT 1 FROM app_config WHERE id = 1);