-- Migration 0019: Remove image_access_secret from app_config table
ALTER TABLE app_config DROP COLUMN image_access_secret;