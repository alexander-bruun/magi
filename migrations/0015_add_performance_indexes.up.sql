-- Performance Indexes for High-Load Scenarios
-- These indexes significantly speed up the most common queries

-- Index for media created_at (used in recently added queries)
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC);

-- Index for chapters by media_slug (used in chapter lookups, premium checks, latest chapter queries)
CREATE INDEX IF NOT EXISTS idx_chapters_media_slug ON chapters(media_slug);

-- Index for chapters by media_slug and type (used in premium chapter filtering)
CREATE INDEX IF NOT EXISTS idx_chapters_media_slug_type ON chapters(media_slug, type);

-- Index for reading_states (used in user progress tracking)
CREATE INDEX IF NOT EXISTS idx_reading_states_user_media ON reading_states(user_name, media_slug);

-- Index for media_tags for tag-based searches
CREATE INDEX IF NOT EXISTS idx_media_tags_tag ON media_tags(tag);

-- Index for media_tags by media_slug
CREATE INDEX IF NOT EXISTS idx_media_tags_media_slug ON media_tags(media_slug);

-- Index for user lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Index for session tokens
CREATE INDEX IF NOT EXISTS idx_session_tokens_token ON session_tokens(token);
