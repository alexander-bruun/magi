CREATE TABLE IF NOT EXISTS reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_username TEXT NOT NULL,
    media_slug TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK(rating >= 1 AND rating <= 10),
    content TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_username, media_slug),
    FOREIGN KEY(user_username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(media_slug) REFERENCES media(slug) ON DELETE CASCADE
);

CREATE INDEX idx_reviews_media ON reviews(media_slug);
CREATE INDEX idx_reviews_user ON reviews(user_username);