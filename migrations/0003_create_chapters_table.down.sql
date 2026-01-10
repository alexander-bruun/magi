-- Drop the chapters table and its indexes
DROP INDEX IF EXISTS idx_chapters_media_slug;
DROP INDEX IF EXISTS idx_chapters_library_slug;
DROP INDEX IF EXISTS idx_chapters_unique;
DROP TABLE IF EXISTS chapters;