-- Create the mangas table
CREATE TABLE IF NOT EXISTS mangas (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    author TEXT,
    description TEXT,
    year INTEGER,
    original_language TEXT,
    manga_type TEXT DEFAULT 'manga',
    status TEXT,
    content_rating TEXT,
    library_slug TEXT,
    cover_art_url TEXT,
    path TEXT,
    file_count INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
