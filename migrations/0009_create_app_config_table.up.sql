-- Migration 0010: Create app_config table for global application settings
CREATE TABLE IF NOT EXISTS app_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_registration INTEGER NOT NULL DEFAULT 1,      -- 1 = true, 0 = false
    max_users INTEGER NOT NULL DEFAULT 1,               -- 0 = unlimited
    content_rating_limit INTEGER NOT NULL DEFAULT 0,    -- 0=safe, 1=suggestive, 2=erotica, 3=pornographic (show all)
    metadata_provider TEXT NOT NULL DEFAULT 'mangadex', -- Metadata provider: 'mangadex', 'mal', or 'anilist'
    mal_api_token TEXT NOT NULL DEFAULT '',             -- MyAnimeList API token
    anilist_api_token TEXT NOT NULL DEFAULT '',         -- AniList API token
    rate_limit_enabled INTEGER NOT NULL DEFAULT 1,      -- 1 = enabled, 0 = disabled
    rate_limit_requests INTEGER NOT NULL DEFAULT 100,   -- requests per window
    rate_limit_window INTEGER NOT NULL DEFAULT 60,      -- window in seconds
    bot_detection_enabled INTEGER NOT NULL DEFAULT 1,   -- 1 = enabled, 0 = disabled
    bot_series_threshold INTEGER NOT NULL DEFAULT 5,    -- max series accesses per time window
    bot_chapter_threshold INTEGER NOT NULL DEFAULT 10,  -- max chapter accesses per time window
    bot_detection_window INTEGER NOT NULL DEFAULT 60,   -- time window in seconds for bot detection
    image_access_secret TEXT NOT NULL DEFAULT '',
    reader_compression_quality INTEGER NOT NULL DEFAULT 70,
    moderator_compression_quality INTEGER NOT NULL DEFAULT 85,
    admin_compression_quality INTEGER NOT NULL DEFAULT 100,
    anonymous_compression_quality INTEGER NOT NULL DEFAULT 70,
    processed_image_quality INTEGER NOT NULL DEFAULT 85
);

-- Ensure exactly one row exists (id = 1)
INSERT INTO app_config (id, allow_registration, max_users, content_rating_limit, metadata_provider, mal_api_token, anilist_api_token, 
    rate_limit_enabled, rate_limit_requests, rate_limit_window, bot_detection_enabled, bot_series_threshold, bot_chapter_threshold, bot_detection_window,
    image_access_secret, reader_compression_quality, moderator_compression_quality, admin_compression_quality, anonymous_compression_quality, processed_image_quality)
SELECT 1, 1, 0, 3, 'mangadex', '', '', 1, 100, 60, 1, 5, 10, 60, '', 70, 85, 100, 70, 85
WHERE NOT EXISTS (SELECT 1 FROM app_config WHERE id = 1);