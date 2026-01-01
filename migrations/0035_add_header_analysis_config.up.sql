-- Add header analysis config to app_config
ALTER TABLE app_config ADD COLUMN header_analysis_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN header_analysis_threshold INTEGER DEFAULT 5;
ALTER TABLE app_config ADD COLUMN header_analysis_strict BOOLEAN DEFAULT FALSE;
