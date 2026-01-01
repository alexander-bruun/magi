-- Remove timing analysis config from app_config
ALTER TABLE app_config DROP COLUMN timing_analysis_enabled;
ALTER TABLE app_config DROP COLUMN timing_variance_threshold;
