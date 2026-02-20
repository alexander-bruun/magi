-- Create banned_ips table to store banned IP addresses
DROP TABLE IF EXISTS banned_ips;
CREATE TABLE IF NOT EXISTS banned_ips (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_address TEXT NOT NULL UNIQUE,
    banned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT,
    expires_at DATETIME
);