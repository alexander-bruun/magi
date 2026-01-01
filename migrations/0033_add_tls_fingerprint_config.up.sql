-- Add TLS fingerprint config to app_config
ALTER TABLE app_config ADD COLUMN tls_fingerprint_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN tls_fingerprint_strict BOOLEAN DEFAULT FALSE;
