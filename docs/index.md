# Magi

A simple, fast manga server and reader.

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

### Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/alexander-bruun/magi/releases).

```bash
# Linux/macOS
tar -xzf magi-linux-amd64.tar.gz
./magi

# Windows
# Extract and run magi.exe
```

## Setup

1. Open http://localhost:3000
2. Create your admin account
3. Add your manga folder as a library
4. Start reading!

## Features

- 📚 Manga library management
- 🔍 Search and filter
- 📖 Web-based reader
- 👥 Multi-user support
- 📱 Mobile friendly
- 🔄 Auto-updates from MangaDex
- 🏷️ Collections and reading lists

## Links

- [GitHub](https://github.com/alexander-bruun/magi)
- [Docker Hub](https://hub.docker.com/r/alexbruun/magi)
