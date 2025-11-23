-- Remove performance indexes for indexer operations
DROP INDEX IF EXISTS idx_media_slug;
DROP INDEX IF EXISTS idx_media_library_slug;
DROP INDEX IF EXISTS idx_chapters_media_slug;