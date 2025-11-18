-- Migration 0010: Create app_config table for global application settings
CREATE TABLE IF NOT EXISTS app_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_registration INTEGER NOT NULL DEFAULT 1,      -- 1 = true, 0 = false
    max_users INTEGER NOT NULL DEFAULT 1,               -- 0 = unlimited
    content_rating_limit INTEGER NOT NULL DEFAULT 0,    -- 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
    metadata_provider TEXT NOT NULL DEFAULT 'mangadex', -- Metadata provider: 'mangadex', 'mal', or 'anilist'
    mal_api_token TEXT NOT NULL DEFAULT '',             -- MyAnimeList API token
    anilist_api_token TEXT NOT NULL DEFAULT ''          -- AniList API token
);

-- Ensure exactly one row exists (id = 1)
INSERT INTO app_config (id, allow_registration, max_users, content_rating_limit, metadata_provider, mal_api_token, anilist_api_token)
SELECT 1, 1, 0, 3, 'mangadex', '', ''
WHERE NOT EXISTS (SELECT 1 FROM app_config WHERE id = 1);