-- Create the daily_statistics table to track daily changes in statistics
CREATE TABLE IF NOT EXISTS daily_statistics (
    date DATE PRIMARY KEY,
    total_mangas INTEGER NOT NULL DEFAULT 0,
    total_chapters INTEGER NOT NULL DEFAULT 0,
    total_chapters_read INTEGER NOT NULL DEFAULT 0,
    total_light_novels INTEGER NOT NULL DEFAULT 0,
    total_light_novel_chapters INTEGER NOT NULL DEFAULT 0,
    total_light_novel_chapters_read INTEGER NOT NULL DEFAULT 0
);