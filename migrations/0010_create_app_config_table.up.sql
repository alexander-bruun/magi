-- Migration 0010: Create app_config table for global application settings
CREATE TABLE IF NOT EXISTS app_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_registration INTEGER NOT NULL DEFAULT 1,   -- 1 = true, 0 = false
    max_users INTEGER NOT NULL DEFAULT 0,            -- 0 = unlimited
    content_rating_limit INTEGER NOT NULL DEFAULT 3  -- 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
);

-- Ensure exactly one row exists (id = 1)
INSERT INTO app_config (id, allow_registration, max_users, content_rating_limit)
SELECT 1, 1, 0, 3
WHERE NOT EXISTS (SELECT 1 FROM app_config WHERE id = 1);