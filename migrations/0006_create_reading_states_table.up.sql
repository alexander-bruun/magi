-- Create the reading_states table to track per-user chapter reads
CREATE TABLE IF NOT EXISTS reading_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT NOT NULL,
    manga_slug TEXT NOT NULL,
    chapter_slug TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_name, manga_slug, chapter_slug),
    FOREIGN KEY(user_name) REFERENCES users(username) ON DELETE CASCADE
);
