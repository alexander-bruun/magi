-- Migration 0019: Add image_access_secret to app_config table
ALTER TABLE app_config ADD COLUMN image_access_secret TEXT NOT NULL DEFAULT '';