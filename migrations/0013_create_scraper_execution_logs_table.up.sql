CREATE TABLE IF NOT EXISTS scraper_execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    script_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('running', 'success', 'error', 'aborted')),
    output TEXT DEFAULT NULL,
    error_message TEXT DEFAULT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER DEFAULT NULL,
    duration_ms INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (script_id) REFERENCES scraper_scripts(id) ON DELETE CASCADE
);

CREATE INDEX idx_scraper_execution_logs_script_id ON scraper_execution_logs(script_id);
CREATE INDEX idx_scraper_execution_logs_start_time ON scraper_execution_logs(start_time DESC);
CREATE INDEX idx_scraper_execution_logs_status ON scraper_execution_logs(status);
