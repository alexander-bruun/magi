-- Add indexes to optimize media search performance
CREATE INDEX IF NOT EXISTS idx_media_name ON media(name);
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);
CREATE INDEX IF NOT EXISTS idx_media_content_rating ON media(content_rating);
CREATE INDEX IF NOT EXISTS idx_media_year ON media(year);
CREATE INDEX IF NOT EXISTS idx_chapters_library_media ON chapters(library_slug, media_slug);
CREATE INDEX IF NOT EXISTS idx_votes_media_slug ON votes(media_slug);
CREATE INDEX IF NOT EXISTS idx_reading_states_media_slug ON reading_states(media_slug);
CREATE INDEX IF NOT EXISTS idx_media_tags_media_tag ON media_tags(media_slug, tag);