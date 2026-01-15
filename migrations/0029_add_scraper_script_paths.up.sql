-- Add script_path and requirements_path columns to scraper_scripts table
ALTER TABLE scraper_scripts ADD COLUMN script_path TEXT DEFAULT NULL;
ALTER TABLE scraper_scripts ADD COLUMN requirements_path TEXT DEFAULT NULL;