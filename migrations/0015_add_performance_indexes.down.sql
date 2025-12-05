-- Rollback: Drop all performance indexes
DROP INDEX IF EXISTS idx_media_created_at;
DROP INDEX IF EXISTS idx_chapters_media_slug;
DROP INDEX IF EXISTS idx_chapters_media_slug_type;
DROP INDEX IF EXISTS idx_reviews_media_slug;
DROP INDEX IF EXISTS idx_reading_states_user_media;
DROP INDEX IF EXISTS idx_media_tags_tag;
DROP INDEX IF EXISTS idx_media_tags_media_slug;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_session_tokens_token;
DROP INDEX IF EXISTS idx_reading_history_user_media;
