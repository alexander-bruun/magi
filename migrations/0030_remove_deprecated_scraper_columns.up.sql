-- Remove deprecated columns from scraper_scripts table
ALTER TABLE scraper_scripts DROP COLUMN script;
ALTER TABLE scraper_scripts DROP COLUMN shared_script;
ALTER TABLE scraper_scripts DROP COLUMN packages;