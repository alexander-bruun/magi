-- Create table to store tags associated with mangas
CREATE TABLE IF NOT EXISTS manga_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    manga_slug TEXT NOT NULL,
    tag TEXT NOT NULL,
    UNIQUE(manga_slug, tag),
    FOREIGN KEY (manga_slug) REFERENCES mangas(slug) ON DELETE CASCADE
);
