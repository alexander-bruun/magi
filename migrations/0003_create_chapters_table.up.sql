-- Create the chapters table
CREATE TABLE IF NOT EXISTS chapters (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4)))),
    media_slug TEXT NOT NULL,
    library_slug TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    file TEXT,
    chapter_cover_url TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    released_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_chapters_media_slug ON chapters(media_slug);
CREATE INDEX IF NOT EXISTS idx_chapters_library_slug ON chapters(library_slug);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chapters_unique ON chapters(media_slug, library_slug, slug);
