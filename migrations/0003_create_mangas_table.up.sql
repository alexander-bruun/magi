CREATE TABLE IF NOT EXISTS mangas (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    author TEXT,
    description TEXT,
    year INTEGER,
    original_language TEXT,
    status TEXT,
    content_rating TEXT,
    library_slug TEXT,
    cover_art_url TEXT,
    path TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
