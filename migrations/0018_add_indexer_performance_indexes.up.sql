-- Add performance indexes for indexer operations
CREATE INDEX IF NOT EXISTS idx_media_slug ON media(slug);
CREATE INDEX IF NOT EXISTS idx_media_library_slug ON media(library_slug);
CREATE INDEX IF NOT EXISTS idx_chapters_media_slug ON chapters(media_slug);