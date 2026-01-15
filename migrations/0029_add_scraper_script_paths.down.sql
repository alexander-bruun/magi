-- Remove script_path and requirements_path columns from scraper_scripts table
ALTER TABLE scraper_scripts DROP COLUMN script_path;
ALTER TABLE scraper_scripts DROP COLUMN requirements_path;