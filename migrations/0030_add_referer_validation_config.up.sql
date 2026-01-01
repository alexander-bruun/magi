-- Add referer validation config to app_config
ALTER TABLE app_config ADD COLUMN referer_validation_enabled BOOLEAN DEFAULT FALSE;
