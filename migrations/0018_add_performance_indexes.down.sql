-- Remove performance indexes

DROP INDEX IF EXISTS idx_media_type;
DROP INDEX IF EXISTS idx_media_status;
DROP INDEX IF EXISTS idx_media_content_rating;
DROP INDEX IF EXISTS idx_media_year;
DROP INDEX IF EXISTS idx_media_name;
DROP INDEX IF EXISTS idx_media_author;
DROP INDEX IF EXISTS idx_media_tags_tag;
DROP INDEX IF EXISTS idx_reading_states_user_media_created;
DROP INDEX IF EXISTS idx_votes_media_slug;
DROP INDEX IF EXISTS idx_favorites_media_slug;