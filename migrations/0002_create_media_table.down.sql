-- Drop triggers
DROP TRIGGER IF EXISTS update_media_content_rating_level;
DROP TRIGGER IF EXISTS media_fts_update;
DROP TRIGGER IF EXISTS media_fts_delete;
DROP TRIGGER IF EXISTS media_fts_insert;

-- Drop indexes
DROP INDEX IF EXISTS idx_media_status;
DROP INDEX IF EXISTS idx_media_type;
DROP INDEX IF EXISTS idx_media_name;
DROP INDEX IF EXISTS idx_media_created_at;

-- Drop tables
DROP TABLE IF EXISTS media_fts;
DROP TABLE IF EXISTS media;