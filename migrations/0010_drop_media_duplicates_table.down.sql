-- Drop media_duplicates table
DROP INDEX IF EXISTS idx_media_duplicates_dismissed;
DROP INDEX IF EXISTS idx_media_duplicates_library_slug;
DROP INDEX IF EXISTS idx_media_duplicates_media_slug;
DROP TABLE IF EXISTS media_duplicates;
