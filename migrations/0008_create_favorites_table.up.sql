CREATE TABLE IF NOT EXISTS favorites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_username TEXT NOT NULL,
    manga_slug TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_username, manga_slug),
    FOREIGN KEY(user_username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(manga_slug) REFERENCES mangas(slug) ON DELETE CASCADE
);
