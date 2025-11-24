-- Create the media table
CREATE TABLE IF NOT EXISTS media (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    author TEXT,
    description TEXT,
    year INTEGER,
    original_language TEXT,
    type TEXT DEFAULT 'manga',
    status TEXT,
    content_rating TEXT,
    library_slug TEXT,
    cover_art_url TEXT,
    path TEXT,
    file_count INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_media_library_slug ON media(library_slug);
