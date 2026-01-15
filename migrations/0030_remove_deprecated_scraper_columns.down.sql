-- Add back deprecated columns to scraper_scripts table
ALTER TABLE scraper_scripts ADD COLUMN script TEXT DEFAULT NULL;
ALTER TABLE scraper_scripts ADD COLUMN shared_script TEXT DEFAULT NULL;
ALTER TABLE scraper_scripts ADD COLUMN packages TEXT DEFAULT '[]';