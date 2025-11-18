# Linux Installation Guide

This guide covers installing Magi as a native binary on Linux with systemd service management.

## Prerequisites

- Linux distribution with systemd (Ubuntu, Debian, Fedora, CentOS, etc.)
- 512MB+ available RAM
- Storage for manga collection and database

## Installation Steps

### 1. Download Magi

Download the appropriate binary for your architecture from the [releases page](https://github.com/alexander-bruun/magi/releases).

**For x86_64 (amd64):**
```bash
wget https://github.com/alexander-bruun/magi/releases/latest/download/magi-linux-amd64
chmod +x magi-linux-amd64
sudo mv magi-linux-amd64 /usr/local/bin/magi
```

**For ARM64:**
```bash
wget https://github.com/alexander-bruun/magi/releases/latest/download/magi-linux-arm64
chmod +x magi-linux-arm64
sudo mv magi-linux-arm64 /usr/local/bin/magi
```

### 2. Create a Dedicated User (Optional but Recommended)

Running Magi as a dedicated user improves security:

```bash
sudo useradd --system --no-create-home --shell /bin/false magi
```

### 3. Create Data Directory

```bash
sudo mkdir -p /var/lib/magi
sudo chown magi:magi /var/lib/magi
```

### 4. Create Systemd Service File

Create `/etc/systemd/system/magi.service`:

```bash
sudo nano /etc/systemd/system/magi.service
```

**Basic service configuration:**

```ini
[Unit]
Description=Magi Manga Server
After=network.target

[Service]
Type=simple
User=magi
Group=magi
WorkingDirectory=/var/lib/magi
Environment="MAGI_DATA_DIR=/var/lib/magi"
Environment="PORT=3000"
ExecStart=/usr/local/bin/magi
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/magi
ReadOnlyPaths=/path/to/manga

[Install]
WantedBy=multi-user.target
```

**Advanced service configuration with better security:**

```ini
[Unit]
Description=Magi Manga Server
Documentation=https://alexander-bruun.github.io/magi/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=magi
Group=magi
WorkingDirectory=/var/lib/magi

# Environment variables
Environment="MAGI_DATA_DIR=/var/lib/magi"
Environment="PORT=3000"

# Main process
ExecStart=/usr/local/bin/magi
ExecReload=/bin/kill -HUP $MAINPID

# Restart policy
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=magi

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/magi
ReadOnlyPaths=/path/to/manga

# Additional sandboxing
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
PrivateDevices=true

# Limit resources
LimitNOFILE=65536
MemoryLimit=2G

[Install]
WantedBy=multi-user.target
```

> [!IMPORTANT]
> Replace `/path/to/manga` with the actual path to your manga collection.

### 5. Enable and Start Service

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable magi

# Start the service
sudo systemctl start magi

# Check status
sudo systemctl status magi
```

### 6. Verify Installation

Check that Magi is running:

```bash
# View service status
sudo systemctl status magi

# View logs
sudo journalctl -u magi -f

# Test HTTP endpoint
curl http://localhost:3000
```

Open `http://localhost:3000` in your browser to access Magi.

## Configuration

### Environment Variables

Edit the service file to customize Magi's behavior:

```bash
sudo systemctl edit --full magi
```

Add or modify environment variables:

```ini
Environment="MAGI_DATA_DIR=/var/lib/magi"
Environment="PORT=3000"
Environment="TZ=America/New_York"
```

After editing, reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart magi
```

### File Permissions

Ensure Magi can read your manga collection:

```bash
# Grant read access to manga directory
sudo chmod -R 755 /path/to/manga

# Allow magi user to read manga files
sudo setfacl -R -m u:magi:rx /path/to/manga
```

## Managing the Service

### Common Commands

```bash
# Start Magi
sudo systemctl start magi

# Stop Magi
sudo systemctl stop magi

# Restart Magi
sudo systemctl restart magi

# View status
sudo systemctl status magi

# Enable auto-start on boot
sudo systemctl enable magi

# Disable auto-start
sudo systemctl disable magi

# View logs
sudo journalctl -u magi -f

# View last 100 log lines
sudo journalctl -u magi -n 100

# View logs since yesterday
sudo journalctl -u magi --since yesterday
```

## Updating Magi

To update to a new version:

```bash
# Stop the service
sudo systemctl stop magi

# Download new version
wget https://github.com/alexander-bruun/magi/releases/latest/download/magi-linux-amd64
chmod +x magi-linux-amd64

# Replace old binary
sudo mv magi-linux-amd64 /usr/local/bin/magi

# Start the service
sudo systemctl start magi

# Verify it's running
sudo systemctl status magi
```

## Reverse Proxy Setup

### Nginx

If you want to expose Magi behind Nginx:

```nginx
server {
    listen 80;
    server_name magi.example.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Apache

For Apache with mod_proxy:

```apache
<VirtualHost *:80>
    ServerName magi.example.com

    ProxyPreserveHost On
    ProxyPass / http://localhost:3000/
    ProxyPassReverse / http://localhost:3000/

    <Location />
        Require all granted
    </Location>
</VirtualHost>
```

### Caddy

Caddy provides automatic HTTPS:

```caddy
magi.example.com {
    reverse_proxy localhost:3000
}
```

## Troubleshooting

### Service Won't Start

**Check logs:**
```bash
sudo journalctl -u magi -n 50 --no-pager
```

**Common issues:**
- Port already in use → Change PORT in service file
- Permission denied → Check file permissions and user
- Binary not found → Verify path in ExecStart

### Permission Errors

If Magi can't access manga files:

```bash
# Check current permissions
ls -la /path/to/manga

# Fix ownership (if files owned by another user)
sudo chown -R magi:magi /var/lib/magi

# Grant read access using ACLs
sudo setfacl -R -m u:magi:rx /path/to/manga
```

### Database Issues

**Reset database (WARNING: Deletes all data):**
```bash
sudo systemctl stop magi
sudo rm -rf /var/lib/magi/magi.db
sudo systemctl start magi
```

### High Memory Usage

Limit memory in service file:

```ini
[Service]
MemoryMax=1G
MemoryHigh=800M
```

Then reload:
```bash
sudo systemctl daemon-reload
sudo systemctl restart magi
```

## Uninstalling

To completely remove Magi:

```bash
# Stop and disable service
sudo systemctl stop magi
sudo systemctl disable magi

# Remove service file
sudo rm /etc/systemd/system/magi.service

# Remove binary
sudo rm /usr/local/bin/magi

# Remove data (optional - this deletes your database)
sudo rm -rf /var/lib/magi

# Remove user (optional)
sudo userdel magi

# Reload systemd
sudo systemctl daemon-reload
```

## Next Steps

- [Create your first library](../usage/getting_started.md)
- [Configure automatic indexing](../usage/configuration.md)
- [Set up user accounts](../usage/configuration.md#user-management)
