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
    potential_poster_urls TEXT, -- JSON array
    vote_score INTEGER DEFAULT 0,
    primary_library_slug TEXT,
    content_rating_level INTEGER
);

-- Create FTS table for media search
DROP TABLE IF EXISTS media_fts;
CREATE VIRTUAL TABLE media_fts USING fts5(
    slug UNINDEXED,
    name,
    author,
    description,
    genres,
    characters,
    alternative_titles,
    content='media',
    content_rowid='rowid'
);

-- Create indexes for media
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_name ON media(name);
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);

-- FTS triggers
DROP TRIGGER IF EXISTS media_fts_insert;
CREATE TRIGGER media_fts_insert AFTER INSERT ON media
BEGIN
    INSERT INTO media_fts(rowid, slug, name, author, description, genres, characters, alternative_titles)
    VALUES (NEW.rowid, NEW.slug, NEW.name, NEW.author, NEW.description, NEW.genres, NEW.characters, NEW.alternative_titles);
END;

DROP TRIGGER IF EXISTS media_fts_delete;
CREATE TRIGGER media_fts_delete AFTER DELETE ON media
BEGIN
    DELETE FROM media_fts WHERE rowid = OLD.rowid;
END;

DROP TRIGGER IF EXISTS media_fts_update;
CREATE TRIGGER media_fts_update AFTER UPDATE ON media
BEGIN
    UPDATE media_fts SET
        slug = NEW.slug,
        name = NEW.name,
        author = NEW.author,
        description = NEW.description,
        genres = NEW.genres,
        characters = NEW.characters,
        alternative_titles = NEW.alternative_titles
    WHERE rowid = NEW.rowid;
END;

-- Content rating level trigger
DROP TRIGGER IF EXISTS update_media_content_rating_level;
CREATE TRIGGER update_media_content_rating_level AFTER UPDATE OF content_rating ON media
BEGIN
    UPDATE media SET content_rating_level = CASE LOWER(TRIM(NEW.content_rating))
        WHEN 'safe' THEN 0
        WHEN 'suggestive' THEN 1
        WHEN 'erotica' THEN 2
        WHEN 'pornographic' THEN 3
        ELSE 3 END
    WHERE slug = NEW.slug;
END;

-- Populate content_rating_level for existing data
UPDATE media SET content_rating_level = CASE LOWER(TRIM(content_rating))
    WHEN 'safe' THEN 0
    WHEN 'suggestive' THEN 1
    WHEN 'erotica' THEN 2
    WHEN 'pornographic' THEN 3
    ELSE 3 END
WHERE content_rating_level IS NULL OR content_rating_level NOT IN (0,1,2,3);
