-- Create the collection_media table
CREATE TABLE IF NOT EXISTS collection_media (
    collection_id INTEGER NOT NULL,
    media_slug TEXT NOT NULL,
    added_at INTEGER NOT NULL,
    PRIMARY KEY (collection_id, media_slug),
    FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE,
    FOREIGN KEY (media_slug) REFERENCES media(slug) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_collection_media_collection_id ON collection_media(collection_id);
CREATE INDEX IF NOT EXISTS idx_collection_media_media_slug ON collection_media(media_slug);