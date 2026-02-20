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

-- Trigger to maintain primary_library_slug in media
DROP TRIGGER IF EXISTS update_media_primary_library;
CREATE TRIGGER update_media_primary_library AFTER INSERT ON chapters
BEGIN
    UPDATE media SET primary_library_slug = NEW.library_slug
    WHERE slug = NEW.media_slug AND (primary_library_slug IS NULL OR primary_library_slug = '');
END;

DROP TRIGGER IF EXISTS update_media_primary_library_delete;
CREATE TRIGGER update_media_primary_library_delete AFTER DELETE ON chapters
BEGIN
    UPDATE media SET primary_library_slug = (
        SELECT library_slug FROM chapters WHERE media_slug = OLD.media_slug LIMIT 1
    ) WHERE slug = OLD.media_slug;
END;
