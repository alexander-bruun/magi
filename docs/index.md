<div align="center">
  <img src="assets/img/icon.png" alt="Magi Icon" height="130"/>
</div>

<div align="center">
  <img alt="GitHub Release" src="https://img.shields.io/github/v/release/alexander-bruun/magi">
  <img alt="GitHub commit activity" src="https://img.shields.io/github/commit-activity/m/alexander-bruun/magi">
  <img alt="GitHub License" src="https://img.shields.io/github/license/alexander-bruun/magi">
  <img alt="GitHub Sponsors" src="https://img.shields.io/github/sponsors/alexander-bruun">
</div>

# Welcome to Magi Documentation

**Magi** is a self-hosted, lightweight manga server and reader designed for simplicity, performance, and ease of use. Whether you're managing a small personal collection or a large library, Magi provides all the tools you need to organize, index, and enjoy your digital manga collection.

![Magi Frontpage](/docs/images/frontpage.png)

> [!IMPORTANT]
> **Magi does NOT distribute copyrighted material.** It's designed exclusively as a local library manager for your legally obtained manga files. Metadata and cover art are fetched from public APIs like MangaDex to enrich your reading experience.

## What is Magi?

Magi is a complete manga management solution that runs as a single binary on your server, desktop, or NAS. It automatically indexes your manga collection, fetches metadata from MangaDex, and provides a beautiful web interface for reading and managing your library.

### Key Features

- **📚 Automatic Library Management**: Scan directories and auto-organize your collection
- **📖 Modern Reader**: Multiple reading modes (webtoon, single page, side-by-side)
- **👥 Multi-User Support**: Create accounts with role-based permissions
- **🔍 Advanced Search**: Filter by tags, types, content rating, and more
- **🎨 Rich Metadata**: Automatic cover art, descriptions, and tags from MangaDex
- **⚡ High Performance**: Written in Go with SQLite for speed and efficiency
- **🐳 Container Ready**: Docker images available for easy deployment
- **📱 Responsive Design**: Works on desktop, tablet, and mobile devices

## Quick Start

Choose your installation method:

=== "Docker"
    ```bash
    docker run -d \
      --name magi \
      -p 3000:3000 \
      -v /path/to/manga:/data/manga \
      -v /path/to/magi-data:/data/magi \
      alexbruun/magi:latest
    ```

=== "Binary"
    1. Download from [GitHub Releases](https://github.com/alexander-bruun/magi/releases)
    2. Run: `./magi`
    3. Open: `http://localhost:3000`

=== "Docker Compose"
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
        restart: unless-stopped
    volumes:
      magi-data:
    ```

After starting Magi, navigate to `http://localhost:3000` and create your admin account (the first user automatically becomes admin).

## Platform Support

Magi builds to a single portable binary for:

| Platform | Architectures |
|----------|---------------|
| Linux    | amd64, arm64  |
| macOS    | amd64, arm64  |
| Windows  | amd64, arm64  |

Docker images are available for `linux/amd64` and `linux/arm64`.

## File Format Support

Magi natively supports common manga archive formats:

- ✅ **CBZ** (Comic Book ZIP) - Recommended
- ✅ **CBR** (Comic Book RAR)
- ✅ **ZIP** archives
- ✅ **RAR** archives

> [!TIP]
> **Performance Tip**: CBZ/ZIP files offer better random access performance than RAR. For the best experience, consider converting RAR files to ZIP format.

## Architecture Overview

Magi is built with cutting-edge web technologies:

- **Backend**: Go + Fiber (Express-inspired web framework)
- **Database**: SQLite with automatic migrations
- **Templates**: Templ (type-safe HTML generation)
- **Frontend**: HTMX + Franken UI (minimal JavaScript)
- **Metadata**: MangaDex API (no authentication required)

Everything compiles into a single binary with embedded assets—no dependencies required.

## Documentation Structure

This documentation is organized into the following sections:

### [Installation](installation/docker.md)
Step-by-step guides for installing Magi on various platforms:

- [Docker](installation/docker.md) - Containerized deployment
- [Linux](installation/linux.md) - Native binary with systemd
- [Windows](installation/windows.md) - Native binary with Windows Service
- [Kubernetes](installation/kubernetes.md) - Kubernetes manifests
- [Podman](installation/podman.md) - Podman containers

### [Usage](usage/getting_started.md)
Learn how to use Magi effectively:

- [Getting Started](usage/getting_started.md) - First steps after installation
- [Configuration](usage/configuration.md) - Admin settings and environment variables
- [Troubleshooting](usage/troubleshooting.md) - Common issues and solutions

## Community & Support

- **GitHub**: [alexander-bruun/magi](https://github.com/alexander-bruun/magi)
- **Issues**: [Report bugs or request features](https://github.com/alexander-bruun/magi/issues)
- **Discussions**: [Ask questions and share ideas](https://github.com/alexander-bruun/magi/discussions)

## Contributing

Magi is open-source and welcomes contributions! Whether you want to:

- Fix bugs
- Add features
- Improve documentation
- Report issues

Check out the [Contributing Guide](https://github.com/alexander-bruun/magi/blob/main/README.md#-contributing) to get started.

## License

Magi is licensed under the [MIT License](https://github.com/alexander-bruun/magi/blob/main/LICENSE), allowing you to use it freely for personal or commercial purposes.

---

**Ready to get started?** Head to the [Installation Guide](installation/docker.md) to set up Magi, or jump to [Getting Started](usage/getting_started.md) if you've already installed it.
