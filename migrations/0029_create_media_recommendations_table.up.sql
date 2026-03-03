-- Create table to store precomputed recommendations for each media
CREATE TABLE IF NOT EXISTS media_recommendations (
    media_slug TEXT NOT NULL,
    recommended_slug TEXT NOT NULL,
    score INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (media_slug, recommended_slug),
    FOREIGN KEY (media_slug) REFERENCES media(slug) ON DELETE CASCADE,
    FOREIGN KEY (recommended_slug) REFERENCES media(slug) ON DELETE CASCADE
);
-- Index for fast lookup
CREATE INDEX IF NOT EXISTS idx_media_recommendations_media_slug ON media_recommendations(media_slug);
