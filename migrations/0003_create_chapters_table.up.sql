-- Create the chapters table
CREATE TABLE IF NOT EXISTS chapters (
    media_slug TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    file TEXT,
    chapter_cover_url TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    released_at INTEGER,
    PRIMARY KEY (media_slug, slug)
);

CREATE INDEX IF NOT EXISTS idx_chapters_media_slug ON chapters(media_slug);
