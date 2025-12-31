-- Migration 0028: Remove parallel indexing configuration settings
-- +migrate Down
ALTER TABLE app_config DROP COLUMN parallel_indexing_threshold;
ALTER TABLE app_config DROP COLUMN parallel_indexing_enabled;