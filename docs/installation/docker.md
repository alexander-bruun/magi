# Docker Installation Guide

Docker is the recommended way to run Magi. It provides isolation, easy updates, and consistent behavior across platforms.

## Prerequisites

- Docker Engine 20.10+ or Docker Desktop
- At least 512MB of available RAM
- Storage for your manga collection and Magi database

## Quick Start

Pull and run Magi with a single command:

```bash
docker run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

Replace `/path/to/manga` with the actual path to your manga collection.

## Docker Compose (Recommended)

For production use, Docker Compose provides easier management and persistence.

### Basic Configuration

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  magi:
    image: alexbruun/magi:latest
    container_name: magi
    ports:
      - "3000:3000"
    volumes:
      # Mount your manga collection (read-only recommended)
      - /path/to/manga:/data/manga:ro
      # Persistent data for database and cache
      - magi-data:/data/magi
    environment:
      - MAGI_DATA_DIR=/data/magi
      - PORT=3000
    restart: unless-stopped

volumes:
  magi-data:
```

### Advanced Configuration

For multiple manga libraries or custom settings:

```yaml
version: '3.8'

services:
  magi:
    image: alexbruun/magi:latest
    container_name: magi
    ports:
      - "3000:3000"
    volumes:
      # Multiple manga collections
      - /path/to/manga/library1:/data/manga/library1:ro
      - /path/to/manga/library2:/data/manga/library2:ro
      - /mnt/nas/manga:/data/manga/nas:ro
      # Persistent application data
      - magi-data:/data/magi
    environment:
      - MAGI_DATA_DIR=/data/magi
      - PORT=3000
      - TZ=America/New_York  # Set your timezone
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  magi-data:
```

### Starting the Container

```bash
# Start in foreground (see logs)
docker compose up

# Start in background (detached)
docker compose up -d

# View logs
docker compose logs -f

# Stop the container
docker compose down
```

## Volume Mounts

### Required Volumes

| Path in Container | Purpose | Recommended Mount |
|-------------------|---------|-------------------|
| `/data/magi` | Database and application data | Named volume |
| `/data/manga` | Manga collection | Bind mount (read-only) |

### Mounting Your Manga Collection

**Linux/macOS:**
```bash
-v /home/user/manga:/data/manga:ro
```

**Windows (PowerShell):**
```bash
-v C:\Users\YourName\Manga:/data/manga:ro
```

**Windows (CMD):**
```bash
-v C:\Users\YourName\Manga:/data/manga:ro
```

**NAS/Network Share:**
```bash
-v /mnt/nas/manga:/data/manga:ro
```

> [!TIP]
> The `:ro` flag mounts the volume as read-only, preventing Magi from accidentally modifying your manga files.

## Environment Variables

Configure Magi's behavior with environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MAGI_DATA_DIR` | `/data/magi` | Directory for database and cache |
| `PORT` | `3000` | HTTP server port |
| `TZ` | `UTC` | Timezone for cron jobs and logs |

Example with custom port:

```bash
docker run -d \
  --name magi \
  -p 8080:8080 \
  -e PORT=8080 \
  -v /path/to/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

## Updating Magi

### With Docker Compose

```bash
# Pull the latest image
docker compose pull

# Recreate the container
docker compose up -d

# Remove old images
docker image prune -f
```

### With Docker CLI

```bash
# Stop and remove the old container
docker stop magi
docker rm magi

# Pull the latest image
docker pull alexbruun/magi:latest

# Start a new container with the same settings
docker run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release (recommended) |
| `v1.2.3` | Specific version |
| `develop` | Development build (may be unstable) |

## Verification

After starting the container, verify it's running correctly:

```bash
# Check container status
docker ps | grep magi

# View logs
docker logs magi

# Check health (if healthcheck configured)
docker inspect --format='{{.State.Health.Status}}' magi
```

Then open `http://localhost:3000` in your browser. You should see the Magi login page.

## Troubleshooting

### Container Won't Start

**Check logs:**
```bash
docker logs magi
```

**Common issues:**
- Port 3000 already in use → Change to another port: `-p 8080:3000`
- Volume mount paths incorrect → Verify paths exist and are readable
- Insufficient permissions → Ensure Docker has access to mounted directories

### Can't Access Web Interface

1. Verify container is running: `docker ps`
2. Check port mapping: `docker port magi`
3. Test from inside container: `docker exec magi wget -qO- http://localhost:3000`
4. Check firewall rules on host

### Database Issues

**Reset database (WARNING: Deletes all data):**
```bash
docker compose down
docker volume rm magi_magi-data
docker compose up -d
```

### Permission Errors

If Magi can't read your manga files:

```bash
# Check directory permissions
ls -la /path/to/manga

# Fix permissions (Linux/macOS)
chmod -R 755 /path/to/manga
```

## Next Steps

- [Configure your first library](../usage/getting_started.md)
- [Set up automatic indexing](../usage/configuration.md)
- [Learn about user roles and permissions](../usage/configuration.md#user-roles)

## Additional Resources

- [Docker Hub: alexbruun/magi](https://hub.docker.com/r/alexbruun/magi)
- [GitHub Container Registry](https://github.com/alexander-bruun/magi/pkgs/container/magi)
- [Magi GitHub Repository](https://github.com/alexander-bruun/magi)
