-- Migration 0029: Remove browser challenge configuration
-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- This is a simplified version that just removes the columns if they exist
-- In practice, SQLite 3.35.0+ supports DROP COLUMN

-- For SQLite 3.35.0+:
-- ALTER TABLE app_config DROP COLUMN browser_challenge_enabled;
-- ALTER TABLE app_config DROP COLUMN browser_challenge_difficulty;
-- ALTER TABLE app_config DROP COLUMN browser_challenge_validity_hours;
-- ALTER TABLE app_config DROP COLUMN browser_challenge_ip_bound;

-- Since we can't easily drop columns in older SQLite, we just leave them as they are harmless
-- The application code will simply not use them after rollback
