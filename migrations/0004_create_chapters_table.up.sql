-- Create the chapters table
CREATE TABLE IF NOT EXISTS chapters (
    manga_slug TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    file TEXT,
    chapter_cover_url TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    PRIMARY KEY (manga_slug, slug)
);
