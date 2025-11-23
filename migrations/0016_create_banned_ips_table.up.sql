-- Create banned_ips table to store banned IP addresses
CREATE TABLE banned_ips (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_address TEXT NOT NULL UNIQUE,
    banned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT
);