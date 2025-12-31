-- Migration 0028: Add parallel indexing configuration settings
-- +migrate Up
ALTER TABLE app_config ADD COLUMN parallel_indexing_enabled INTEGER NOT NULL DEFAULT 1; -- 1 = enabled, 0 = disabled
ALTER TABLE app_config ADD COLUMN parallel_indexing_threshold INTEGER NOT NULL DEFAULT 100; -- minimum series count to trigger parallel processing

-- +migrate Down
ALTER TABLE app_config DROP COLUMN parallel_indexing_threshold;
ALTER TABLE app_config DROP COLUMN parallel_indexing_enabled;