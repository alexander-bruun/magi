-- Create table to store tags associated with medias
CREATE TABLE IF NOT EXISTS media_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_slug TEXT NOT NULL,
    tag TEXT NOT NULL,
    UNIQUE(media_slug, tag),
    FOREIGN KEY (media_slug) REFERENCES media(slug) ON DELETE CASCADE
);
