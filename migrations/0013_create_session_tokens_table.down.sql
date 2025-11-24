-- Drop the session_tokens table and its indexes
DROP INDEX IF EXISTS idx_session_tokens_expires_at;
DROP INDEX IF EXISTS idx_session_tokens_username;
DROP TABLE IF EXISTS session_tokens;