CREATE TABLE IF NOT EXISTS votes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_username TEXT NOT NULL,
    media_slug TEXT NOT NULL,
    value INTEGER NOT NULL CHECK(value IN (1, -1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_username, media_slug),
    FOREIGN KEY(user_username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(media_slug) REFERENCES media(slug) ON DELETE CASCADE
);

-- Triggers to maintain vote_score in media
DROP TRIGGER IF EXISTS update_media_vote_score;
CREATE TRIGGER update_media_vote_score AFTER INSERT ON votes
BEGIN
    UPDATE media SET vote_score = (
        SELECT COALESCE(SUM(value), 0) FROM votes WHERE media_slug = NEW.media_slug
    ) WHERE slug = NEW.media_slug;
END;

DROP TRIGGER IF EXISTS update_media_vote_score_delete;
CREATE TRIGGER update_media_vote_score_delete AFTER DELETE ON votes
BEGIN
    UPDATE media SET vote_score = (
        SELECT COALESCE(SUM(value), 0) FROM votes WHERE media_slug = OLD.media_slug
    ) WHERE slug = OLD.media_slug;
END;
