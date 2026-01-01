-- Remove tarpit config from app_config
ALTER TABLE app_config DROP COLUMN tarpit_enabled;
ALTER TABLE app_config DROP COLUMN tarpit_max_delay;
