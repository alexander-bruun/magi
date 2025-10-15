-- Migration 0011: Add manga_type column to mangas
ALTER TABLE mangas ADD COLUMN manga_type TEXT DEFAULT 'manga';
