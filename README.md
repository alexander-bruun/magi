<div align="center">
  <img src="assets/img/icon.png" alt="Magi Icon" height="130"/>
</div>

<div align="center">
  <img alt="GitHub Release" src="https://img.shields.io/github/v/release/alexander-bruun/magi">
  <img alt="GitHub commit activity" src="https://img.shields.io/github/commit-activity/m/alexander-bruun/magi">
  <img alt="GitHub License" src="https://img.shields.io/github/license/alexander-bruun/magi">
  <img alt="GitHub Sponsors" src="https://img.shields.io/github/sponsors/alexander-bruun">
</div>

# Magi

**Magi** is a self-hosted, lightweight manga & light novel server and reader. Organize and read your personal digital manga collection through a modern web interface.

<div align="center">
  <img src="docs/example.png" alt="Magi Example Homepage" width="600"/>
</div>

> [!IMPORTANT]
> **Magi does NOT distribute copyrighted material.** It's designed exclusively as a local library manager for your legally obtained manga files.

## ✨ Features

- **Library Management**: Automatic indexing, multi-library support, metadata fetching
- **Reading Experience**: Multiple reading modes, progress tracking, keyboard navigation
- **User Management**: Multi-user support with role-based access
- **Discovery**: Advanced search, tags, favorites, and reading lists
- **Administration**: Web-based configuration and job monitoring

## 📦 Supported Platforms

Magi compiles to a single portable binary for Linux, Windows, and macOS (amd64, arm64). Docker images available on [Docker Hub](https://hub.docker.com/r/alexbruun/magi).

> [!NOTE]
> macOS builds do not include WebP image support due to platform limitations.

## 🚀 Quick Start

### Using Docker (Recommended)

```bash
docker run -d \
  --name magi \
  -p 3000:3000 \
  -v /path/to/manga:/data/manga \
  -v /path/to/magi-data:/data/magi \
  alexbruun/magi:latest
```

### Using Binary

1. Download from [GitHub Releases](https://github.com/alexander-bruun/magi/releases)
2. Extract and run: `./magi`
3. Open `http://localhost:3000` and create your admin account

**Full docs**: [Installation & Usage](https://alexander-bruun.github.io/magi/)

## 🧑‍💻 Development

Requires Go 1.25+. Clone and run with `air` for live reload.

## 🤝 Contributing

Contributions welcome! Report bugs, suggest features, or submit PRs to the `next` branch.

## 📄 License

[MIT License](LICENSE)
