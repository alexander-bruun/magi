# Configuration Guide

This guide covers all configuration options available in Magi, from basic settings to advanced customization.

## Admin Configuration Page

Access the configuration page via **Admin > Configuration** (admin role required).

### Global Settings

#### Allow New User Registrations

Controls whether new users can create accounts.

- **Enabled**: Registration page is accessible, new users can sign up
- **Disabled**: Registration page shows "Registration is disabled" message

**Use cases:**
- Disable after creating accounts for family/friends
- Enable temporarily when adding new users
- Keep disabled for single-user installations

**To change:**

1. Navigate to **Admin > Configuration**
2. Toggle "Allow new user registrations"
3. Click **Save**

#### Maximum Users

Sets a hard limit on total user accounts.

- **0**: Unlimited users (default)
- **>0**: Blocks registration once limit is reached

**Use cases:**
- Limit server resources for personal use
- Control access to your manga collection
- Prevent unauthorized account creation

**Example:**
```
Max Users: 5
```

After 5 accounts are created, registration attempts will be rejected even if registration is enabled.

### First User Privileges

The first user to register on a fresh Magi installation automatically receives **admin** role. This ensures you have full control from the start.

## Environment Variables

Configure Magi's behavior with environment variables set before starting the application.

### Available Variables

#### MAGI_DATA_DIR

**Description**: Directory for database and cached data  
**Default**: OS-specific:
- Linux: `~/.local/share/magi`
- Windows: `%APPDATA%\magi`
- macOS: `~/Library/Application Support/magi`

**Example:**
```bash
export MAGI_DATA_DIR=/var/lib/magi
```

#### PORT

**Description**: HTTP server port  
**Default**: `3000`

**Example:**
```bash
export PORT=8080
```

#### TZ

**Description**: Timezone for cron jobs and logs  
**Default**: System timezone

**Example:**
```bash
export TZ=America/New_York
```

**Common timezones:**
- `America/New_York` - Eastern Time
- `America/Los_Angeles` - Pacific Time
- `Europe/London` - British Time
- `Asia/Tokyo` - Japan Standard Time
- `Australia/Sydney` - Australian Eastern Time

Find your timezone: [Wikipedia List](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)

### Setting Environment Variables

#### Docker

```bash
docker run -d \
  -e MAGI_DATA_DIR=/data/magi \
  -e PORT=3000 \
  -e TZ=America/New_York \
  alexbruun/magi:latest
```

#### Docker Compose

```yaml
services:
  magi:
    image: alexbruun/magi:latest
    environment:
      - MAGI_DATA_DIR=/data/magi
      - PORT=3000
      - TZ=America/New_York
```

#### Linux Systemd

Edit `/etc/systemd/system/magi.service`:

```ini
[Service]
Environment="MAGI_DATA_DIR=/var/lib/magi"
Environment="PORT=3000"
Environment="TZ=America/New_York"
```

#### Windows (PowerShell)

```powershell
$env:MAGI_DATA_DIR = "C:\MagiData"
$env:PORT = "3000"
.\magi.exe
```

#### Windows Service (NSSM)

```powershell
.\nssm.exe set Magi AppEnvironmentExtra MAGI_DATA_DIR=C:\MagiData PORT=3000
```

## Library Configuration

Manage libraries from **Admin > Libraries**.

### Creating a Library

1. Click **New Library**
2. Fill in details:
   - **Name**: Display name
   - **Description**: Optional notes
   - **Folders**: Paths to scan (one per line)
   - **Cron Schedule**: Auto-scan frequency
3. Click **Save**

### Library Settings

#### Name

Display name shown in the UI.

**Examples:**
- "Main Collection"
- "Manga Library"
- "Downloads"

#### Description

Optional field for notes about the library.

**Examples:**
- "Completed series only"
- "Auto-imported from downloads folder"
- "Shared family collection"

#### Folders

List of absolute paths to scan for manga, one per line.

**Linux/macOS:**
```
/home/user/manga
/mnt/nas/manga
/media/external/manga
```

**Windows:**
```
C:\Manga
D:\Downloads\Manga
\\NAS\Media\Manga
```

**Docker:**
```
/data/manga
/data/manga/completed
/data/manga/ongoing
```

> [!TIP]
> Magi recursively scans subdirectories, so you can point to a top-level folder.

#### Cron Schedule

Determines when the library is automatically re-indexed.

**Format**: `minute hour day month weekday`

**Common schedules:**

| Schedule | Description |
|----------|-------------|
| `0 2 * * *` | Daily at 2 AM |
| `0 */6 * * *` | Every 6 hours |
| `0 0 * * 0` | Weekly (Sunday midnight) |
| `0 3 * * 1` | Weekly (Monday 3 AM) |
| `@hourly` | Every hour |
| `@daily` | Daily at midnight |
| `@weekly` | Weekly (Sunday midnight) |

**Examples:**

- **Light users**: `0 3 * * *` (daily at 3 AM)
- **Heavy users**: `0 */3 * * *` (every 3 hours)
- **Weekly updates**: `0 2 * * 0` (Sunday 2 AM)

**Disable auto-scan**: Leave blank or use a very infrequent schedule.

### Managing Libraries

#### Editing

1. Click **Edit** next to the library
2. Modify settings
3. Click **Save**

#### Manual Indexing

Click **Index Now** to immediately scan the library, bypassing the schedule.

**Use when:**
- Adding new manga
- Reorganizing files
- Testing configuration changes

#### Deleting

1. Click **Delete** next to the library
2. Confirm deletion

> [!WARNING]
> Deleting a library removes all associated manga and chapters from the database. This does NOT delete your manga files.

## User Management

Manage users from **Admin > Users** (admin role required).

### User Roles

#### Reader

**Permissions:**
- Browse and search manga
- Read chapters
- Track reading progress
- Add favorites
- Vote on manga

**Cannot:**
- Edit metadata
- Manage libraries
- Create/manage users

#### Moderator

**Permissions:**
- All Reader permissions
- Update manga metadata
- Refresh manga/chapters
- Manual metadata editing
- Create/edit scrapers

**Cannot:**
- Manage libraries
- Manage users
- Access configuration

#### Admin

**Permissions:**
- All Moderator permissions
- Create/edit/delete libraries
- Manage users (create, ban, change roles)
- Access configuration page
- View all admin features

### Changing User Roles

1. Navigate to **Admin > Users**
2. Find the user
3. Click **Change Role**
4. Select new role
5. Confirm

### Banning Users

Banned users cannot log in or access any content.

1. Navigate to **Admin > Users**
2. Find the user
3. Click **Ban**
4. Confirm

**To unban:**
1. Click **Unban** next to the user
2. Confirm

### Creating Users (Admin)

While users can self-register (if enabled), admins can manually create accounts:

1. Enable registration temporarily
2. Share the registration link
3. Let user create account
4. Disable registration
5. Assign appropriate role

> [!NOTE]
> Admins cannot directly create user accounts through the UI yet. Users must register themselves.

## Content Rating Filter

Magi respects MangaDex content ratings and can filter content.

### Content Ratings

| Rating | Description |
|--------|-------------|
| **Safe** | All-ages content |
| **Suggestive** | Mild fanservice |
| **Erotica** | Sexual content (not explicit) |
| **Pornographic** | Explicit sexual content |

### Filtering

Currently, content rating filtering is handled at the database level. Future updates may add user-level filtering.

## Scraper Configuration

Magi includes a web-based scraper system for downloading manga from online sources.

### Accessing Scrapers

Navigate to **Admin > Scrapers** (moderator/admin only).

### Creating a Scraper

1. Click **New Script**
2. Enter script details:
   - **Name**: Descriptive name
   - **URL**: Target website
   - **Variables**: Custom parameters
3. Write JavaScript code in the editor
4. Click **Save**

### Running Scrapers

1. Select a scraper from the list
2. Configure variables (if any)
3. Click **Run**
4. Monitor progress in **Job Status**

### Scraper Variables

Variables allow dynamic scraper behavior:

**Example:**
```
manga_id = 12345
chapter_start = 1
chapter_end = 100
```

Access in JavaScript:
```javascript
const mangaId = variables.manga_id;
```

> [!WARNING]
> Scrapers are powerful but can violate website terms of service. Use responsibly and respect copyright laws.

## Database Maintenance

### Location

The SQLite database is stored in `MAGI_DATA_DIR`:

```
$MAGI_DATA_DIR/magi.db
```

### Backup

**Manual backup:**

```bash
# Stop Magi
# Copy database file
cp ~/.local/share/magi/magi.db ~/backups/magi-$(date +%Y%m%d).db
# Restart Magi
```

**Automated backup (Linux):**

```bash
# Add to crontab
0 3 * * * cp ~/.local/share/magi/magi.db ~/backups/magi-$(date +\%Y\%m\%d).db
```

### Restore

```bash
# Stop Magi
# Replace database
cp ~/backups/magi-20240101.db ~/.local/share/magi/magi.db
# Restart Magi
```

### Reset Database

> [!DANGER]
> This deletes ALL data: users, libraries, manga, chapters, progress.

```bash
# Stop Magi
rm ~/.local/share/magi/magi.db
# Restart Magi (creates fresh database)
```

## Performance Tuning

### Indexing Performance

**Factors affecting speed:**
- Collection size
- Internet speed (for metadata)
- Disk I/O (especially for cover art)
- CPU (for archive extraction)

**Tips:**
- Use SSD storage for faster indexing
- Limit concurrent libraries indexing
- Schedule indexing during off-peak hours
- Use CBZ instead of RAR

### Memory Usage

Magi is memory-efficient, but large collections may need tuning.

**Docker memory limit:**
```bash
docker run -d --memory=2g alexbruun/magi:latest
```

**Systemd memory limit:**
```ini
[Service]
MemoryMax=2G
```

### Cache Management

Magi caches:
- Cover images
- Chapter thumbnails
- Extracted pages (temporary)

**Location**: `$MAGI_DATA_DIR/cache/`

**Clear cache:**
```bash
rm -rf ~/.local/share/magi/cache/*
```

## Security Considerations

### Network Access

By default, Magi listens on all interfaces (`0.0.0.0:3000`).

**Restrict to localhost:**
Not currently configurable. Use reverse proxy or firewall rules.

### HTTPS

Magi doesn't include built-in HTTPS. Use a reverse proxy:

- **Nginx**: [Guide](https://docs.nginx.com/nginx/admin-guide/security-controls/terminating-ssl-http/)
- **Caddy**: Automatic HTTPS with Let's Encrypt
- **Traefik**: Container-native reverse proxy

### Authentication

Magi uses JWT tokens for authentication:

- Access tokens expire after 15 minutes
- Refresh tokens stored in HTTP-only cookies
- Tokens invalidated on password change

### Password Security

- Passwords hashed with bcrypt
- No password requirements (set your own)
- Change password: **Account > Settings** (not yet implemented)

## Advanced Configuration

### Multiple Instances

Run multiple Magi instances with different data directories:

**Instance 1:**
```bash
MAGI_DATA_DIR=/data/magi1 PORT=3000 ./magi
```

**Instance 2:**
```bash
MAGI_DATA_DIR=/data/magi2 PORT=3001 ./magi
```

### Custom Data Locations

Structure your data directory:

```
/var/lib/magi/
├── magi.db          # Database
├── cache/           # Cached images
│   ├── covers/
│   └── pages/
└── logs/            # Application logs (if enabled)
```

### Reverse Proxy Examples

#### Nginx

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
        proxy_cache_bypass $http_upgrade;
    }
}
```

#### Caddy

```caddy
magi.example.com {
    reverse_proxy localhost:3000
}
```

#### Traefik (Docker labels)

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.magi.rule=Host(`magi.example.com`)"
  - "traefik.http.services.magi.loadbalancer.server.port=3000"
```

## Troubleshooting Configuration

### Changes Not Applied

1. Verify settings saved correctly
2. Restart Magi service/container
3. Check environment variables are set
4. Clear browser cache

### Port Conflicts

If port 3000 is in use:

```bash
# Find process using port
sudo lsof -i :3000

# Use different port
PORT=8080 ./magi
```

### Permission Issues

Ensure Magi can:
- Read manga directories
- Write to data directory
- Bind to configured port

```bash
# Check permissions
ls -la /path/to/manga
ls -la $MAGI_DATA_DIR
```

## Next Steps

- [Troubleshoot common issues](troubleshooting.md)
- [Learn about web-based scrapers](#) (coming soon)
- [Explore advanced features](#) (coming soon)

