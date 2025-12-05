CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_username TEXT NOT NULL,
    target_type TEXT NOT NULL CHECK(target_type IN ('media', 'chapter')),
    target_slug TEXT NOT NULL,
    media_slug TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(user_username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(media_slug) REFERENCES media(slug) ON DELETE CASCADE
);

CREATE INDEX idx_comments_target ON comments(target_type, target_slug);
CREATE INDEX idx_comments_user ON comments(user_username);
CREATE INDEX idx_comments_media_slug ON comments(media_slug);