-- Create the libraries table
CREATE TABLE IF NOT EXISTS libraries (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    cron TEXT NOT NULL,
    folders TEXT,  -- Serialized JSON array as text
    metadata_provider TEXT,  -- Optional: mangadex, mal, anilist, jikan
    created_at INTEGER NOT NULL,  -- Unix timestamp
    updated_at INTEGER NOT NULL   -- Unix timestamp
);
