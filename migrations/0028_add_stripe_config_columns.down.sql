-- +migrate Down
ALTER TABLE app_config DROP COLUMN stripe_webhook_secret;
ALTER TABLE app_config DROP COLUMN stripe_secret_key;
ALTER TABLE app_config DROP COLUMN stripe_publishable_key;
ALTER TABLE app_config DROP COLUMN stripe_enabled;