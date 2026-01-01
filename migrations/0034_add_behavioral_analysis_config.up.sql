-- Add behavioral analysis config to app_config
ALTER TABLE app_config ADD COLUMN behavioral_analysis_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN behavioral_score_threshold INTEGER DEFAULT 40;
