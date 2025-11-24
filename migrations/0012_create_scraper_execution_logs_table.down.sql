-- Drop the scraper_execution_logs table and its indexes
DROP INDEX IF EXISTS idx_scraper_execution_logs_status;
DROP INDEX IF EXISTS idx_scraper_execution_logs_start_time;
DROP INDEX IF EXISTS idx_scraper_execution_logs_script_id;
DROP TABLE IF EXISTS scraper_execution_logs;