# Installation

## Docker (Recommended)

```bash
docker run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga:ro \
  -v magi-data:/data/magi \
  alexbruun/magi:latest
```

## Binary

1. Download the latest release from [GitHub Releases](https://github.com/alexander-bruun/magi/releases)
2. Extract the archive
3. Run the binary: `./magi` (Linux/macOS) or `magi.exe` (Windows)
4. Open http://localhost:3000

## Docker Compose

```yaml
version: '3.8'
services:
  magi:
    image: alexbruun/magi:latest
    ports:
      - "3000:3000"
    volumes:
      - /path/to/manga:/data/manga:ro
      - magi-data:/data/magi
volumes:
  magi-data:
```