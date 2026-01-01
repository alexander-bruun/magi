-- Remove header analysis config from app_config
ALTER TABLE app_config DROP COLUMN header_analysis_enabled;
ALTER TABLE app_config DROP COLUMN header_analysis_threshold;
ALTER TABLE app_config DROP COLUMN header_analysis_strict;
