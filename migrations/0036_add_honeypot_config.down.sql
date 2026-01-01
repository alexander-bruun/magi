-- Remove honeypot config from app_config
ALTER TABLE app_config DROP COLUMN honeypot_enabled;
ALTER TABLE app_config DROP COLUMN honeypot_auto_block;
ALTER TABLE app_config DROP COLUMN honeypot_block_duration;
