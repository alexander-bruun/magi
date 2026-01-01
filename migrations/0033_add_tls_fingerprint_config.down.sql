-- Remove TLS fingerprint config from app_config
ALTER TABLE app_config DROP COLUMN tls_fingerprint_enabled;
ALTER TABLE app_config DROP COLUMN tls_fingerprint_strict;
