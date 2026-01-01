-- Add honeypot config to app_config
ALTER TABLE app_config ADD COLUMN honeypot_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN honeypot_auto_block BOOLEAN DEFAULT TRUE;
ALTER TABLE app_config ADD COLUMN honeypot_block_duration INTEGER DEFAULT 60;
