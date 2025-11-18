CREATE TABLE IF NOT EXISTS user_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT NOT NULL,
    manga_slug TEXT NOT NULL,
    chapter_slug TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (user_name) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY (manga_slug) REFERENCES mangas(slug) ON DELETE CASCADE,
    FOREIGN KEY (manga_slug, chapter_slug) REFERENCES chapters(manga_slug, slug) ON DELETE CASCADE
);

CREATE INDEX idx_user_notifications_user ON user_notifications(user_name, is_read, created_at DESC);
CREATE INDEX idx_user_notifications_manga_chapter ON user_notifications(manga_slug, chapter_slug);
