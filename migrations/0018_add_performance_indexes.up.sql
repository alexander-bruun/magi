-- Add performance indexes for common query patterns

-- Media table indexes for filtering and searching
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);
CREATE INDEX IF NOT EXISTS idx_media_content_rating ON media(content_rating);
CREATE INDEX IF NOT EXISTS idx_media_year ON media(year DESC);
CREATE INDEX IF NOT EXISTS idx_media_name ON media(name);
CREATE INDEX IF NOT EXISTS idx_media_author ON media(author);

-- Media tags table index for tag-based queries
CREATE INDEX IF NOT EXISTS idx_media_tags_tag ON media_tags(tag);

-- Reading states table index for ordering queries
CREATE INDEX IF NOT EXISTS idx_reading_states_user_media_created ON reading_states(user_name, media_slug, created_at DESC);

-- Votes table index for media-based queries
CREATE INDEX IF NOT EXISTS idx_votes_media_slug ON votes(media_slug);
CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at);

-- Reading states table index for date-based queries
CREATE INDEX IF NOT EXISTS idx_reading_states_created_at ON reading_states(created_at);

-- Favorites table index for media-based queries
CREATE INDEX IF NOT EXISTS idx_favorites_media_slug ON favorites(media_slug);