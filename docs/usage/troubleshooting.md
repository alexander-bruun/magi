# Troubleshooting Guide

This guide covers common issues and their solutions when running Magi.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Database Problems](#database-problems)
- [Indexing Issues](#indexing-issues)
- [Reading and Display Issues](#reading-and-display-issues)
- [Performance Problems](#performance-problems)
- [Network and Access Issues](#network-and-access-issues)
- [User and Authentication Issues](#user-and-authentication-issues)
- [Advanced Debugging](#advanced-debugging)

## Installation Issues

### Binary Won't Run

**Symptoms**: `Permission denied` or `command not found`

**Solution**:
```bash
# Make binary executable
chmod +x magi

# Verify it's executable
ls -la magi

# Run with explicit path
./magi
```

### Port Already in Use

**Symptoms**: `Error: listen tcp :3000: bind: address already in use`

**Solution**:

**Find what's using the port:**
```bash
# Linux/macOS
sudo lsof -i :3000
sudo ss -tulpn | grep 3000

# Windows (PowerShell)
netstat -ano | findstr :3000
```

**Change Magi's port:**
```bash
PORT=8080 ./magi
```

### Missing Dependencies

**Symptoms**: `error while loading shared libraries`

**Solution**:

**Linux:**
```bash
# Install common dependencies
sudo apt install libsqlite3-0  # Debian/Ubuntu
sudo dnf install sqlite-libs   # Fedora/RHEL
```

**Windows:**
Install [Microsoft Visual C++ Redistributable](https://aka.ms/vs/17/release/vc_redist.x64.exe)

## Database Problems

### Database Locked

**Symptoms**: `database is locked` errors in logs

**Cause**: Multiple Magi instances accessing the same database

**Solution**:
1. Stop all Magi instances
2. Ensure only one instance runs per database
3. Check for orphaned processes: `ps aux | grep magi`

### Corrupted Database

**Symptoms**: `database disk image is malformed`, crashes on startup

**Solution**:

**Check integrity:**
```bash
sqlite3 ~/magi/magi.db "PRAGMA integrity_check;"
```

**Attempt repair:**
```bash
sqlite3 ~/magi/magi.db ".recover" | sqlite3 ~/magi/magi_recovered.db
```

**Restore from backup:**
```bash
# Stop Magi
cp ~/backups/magi.db ~/.local/share/magi/magi.db
# Start Magi
```

**Last resort - Reset database** (⚠️ Deletes all data):
```bash
rm ~/.local/share/magi/magi.db
```

### Database Inspection

**Access database:**
```bash
# Install SQLite tools
sudo apt install sqlite3  # Debian/Ubuntu
sudo dnf install sqlite   # Fedora/RHEL

# Open database
sqlite3 ~/magi/magi.db
```

**Common queries:**
```sql
-- View tables
.tables

-- View schema
.schema

-- Count manga
SELECT COUNT(*) FROM mangas;

-- Count users
SELECT COUNT(*) FROM users;

-- View all users
SELECT username, role, banned FROM users;

-- Check configuration
SELECT * FROM app_config;

-- Exit
.quit
```

## Indexing Issues

### Library Won't Index

**Symptoms**: Library shows 0 manga, indexing completes but nothing added

**Causes and solutions:**

#### 1. Incorrect Path

```bash
# Verify path exists
ls -la /path/to/manga

# Check Docker volume mounts
docker inspect magi | grep Mounts -A 20
```

#### 2. Permission Denied

```bash
# Check file permissions
ls -la /path/to/manga

# Fix permissions (Linux)
chmod -R 755 /path/to/manga
chown -R magi:magi /path/to/manga

# Docker: ensure volume is readable
docker run --rm -v /path/to/manga:/data:ro busybox ls -la /data
```

#### 3. No Supported Files

Magi requires:
- CBZ, CBR, ZIP, or RAR files
- Or directories containing images

**Verify structure:**
```
/manga/
├── Series Name/          # Folder with manga name
│   ├── Chapter 1.cbz    # Chapter files
│   └── Chapter 2.cbz
```

#### 4. Invalid File Names

**Problematic names:**
- Non-UTF-8 characters
- Very long filenames
- Special characters: `<>:"|?*`

**Fix:**
```bash
# Rename files with problematic characters
find /path/to/manga -name '*[<>:"|?*]*' -print
```

### Manga Not Found on MangaDex

**Symptoms**: Manga indexed but no metadata or cover art

**Solutions:**

1. **Manual metadata update:**
   - Open manga page
   - Click **Update Metadata** (moderator/admin)
   - Search for correct manga
   - Apply metadata

2. **Manual edit:**
   - Click **Manual Edit**
   - Enter title, author, description manually
   - Save

3. **Offline mode:**
   Magi uses folder names if no metadata found. This is normal for:
   - Unlicensed manga
   - Very new releases
   - Non-English manga

### Slow Indexing

**Symptoms**: Indexing takes hours for moderate collections

**Causes and solutions:**

#### 1. Slow Internet

Metadata fetching requires internet. Each manga queries MangaDex API.

**Solution:** Be patient or index during off-peak hours.

#### 2. Slow Disk I/O

Cover art downloads and archive extraction are disk-intensive.

**Solution:** 
- Use SSD storage
- Reduce concurrent operations
- Index one library at a time

#### 3. Large RAR Archives

RAR files are slow for random access.

**Solution:** Convert to CBZ:
```bash
# Extract and recompress as ZIP
for f in *.cbr; do
    mkdir temp
    unrar x "$f" temp/
    cd temp
    zip -r "../${f%.cbr}.cbz" *
    cd ..
    rm -rf temp
done
```

### Duplicate Detection Issues

**Symptoms**: Same manga indexed multiple times

**Cause**: Multiple folders with similar names, or same files in different libraries

**Solution:**

1. Check **Admin > Duplicates** (if available)
2. Manually delete unwanted duplicates:
   - Open duplicate manga
   - Click **Delete** (admin only)
3. Reorganize folders to avoid duplicates

## Reading and Display Issues

### Images Not Loading

**Symptoms**: Blank pages, broken images in reader

**Causes and solutions:**

#### 1. Archive Extraction Failed

**Check logs:**
```bash
# Docker
docker logs magi

# Systemd
journalctl -u magi -f

# Direct run
# Check terminal output
```

**Solution:** Re-index manga to retry extraction.

#### 2. Cached Data Corruption

**Clear cache:**
```bash
# Stop Magi
rm -rf ~/.local/share/magi/cache/*
# Start Magi
```

#### 3. File Permissions

**Verify Magi can read files:**
```bash
ls -la /path/to/manga/SeriesName/Chapter.cbz

# Test extraction manually
unzip -t /path/to/manga/SeriesName/Chapter.cbz
```

### Reader Controls Not Working

**Symptoms**: Keyboard shortcuts don't work, navigation broken

**Solutions:**

1. **Reload page:** Press `F5` or `Ctrl+R`
2. **Clear browser cache:** `Ctrl+Shift+Delete`
3. **Check JavaScript:** Ensure JavaScript is enabled
4. **Try different browser:** Test in Chrome/Firefox

### Cover Art Missing

**Symptoms**: Manga has no cover image, shows placeholder

**Causes:**

1. **MangaDex API issue:** Try updating metadata
2. **Network error during download:** Re-index manga
3. **Cache cleared:** Will re-download on next access

**Solution:**
```bash
# Force cover re-download
# Option 1: Re-index manga
Click "Refresh Metadata" on manga page

# Option 2: Delete cached cover
rm ~/.local/share/magi/cache/covers/manga-slug.jpg
```

### Wrong Chapter Order

**Symptoms**: Chapters listed in incorrect order

**Cause**: Chapter naming doesn't sort naturally

**Examples of problematic naming:**
- `Chapter 1`, `Chapter 10`, `Chapter 2` (string sort)
- Mixed formats: `Ch 1`, `Chapter 2`, `Ep 3`

**Solution:**

Rename chapters with zero-padded numbers:
```bash
# Example: Rename to 001, 002, etc.
for i in {1..99}; do
    mv "Chapter $i.cbz" "Chapter $(printf %03d $i).cbz"
done
```

## Performance Problems

### High Memory Usage

**Symptoms**: Magi using 2GB+ RAM, system slowdown

**Solutions:**

**Docker - Limit memory:**
```bash
docker run -d --memory=1g --memory-swap=2g alexbruun/magi:latest
```

**Systemd - Limit memory:**
```ini
[Service]
MemoryMax=1G
MemoryHigh=800M
```

**Reduce collection size:**
- Split into multiple libraries
- Index fewer manga at once

### Slow Page Loading

**Symptoms**: Pages take seconds to load in reader

**Causes and solutions:**

#### 1. Large Image Files

Some manga have huge images (10MB+ per page).

**No direct solution**, but Magi should cache after first load.

#### 2. Slow Storage

**Solution:** Move manga to faster storage (SSD).

#### 3. Network Latency

If accessing Magi over network, latency affects load times.

**Solution:** Use local network, not internet.

### High CPU Usage

**Symptoms**: 100% CPU during indexing

**This is normal during:**
- Initial indexing
- Cover art download/processing
- Archive extraction

**If persistent after indexing:**
1. Check for indexing loops (bad cron schedule)
2. Verify no corrupted archives causing retries
3. Restart Magi

## Network and Access Issues

### Can't Access from Other Devices

**Symptoms**: `http://[server-ip]:3000` times out from other computers

**Solutions:**

#### 1. Firewall Blocking Port

**Linux (firewalld):**
```bash
sudo firewall-cmd --add-port=3000/tcp --permanent
sudo firewall-cmd --reload
```

**Linux (ufw):**
```bash
sudo ufw allow 3000/tcp
sudo ufw reload
```

**Windows:**
```powershell
New-NetFirewallRule -DisplayName "Magi" -Direction Inbound -Protocol TCP -LocalPort 3000 -Action Allow
```

#### 2. Wrong IP Address

**Find correct IP:**
```bash
# Linux/macOS
ip addr show
ifconfig

# Windows
ipconfig
```

Use the IP from your local network (usually `192.168.x.x` or `10.x.x.x`).

#### 3. Docker Network Mode

If using Docker's `host` network mode, IP should be the host's IP, not Docker's.

### CORS Errors

**Symptoms**: Browser console shows CORS policy errors

**Cause**: Accessing Magi from different domain/port

**Solution:** Use reverse proxy (Nginx, Caddy) with proper CORS headers.

### WebSocket Connection Failed

**Symptoms**: Job status not updating in real-time

**Cause**: Reverse proxy not forwarding WebSocket connections

**Solution (Nginx):**
```nginx
location / {
    proxy_pass http://localhost:3000;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## User and Authentication Issues

### Can't Login

**Symptoms**: "Invalid username or password" despite correct credentials

**Solutions:**

#### 1. Password Typo

Double-check username and password (case-sensitive).

#### 2. Account Banned

Ask admin to check: **Admin > Users**

#### 3. Database Issue

```bash
# Check if user exists
sqlite3 ~/magi/magi.db "SELECT username, banned FROM users WHERE username='yourname';"
```

### Logged Out Repeatedly

**Symptoms**: Session expires too quickly

**Cause:** Tokens expire after 15 minutes (by design)

**Workaround:** Stay active or login again.

### Registration Disabled

**Symptoms**: "Registration is disabled" message

**Cause:** Admin disabled registration in config

**Solution:** Ask admin to enable registration temporarily, or have admin change your role.

### Can't Change Password

**Symptoms**: No password change option in UI

**Current limitation:** Password changes not yet implemented in UI.

**Workaround (admin):**
```bash
# Manually reset password via database
# (Requires bcrypt hash generation - complex)
```

## Advanced Debugging

### Enable Debug Logging

**Linux/macOS:**
```bash
# Not currently configurable
# Check implementation for log level options
```

**Docker:**
```bash
docker logs -f magi
```

**Systemd:**
```bash
journalctl -u magi -f -n 100
```

### Check Application Health

**Test HTTP endpoint:**
```bash
curl -v http://localhost:3000/
```

**Expected:** HTML response with status 200.

### Verify File System

**Check file system for errors:**
```bash
# Linux - Check disk
sudo fsck /dev/sda1

# Check manga directory
ls -laR /path/to/manga | head -100
```

### Network Debugging

**Test connectivity:**
```bash
# From same server
curl http://localhost:3000

# From another device
curl http://[server-ip]:3000

# Test DNS
nslookup magi.example.com
```

### Database Debugging

**Check database size:**
```bash
du -h ~/.local/share/magi/magi.db
```

**Analyze query performance:**
```bash
sqlite3 ~/magi/magi.db
> EXPLAIN QUERY PLAN SELECT * FROM mangas;
```

**Vacuum database:**
```bash
sqlite3 ~/magi/magi.db "VACUUM;"
```

### Container Debugging

**Docker:**
```bash
# Shell into container
docker exec -it magi /bin/sh

# Check environment
docker exec magi env

# Check processes
docker top magi

# Inspect container config
docker inspect magi
```

**Podman:**
```bash
# Similar commands, replace docker with podman
podman exec -it magi /bin/sh
```

## Getting Help

If you're still stuck:

1. **Check existing issues:** [GitHub Issues](https://github.com/alexander-bruun/magi/issues)
2. **Search discussions:** [GitHub Discussions](https://github.com/alexander-bruun/magi/discussions)
3. **Create new issue:** Include:
   - Magi version (`./magi --version`)
   - Operating system
   - Installation method (binary, Docker, etc.)
   - Relevant logs
   - Steps to reproduce
4. **Ask questions:** Post in Discussions for general help

### Collecting Debug Information

When reporting issues, provide:

```bash
# Magi version
./magi --version

# OS info
uname -a

# Docker version (if using Docker)
docker --version

# Recent logs (last 100 lines)
journalctl -u magi -n 100 --no-pager

# Database stats
sqlite3 ~/magi/magi.db "SELECT COUNT(*) FROM mangas; SELECT COUNT(*) FROM users;"

# Disk space
df -h
```

## Common Error Messages

### `database schema version mismatch`

**Solution:** Database needs migration. Restart Magi (migrations run automatically).

### `failed to fetch metadata: connection refused`

**Solution:** 
- Check internet connection
- MangaDex may be down, try again later
- Firewall blocking outbound HTTPS

### `failed to extract archive: unsupported format`

**Solution:** 
- Archive may be corrupted
- Unsupported compression method
- Re-download or convert to standard CBZ

### `permission denied: /data/magi/magi.db`

**Solution:** Fix file permissions (see [Permission Errors](#permission-denied) above).

### `context deadline exceeded`

**Solution:** 
- Request timed out (slow network/API)
- Try again
- Increase timeout (not currently configurable)

## Preventive Maintenance

Avoid issues with regular maintenance:

1. **Backup database weekly:**
   ```bash
   cp ~/.local/share/magi/magi.db ~/backups/
   ```

2. **Monitor disk space:**
   ```bash
   df -h
   ```

3. **Update Magi regularly:**
   Stay on latest version for bug fixes.

4. **Organize files properly:**
   Consistent naming and structure reduces issues.

5. **Test after changes:**
   After configuration changes, verify everything works.

## Known Issues

Track known issues and their status: [GitHub Issues](https://github.com/alexander-bruun/magi/issues)

**Common known issues:**
- RAR archives slower than ZIP (by design)
- Large collections (10,000+ manga) may be slow
- Password change UI not yet implemented

---

**Still having problems?** Open an issue with detailed information, and the community will help!
