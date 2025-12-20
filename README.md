<div align="center">
  <img src="assets/img/icon.png" alt="Magi Icon" height="130"/>
</div>

<div align="center">
  <img alt="GitHub Release" src="https://img.shields.io/github/v/release/alexander-bruun/magi">
  <img alt="GitHub commit activity" src="https://img.shields.io/github/commit-activity/m/alexander-bruun/magi">
  <img alt="GitHub License" src="https://img.shields.io/github/license/alexander-bruun/magi">
  <img alt="GitHub Sponsors" src="https://img.shields.io/github/sponsors/alexander-bruun">
  <img alt="Tests" src="https://github.com/alexander-bruun/magi/actions/workflows/test.yml/badge.svg">
</div>

# Magi

**Magi** is a self-hosted, lightweight manga & light novel server and reader built for simplicity and performance. It helps you organize, index, and read your personal digital manga collection through a modern web interface.

![Magi Frontpage](/docs/images/home.png)

> [!IMPORTANT]
> **Magi does NOT distribute copyrighted material.** It's designed exclusively as a local library manager for your legally obtained manga files. Metadata and cover art are fetched from public APIs like MangaDex to enrich your reading experience.

## ✨ Features

### 📚 Library Management
- **Automatic Indexing**: Scan local directories and automatically organize your manga & light novels collection
- **Multi-Library Support**: Organize manga across multiple libraries with custom scan schedules
- **Metadata Fetching**: Automatically retrieve titles, descriptions, cover art, tags, and more from public and private metadata providers
- **Smart Duplicate Detection**: Identify and manage duplicate manga across different folders
- **Manual Metadata Editing**: Override automatic metadata with custom information

### 📖 Reading Experience
- **Multiple Reading Modes**: Webtoon (vertical scroll), single page, and side-by-side views
- **Progress Tracking**: Automatic chapter read/unread status with per-user tracking
- **Keyboard Navigation**: Full keyboard shortcuts for efficient reading
- **Responsive Design**: Optimized for desktop, tablet, and mobile devices
- **Lazy Loading**: Fast page loads with progressive image loading

### 👥 User Management
- **Multi-User Support**: Create accounts for family members or friends
- **Role-Based Access**: Three permission levels (reader, moderator, admin)
- **Personal Libraries**: Favorites, reading lists, and voting per user
- **User Banning**: Administrators can ban problematic users

### 🔍 Discovery & Organization
- **Advanced Search**: Filter by title, author, tags, type, content rating, and library
- **Tag System**: Browse and filter manga by genres and themes
- **Favorites & Reading Lists**: Track what you're reading and save favorites
- **Voting System**: Upvote or downvote manga to help organize your collection
- **Content Rating Filters**: Filter manga by safe, suggestive, erotica, or pornographic ratings

### 🛠️ Administration
- **Web-Based Scrapers**: Create custom JavaScript scrapers through the admin interface
- **Live Job Monitoring**: WebSocket-based real-time progress for indexing and scraping jobs
- **Configuration Dashboard**: Control registration, user limits, and global settings
- **Database Migrations**: Automatic schema versioning and upgrades

### 🏗️ Technical Highlights
- **Single Binary**: No dependencies, just download and run
- **Embedded Assets**: All CSS, JS, and views compiled into the binary
- **SQLite Database**: Zero-configuration database with automatic migrations
- **Efficient Archive Handling**: Native support for `.cbz`, `.cbr`, `.zip`, `.rar` and `.epub` formats (other formats planned: `PDF`)
- **HTMX-Powered UI**: Smooth, responsive interface without heavy JavaScript frameworks

## 📦 Supported Platforms

Magi compiles to a single portable binary for:

| OS | Architectures |
|---|---|
| **Linux** | `amd64`, `arm64` |
| **macOS** | `amd64`, `arm64` |
| **Windows** | `amd64`, `arm64` |

Docker images are available for `linux/amd64` and `linux/arm64` on [Docker Hub](https://hub.docker.com/r/alexbruun/magi).

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

1. Download the latest release from [GitHub Releases](https://github.com/alexander-bruun/magi/releases)
2. Extract and run:
   ```bash
   chmod +x magi
   ./magi
   ```
3. Open `http://localhost:3000` in your browser
4. Create your admin account (first user automatically becomes admin)

**Full installation guides**: See the [documentation](https://alexander-bruun.github.io/magi/) for detailed setup instructions including systemd services, Windows services, Kubernetes, and more.

## 🏗️ Architecture

Magi is built with modern, performant technologies:

- **[Go](https://go.dev/)** - High-performance backend runtime
- **[Fiber](https://docs.gofiber.io/)** - Express-inspired web framework
- **[SQLite](https://github.com/ncruces/go-sqlite3)** - Embedded relational database
- **[Templ](https://templ.guide/)** - Type-safe HTML templating
- **[HTMX](https://htmx.org/)** - Dynamic HTML without heavy JavaScript
- **[Franken UI](https://franken-ui.dev/)** - Modern UI component library

All assets (CSS, JavaScript, templates) are embedded into the binary at compile time, making Magi truly portable with zero external dependencies.

## 📖 Usage Overview

### Creating Libraries

1. Navigate to **Admin > Libraries**
2. Click **New Library**
3. Configure:
   - **Name**: Display name for the library
   - **Description**: Optional description
   - **Folders**: Local paths to scan (one per line)
   - **Cron Schedule**: Automatic re-scan frequency (e.g., `0 2 * * *` for 2 AM daily)

### Reading Manga

1. Browse the **Mangas** page or search by title
2. Click a manga to view details and chapters
3. Select a chapter to start reading
4. Use keyboard shortcuts:
   - `→` / `←`: Next/previous page
   - `Space`: Next page
   - `Home` / `End`: First/last page
5. Switch reading modes in the reader toolbar

### Managing Metadata

Moderators and admins can update manga metadata:

- **Auto-refresh**: Re-scan the manga folder to detect new chapters
- **Metadata Search**: Find and apply metadata from multiple providers
- **Manual Edit**: Override any field with custom values

## 🔧 Configuration

### Environment Variables

```bash
# Data directory (default: OS-specific)
MAGI_DATA_DIR=/path/to/data

# Cache directory (default: $MAGI_DATA_DIR/cache)
MAGI_CACHE_DIR=/path/to/cache

# Server port (default: 3000)
PORT=3000
```

### Admin Configuration Page

Access **Admin > Configuration** to control:

- **Allow Registration**: Enable/disable new user signups
- **Max Users**: Limit total user accounts (0 = unlimited)

The first user to register automatically receives admin privileges.

## 🧑‍💻 Development

### Prerequisites

- Go 1.21+
- [Air](https://github.com/cosmtrek/air) (for live reload)
- [Templ](https://templ.guide/) (for template compilation)

### Running Locally

```bash
# Clone the repository
git clone https://github.com/alexander-bruun/magi.git
cd magi

# Start with live reload
air
```

The application will be available at:

- `http://localhost:3000` - Main application
- `http://localhost:3001` - Auto-reloading proxy (development only)

## 🤝 Contributing

Contributions are welcome! Magi is in active development, and we'd love your help improving it.

### Ways to Contribute

- 🐛 **Report bugs** via GitHub Issues
- 💡 **Suggest features** you'd like to see
- 📖 **Improve documentation**
- 🔧 **Submit pull requests**

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Test thoroughly
5. Commit: `git commit -m 'Add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request to the `next` branch

> [!NOTE]
> We primarily develop in the `next` branch and merge to `main` for releases. Please target `next` with your PRs.

## ⚠️ Known Limitations

- **RAR Performance**: RAR archives have slower random access than ZIP. Consider converting to CBZ for better performance.

## 📄 License

[MIT License](LICENSE) - Feel free to use Magi for personal or commercial purposes.
