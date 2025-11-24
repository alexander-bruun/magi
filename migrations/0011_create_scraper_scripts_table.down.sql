-- Drop the scraper_scripts table and its indexes
DROP INDEX IF EXISTS idx_scraper_scripts_enabled;
DROP INDEX IF EXISTS idx_scraper_scripts_name;
DROP TABLE IF EXISTS scraper_scripts;