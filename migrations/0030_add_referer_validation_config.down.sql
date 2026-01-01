-- Remove referer validation config from app_config
ALTER TABLE app_config DROP COLUMN referer_validation_enabled;
