-- Drop manga_duplicates table
DROP INDEX IF EXISTS idx_manga_duplicates_dismissed;
DROP INDEX IF EXISTS idx_manga_duplicates_library_slug;
DROP INDEX IF EXISTS idx_manga_duplicates_manga_slug;
DROP TABLE IF EXISTS manga_duplicates;
