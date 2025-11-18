# Podman Installation Guide

Podman is a daemonless container engine that's compatible with Docker. This guide covers installing and running Magi with Podman.

## Prerequisites

- Podman 3.0+ installed
- 512MB+ available RAM
- Storage for manga collection and database

## Installing Podman

### Fedora/RHEL/CentOS

```bash
sudo dnf install podman
```

### Ubuntu/Debian

```bash
sudo apt update
sudo apt install podman
```

### macOS

```bash
brew install podman
podman machine init
podman machine start
```

### Windows

Download from [Podman Desktop](https://podman-desktop.io/) or use:

```powershell
winget install RedHat.Podman
```

## Quick Start

### Basic Run Command

```bash
podman run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest
```

Replace `/path/to/manga` with your actual manga directory.

> [!NOTE]
> The `:Z` flag is important for SELinux systems (Fedora, RHEL, CentOS). It relabels the volume for container access.

## Podman Compose

### Installing Podman Compose

```bash
# Using pip
pip3 install podman-compose

# Or using system packages (Fedora/RHEL)
sudo dnf install podman-compose
```

### Basic Configuration

Create `docker-compose.yml` (compatible with Podman):

```yaml
version: '3.8'

services:
  magi:
    image: alexbruun/magi:latest
    container_name: magi
    ports:
      - "3000:3000"
    volumes:
      - /path/to/manga:/data/manga:ro
      - magi-data:/data/magi
    environment:
      - MAGI_DATA_DIR=/data/magi
      - PORT=3000
    restart: unless-stopped

volumes:
  magi-data:
```

### Running with Podman Compose

```bash
# Start services
podman-compose up -d

# View logs
podman-compose logs -f

# Stop services
podman-compose down
```

## Rootless Podman (Recommended)

Podman can run containers as a regular user without root privileges.

### Setup Rootless

```bash
# Enable user namespaces (if needed)
echo "user.max_user_namespaces=28633" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Verify rootless mode works
podman run --rm hello-world
```

### Run Magi Rootless

```bash
podman run -d \
  --name magi \
  -p 3000:3000 \
  -v ~/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

## Systemd Integration

Run Magi as a systemd service for automatic startup.

### Generate Systemd Unit

```bash
# Create the container first
podman run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest

# Generate systemd unit file
podman generate systemd --new --name magi > ~/.config/systemd/user/magi.service

# Stop the temporary container
podman stop magi
podman rm magi
```

### Enable and Start Service

```bash
# Reload systemd
systemctl --user daemon-reload

# Enable service to start on boot
systemctl --user enable magi

# Start service
systemctl --user start magi

# Check status
systemctl --user status magi

# View logs
journalctl --user -u magi -f
```

### Enable Linger (Persist After Logout)

```bash
# Allow user services to run after logout
sudo loginctl enable-linger $USER
```

## Pods (Multi-Container Setup)

Podman pods group containers with shared networking.

### Create a Pod

```bash
# Create pod with exposed port
podman pod create \
  --name magi-pod \
  -p 3000:3000

# Run Magi in the pod
podman run -d \
  --pod magi-pod \
  --name magi \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest

# Add additional containers if needed (e.g., reverse proxy)
```

## Volume Management

### Named Volumes

```bash
# Create volume
podman volume create magi-data

# Inspect volume
podman volume inspect magi-data

# List volumes
podman volume ls

# Remove volume
podman volume rm magi-data
```

### Bind Mounts

```bash
# Mount specific directory
podman run -d \
  --name magi \
  -v /home/user/manga:/data/manga:ro,Z \
  -v /home/user/magi-data:/data/magi:Z \
  alexbruun/magi:latest
```

## Network Configuration

### Port Mapping

```bash
# Map to different host port
podman run -d --name magi -p 8080:3000 alexbruun/magi:latest

# Bind to specific interface
podman run -d --name magi -p 192.168.1.100:3000:3000 alexbruun/magi:latest

# Map multiple ports
podman run -d --name magi -p 3000:3000 -p 3001:3001 alexbruun/magi:latest
```

### Custom Networks

```bash
# Create network
podman network create magi-net

# Run container on custom network
podman run -d \
  --name magi \
  --network magi-net \
  alexbruun/magi:latest
```

## Managing Containers

### Basic Commands

```bash
# List running containers
podman ps

# List all containers
podman ps -a

# View logs
podman logs magi
podman logs -f magi  # Follow

# Stop container
podman stop magi

# Start container
podman start magi

# Restart container
podman restart magi

# Remove container
podman rm magi

# Execute command in container
podman exec -it magi /bin/sh
```

### Resource Limits

```bash
podman run -d \
  --name magi \
  --memory=2g \
  --cpus=2 \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest
```

## Updating Magi

### Manual Update

```bash
# Stop and remove old container
podman stop magi
podman rm magi

# Pull latest image
podman pull alexbruun/magi:latest

# Start new container
podman run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest
```

### With Systemd

```bash
# Pull new image
podman pull alexbruun/magi:latest

# Restart service (will use new image if --new flag was used)
systemctl --user restart magi
```

## Auto-Update

Configure automatic image updates:

```bash
# Run with auto-update label
podman run -d \
  --name magi \
  --label "io.containers.autoupdate=registry" \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro,Z \
  -v magi-data:/data/magi:Z \
  alexbruun/magi:latest

# Enable auto-update timer
systemctl --user enable --now podman-auto-update.timer

# Check update status
systemctl --user status podman-auto-update.timer

# Manually trigger update
podman auto-update
```

## Troubleshooting

### SELinux Issues

If you get permission denied errors on SELinux systems:

```bash
# Use Z flag for private volume labeling
-v /path:/container/path:Z

# Or z flag for shared volume labeling
-v /path:/container/path:z

# Check SELinux context
ls -lZ /path/to/manga

# Restore SELinux context
sudo restorecon -Rv /path/to/manga
```

### Port Already in Use

```bash
# Find process using port
sudo ss -tulpn | grep 3000

# Use different port
podman run -d --name magi -p 8080:3000 alexbruun/magi:latest
```

### Permission Errors

```bash
# Check file ownership
ls -la /path/to/manga

# Fix permissions
chmod -R 755 /path/to/manga

# For rootless, ensure files are readable by your user
chown -R $USER:$USER /path/to/manga
```

### Container Won't Start

```bash
# Check logs
podman logs magi

# Inspect container
podman inspect magi

# Check events
podman events --filter container=magi
```

## Podman vs Docker Differences

| Feature | Docker | Podman |
|---------|--------|--------|
| Daemon | Required | Daemonless |
| Root | Usually needs root | Rootless by default |
| Compatibility | Docker-specific | OCI-compliant |
| Systemd | External tools | Built-in integration |
| Pods | Docker Compose | Native pods support |

Most Docker commands work with Podman by replacing `docker` with `podman`.

## Migration from Docker

### Using podman-docker Package

```bash
# Install Docker compatibility
sudo dnf install podman-docker

# Now 'docker' command uses Podman
docker ps  # Actually runs podman ps
```

### Manual Alias

```bash
# Add to ~/.bashrc or ~/.zshrc
alias docker=podman

# Reload shell
source ~/.bashrc
```

## Verification

After starting Magi, verify it's working:

```bash
# Check container status
podman ps | grep magi

# Test HTTP endpoint
curl http://localhost:3000

# Check resource usage
podman stats magi
```

Then open `http://localhost:3000` in your browser.

## Cleanup

Remove all Magi resources:

```bash
# Stop and remove container
podman stop magi
podman rm magi

# Remove volumes (WARNING: Deletes data)
podman volume rm magi-data

# Remove images
podman rmi alexbruun/magi:latest

# Remove pod (if used)
podman pod rm -f magi-pod
```

## Next Steps

- [Create your first library](../usage/getting_started.md)
- [Configure automatic indexing](../usage/configuration.md)
- [Learn about systemd integration](../usage/troubleshooting.md)

## Additional Resources

- [Podman Documentation](https://docs.podman.io/)
- [Podman Desktop](https://podman-desktop.io/)
- [Red Hat Podman Guide](https://www.redhat.com/sysadmin/podman-guides)
