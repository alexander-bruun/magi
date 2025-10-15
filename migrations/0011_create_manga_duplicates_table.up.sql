-- Create manga_duplicates table
CREATE TABLE IF NOT EXISTS manga_duplicates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    manga_slug TEXT NOT NULL,
    library_slug TEXT NOT NULL,
    folder_path_1 TEXT NOT NULL,
    folder_path_2 TEXT NOT NULL,
    dismissed INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    UNIQUE(manga_slug, folder_path_1, folder_path_2),
    FOREIGN KEY (manga_slug) REFERENCES mangas(slug) ON DELETE CASCADE,
    FOREIGN KEY (library_slug) REFERENCES libraries(slug) ON DELETE CASCADE
);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_manga_duplicates_manga_slug ON manga_duplicates(manga_slug);
CREATE INDEX IF NOT EXISTS idx_manga_duplicates_library_slug ON manga_duplicates(library_slug);
CREATE INDEX IF NOT EXISTS idx_manga_duplicates_dismissed ON manga_duplicates(dismissed);
