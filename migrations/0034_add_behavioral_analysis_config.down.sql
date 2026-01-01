-- Remove behavioral analysis config from app_config
ALTER TABLE app_config DROP COLUMN behavioral_analysis_enabled;
ALTER TABLE app_config DROP COLUMN behavioral_score_threshold;
