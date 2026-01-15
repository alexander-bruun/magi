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
    cover_art_url TEXT,
    file_count INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    start_date TEXT,
    end_date TEXT,
    chapter_count INTEGER,
    volume_count INTEGER,
    average_score REAL,
    popularity INTEGER,
    favorites INTEGER,
    demographic TEXT,
    publisher TEXT,
    magazine TEXT,
    serialization TEXT,
    authors TEXT, -- JSON array
    artists TEXT, -- JSON array
    genres TEXT, -- JSON array
    characters TEXT, -- JSON array
    alternative_titles TEXT, -- JSON array
    attribution_links TEXT, -- JSON array
    potential_poster_urls TEXT -- JSON array
);
