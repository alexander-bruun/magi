-- Migration 0010: Create app_config table for global application settings
DROP TABLE IF EXISTS app_config;
CREATE TABLE IF NOT EXISTS app_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_registration INTEGER NOT NULL DEFAULT 1,      -- 1 = true, 0 = false
    max_users INTEGER NOT NULL DEFAULT 1,               -- 0 = unlimited
    content_rating_limit INTEGER NOT NULL DEFAULT 0,    -- 0=safe, 1=suggestive, 2=erotica, 3=explicit (show all)
    metadata_provider TEXT NOT NULL DEFAULT 'mangadex', -- Metadata provider: 'mangadex', 'mal', or 'anilist'
    rate_limit_enabled INTEGER NOT NULL DEFAULT 1,      -- 1 = enabled, 0 = disabled
    rate_limit_requests INTEGER NOT NULL DEFAULT 100,   -- requests per window
    rate_limit_window INTEGER NOT NULL DEFAULT 60,      -- window in seconds
    rate_limit_block_duration INTEGER NOT NULL DEFAULT 300, -- block duration in seconds
    bot_detection_enabled INTEGER NOT NULL DEFAULT 1,   -- 1 = enabled, 0 = disabled
    bot_series_threshold INTEGER NOT NULL DEFAULT 5,    -- max series accesses per time window
    bot_chapter_threshold INTEGER NOT NULL DEFAULT 10,  -- max chapter accesses per time window
    bot_detection_window INTEGER NOT NULL DEFAULT 60,   -- time window in seconds for bot detection
    bot_ban_duration INTEGER NOT NULL DEFAULT 300,       -- how long bot bans last in seconds
    image_access_secret TEXT NOT NULL DEFAULT '',
    premium_early_access_duration INTEGER NOT NULL DEFAULT 3600, -- default 1 hour in seconds
    max_premium_chapters INTEGER NOT NULL DEFAULT 3,
    premium_cooldown_scaling_enabled INTEGER NOT NULL DEFAULT 0,
    maintenance_enabled INTEGER NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    maintenance_message TEXT NOT NULL DEFAULT 'We are currently performing maintenance. Please check back later.', -- Custom maintenance message
    new_badge_duration INTEGER NOT NULL DEFAULT 48, -- Duration in hours that media is marked as NEW after update
    stripe_enabled INTEGER NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    stripe_publishable_key TEXT NOT NULL DEFAULT '',
    stripe_secret_key TEXT NOT NULL DEFAULT '',
    stripe_webhook_secret TEXT NOT NULL DEFAULT '',
    honeypot_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    honeypot_auto_block BOOLEAN NOT NULL DEFAULT 1, -- 1 = enabled, 0 = disabled
    honeypot_auto_ban BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    honeypot_block_duration INTEGER NOT NULL DEFAULT 60, -- block duration in minutes
    parallel_indexing_enabled INTEGER NOT NULL DEFAULT 1, -- 1 = enabled, 0 = disabled
    parallel_indexing_threshold INTEGER NOT NULL DEFAULT 100, -- minimum series count to trigger parallel processing
    browser_challenge_enabled INTEGER NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    browser_challenge_difficulty INTEGER NOT NULL DEFAULT 3, -- proof-of-work difficulty
    browser_challenge_validity_hours INTEGER NOT NULL DEFAULT 24, -- validity period in hours
    browser_challenge_ip_bound INTEGER NOT NULL DEFAULT 0, -- 1 = bind to IP, 0 = don't bind
    referer_validation_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    tarpit_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    tarpit_max_delay INTEGER NOT NULL DEFAULT 5000, -- maximum delay in milliseconds
    timing_analysis_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    timing_variance_threshold REAL NOT NULL DEFAULT 0.1, -- coefficient of variation threshold
    tls_fingerprint_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    tls_fingerprint_strict BOOLEAN NOT NULL DEFAULT 0, -- 1 = strict mode, 0 = lenient
    behavioral_analysis_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    behavioral_score_threshold INTEGER NOT NULL DEFAULT 40, -- score threshold for flagging
    header_analysis_enabled BOOLEAN NOT NULL DEFAULT 0, -- 1 = enabled, 0 = disabled
    header_analysis_threshold INTEGER NOT NULL DEFAULT 5, -- suspicion score threshold
    header_analysis_strict BOOLEAN NOT NULL DEFAULT 0, -- 1 = block suspicious, 0 = don't block
    discord_invite_link TEXT NOT NULL DEFAULT '',
    metadata_embedding_enabled BOOLEAN NOT NULL DEFAULT 0
);

-- Ensure exactly one row exists (id = 1)
INSERT INTO app_config (id, allow_registration, max_users, content_rating_limit, metadata_provider, 
    rate_limit_enabled, rate_limit_requests, rate_limit_window, rate_limit_block_duration, bot_detection_enabled, bot_series_threshold, bot_chapter_threshold, bot_detection_window, bot_ban_duration,
    image_access_secret, premium_early_access_duration, max_premium_chapters, premium_cooldown_scaling_enabled, maintenance_enabled, maintenance_message, new_badge_duration, stripe_enabled, stripe_publishable_key, stripe_secret_key, stripe_webhook_secret, honeypot_enabled, honeypot_auto_block, honeypot_auto_ban, honeypot_block_duration, parallel_indexing_enabled, parallel_indexing_threshold, browser_challenge_enabled, browser_challenge_difficulty, browser_challenge_validity_hours, browser_challenge_ip_bound, referer_validation_enabled, tarpit_enabled, tarpit_max_delay, timing_analysis_enabled, timing_variance_threshold, tls_fingerprint_enabled, tls_fingerprint_strict, behavioral_analysis_enabled, behavioral_score_threshold, header_analysis_enabled, header_analysis_threshold, header_analysis_strict, discord_invite_link, metadata_embedding_enabled)
SELECT 1, 1, 0, 3, 'mangadex', 1, 100, 60, 300, 1, 5, 10, 60, 300, '', 3600, 3, 1, 0, 'We are currently performing maintenance. Please check back later.', 48, 0, '', '', '', 0, 1, 0, 60, 1, 100, 0, 3, 24, 0, 0, 0, 5000, 0, 0.1, 0, 0, 0, 40, 0, 5, 0, '', 0
WHERE NOT EXISTS (SELECT 1 FROM app_config WHERE id = 1);