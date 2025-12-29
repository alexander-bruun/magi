-- +migrate Up
ALTER TABLE app_config ADD COLUMN stripe_enabled BOOLEAN DEFAULT 0;
ALTER TABLE app_config ADD COLUMN stripe_publishable_key TEXT DEFAULT '';
ALTER TABLE app_config ADD COLUMN stripe_secret_key TEXT DEFAULT '';
ALTER TABLE app_config ADD COLUMN stripe_webhook_secret TEXT DEFAULT '';