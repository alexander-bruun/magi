-- Migration 0029: Add browser challenge configuration
ALTER TABLE app_config ADD COLUMN browser_challenge_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_config ADD COLUMN browser_challenge_difficulty INTEGER NOT NULL DEFAULT 3;
ALTER TABLE app_config ADD COLUMN browser_challenge_validity_hours INTEGER NOT NULL DEFAULT 24;
ALTER TABLE app_config ADD COLUMN browser_challenge_ip_bound INTEGER NOT NULL DEFAULT 0;
