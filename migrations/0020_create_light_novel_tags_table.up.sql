-- Create table to store tags associated with light novels
CREATE TABLE IF NOT EXISTS light_novel_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    light_novel_slug TEXT NOT NULL,
    tag TEXT NOT NULL,
    UNIQUE(light_novel_slug, tag),
    FOREIGN KEY (light_novel_slug) REFERENCES light_novels(slug) ON DELETE CASCADE
);