-- Add timing analysis config to app_config
ALTER TABLE app_config ADD COLUMN timing_analysis_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN timing_variance_threshold REAL DEFAULT 0.1;
