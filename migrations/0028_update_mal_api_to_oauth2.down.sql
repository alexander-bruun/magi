-- Migration 0028: Revert MAL OAuth2 to single API token
ALTER TABLE app_config ADD COLUMN mal_api_token TEXT NOT NULL DEFAULT '';
UPDATE app_config SET mal_api_token = mal_client_id WHERE id = 1;
ALTER TABLE app_config DROP COLUMN mal_client_id;
ALTER TABLE app_config DROP COLUMN mal_client_secret;