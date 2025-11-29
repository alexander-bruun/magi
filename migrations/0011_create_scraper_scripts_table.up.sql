CREATE TABLE IF NOT EXISTS scraper_scripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    script TEXT NOT NULL,
    language TEXT NOT NULL CHECK(language IN ('bash', 'python')),
    schedule TEXT DEFAULT '0 0 * * *',
    last_run INTEGER DEFAULT NULL,
    last_run_output TEXT DEFAULT NULL,
    last_run_error TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    enabled BOOLEAN DEFAULT 1,
    variables TEXT DEFAULT '{}',
    packages TEXT DEFAULT '[]',
    shared_script TEXT DEFAULT NULL
);

CREATE INDEX idx_scraper_scripts_name ON scraper_scripts(name);
CREATE INDEX idx_scraper_scripts_enabled ON scraper_scripts(enabled);
