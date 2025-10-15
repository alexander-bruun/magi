-- Migration 0011 down: Remove manga_type column
-- SQLite doesn't support DROP COLUMN; create new table without the column and copy data.
BEGIN TRANSACTION;
CREATE TABLE IF NOT EXISTS mangas_new (
    slug TEXT PRIMARY KEY,
    name TEXT,
    author TEXT,
    description TEXT,
    year INTEGER,
    original_language TEXT,
    status TEXT,
    content_rating TEXT,
    library_slug TEXT,
    cover_art_url TEXT,
    path TEXT,
    file_count INTEGER,
    created_at INTEGER,
    updated_at INTEGER
);
INSERT INTO mangas_new (slug, name, author, description, year, original_language, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at)
SELECT slug, name, author, description, year, original_language, status, content_rating, library_slug, cover_art_url, path, file_count, created_at, updated_at FROM mangas;
DROP TABLE mangas;
ALTER TABLE mangas_new RENAME TO mangas;
COMMIT;
