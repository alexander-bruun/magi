-- Add tarpit config to app_config
ALTER TABLE app_config ADD COLUMN tarpit_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN tarpit_max_delay INTEGER DEFAULT 5000;
