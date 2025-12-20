-- Create the highlights table
CREATE TABLE highlights (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_slug TEXT NOT NULL UNIQUE,
    background_image_url TEXT,
    description TEXT,
    display_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_slug) REFERENCES media(slug) ON DELETE CASCADE
);