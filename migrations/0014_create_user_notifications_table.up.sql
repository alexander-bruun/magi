CREATE TABLE IF NOT EXISTS user_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT NOT NULL,
    media_slug TEXT NOT NULL,
    library_slug TEXT NOT NULL,
    chapter_slug TEXT NOT NULL,
    message TEXT NOT NULL,
    type TEXT DEFAULT 'chapter',
    is_read INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (user_name) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY (media_slug) REFERENCES media(slug) ON DELETE CASCADE,
    FOREIGN KEY (media_slug, library_slug, chapter_slug) REFERENCES chapters(media_slug, library_slug, slug) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_user ON user_notifications(user_name, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_notifications_media_chapter ON user_notifications(media_slug, library_slug, chapter_slug);
