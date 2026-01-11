-- Migration 0028: Update MAL API token to client ID and secret for OAuth2
ALTER TABLE app_config ADD COLUMN mal_client_id TEXT NOT NULL DEFAULT '';
ALTER TABLE app_config ADD COLUMN mal_client_secret TEXT NOT NULL DEFAULT '';
UPDATE app_config SET mal_client_id = mal_api_token WHERE id = 1;
ALTER TABLE app_config DROP COLUMN mal_api_token;