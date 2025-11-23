-- Create the light_novel_votes table
CREATE TABLE IF NOT EXISTS light_novel_votes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_username TEXT NOT NULL,
    light_novel_slug TEXT NOT NULL,
    value INTEGER NOT NULL CHECK(value IN (1, -1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_username, light_novel_slug),
    FOREIGN KEY(user_username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(light_novel_slug) REFERENCES light_novels(slug) ON DELETE CASCADE
);