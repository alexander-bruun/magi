# Getting Started with Magi

This guide will walk you through the initial setup and basic usage of Magi after installation.

## First Steps

### 1. Access Magi

Open your web browser and navigate to:

```
http://localhost:3000
```

Or if accessing from another device on your network:

```
http://[server-ip]:3000
```

### 2. Create Your Admin Account

On first launch, you'll see the registration page. Create your account:

1. Enter a **username** (alphanumeric, no spaces)
2. Enter a **password** (minimum 8 characters recommended)
3. Confirm your password
4. Click **Register**

> [!IMPORTANT]
> **The first user to register automatically becomes an administrator** with full access to all features.

After registration, you'll be logged in automatically.

## Creating Your First Library

Libraries are collections of manga organized by folder paths. Let's create your first library.

### 1. Navigate to Libraries

Click **Admin** in the navigation bar, then select **Libraries**.

### 2. Create a New Library

Click the **New Library** button.

### 3. Configure Library Settings

Fill in the library details:

| Field | Description | Example |
|-------|-------------|---------|
| **Name** | Display name for the library | `My Manga Collection` |
| **Description** | Optional description | `Main manga library` |
| **Folders** | Paths to scan (one per line) | `/data/manga`<br>`/mnt/nas/manga` |
| **Cron Schedule** | When to auto-scan (cron format) | `0 2 * * *` (2 AM daily) |

#### Understanding Cron Schedules

Cron schedules determine when Magi automatically re-scans your library for changes.

**Common schedules:**

| Schedule | Meaning | When It Runs |
|----------|---------|--------------|
| `0 2 * * *` | Daily at 2 AM | Every day at 2:00 AM |
| `0 */6 * * *` | Every 6 hours | 12 AM, 6 AM, 12 PM, 6 PM |
| `0 0 * * 0` | Weekly on Sunday | Every Sunday at midnight |
| `@hourly` | Every hour | Top of every hour |
| `@daily` | Daily at midnight | Every day at 12:00 AM |

**Cron format:** `minute hour day month weekday`

Use [crontab.guru](https://crontab.guru/) to build custom schedules.

### 4. Save the Library

Click **Save** to create the library.

### 5. Start Initial Indexing

After creating the library:

1. You'll see your new library in the list
2. Click the **Index Now** button to start scanning
3. Monitor progress in the **Admin > Job Status** page

The indexing process will:

- Scan all folders for manga files
- Detect manga by folder/file names
- Fetch metadata from MangaDex
- Download cover art
- Index all chapters

## Organizing Your Manga Files

Magi works best when your manga is organized logically.

### Recommended Folder Structure

```
/data/manga/
├── One Piece/
│   ├── Chapter 001.cbz
│   ├── Chapter 002.cbz
│   └── ...
├── Attack on Titan/
│   ├── Volume 01/
│   │   ├── Chapter 001.cbz
│   │   └── Chapter 002.cbz
│   └── Volume 02/
│       └── ...
└── Naruto/
    ├── 001 - Uzumaki Naruto.cbz
    ├── 002 - Konohamaru.cbz
    └── ...
```

### Supported Archive Formats

- ✅ **CBZ** (ZIP-based) - Recommended
- ✅ **CBR** (RAR-based)
- ✅ **ZIP** files
- ✅ **RAR** files

### File Naming Tips

Magi is smart about parsing manga names from folders and files:

**Good naming:**
- `Manga Name/Chapter 001.cbz`
- `[Author] Manga Name/Vol.01 Ch.001.cbz`
- `Series Title (2020)/Chapter 001 - Title.cbz`

**What Magi ignores:**
- Brackets: `[Group]`, `(Year)`
- Common patterns: `Vol.`, `Ch.`, `Chapter`, etc.
- File extensions

**Single-file manga:**

You can also have manga as individual files:
```
/data/manga/
├── One Shot Title.cbz
├── Short Story.cbz
```

## Browsing Your Collection

### Home Page

The home page shows:

- **Recently Added**: Latest manga in your libraries
- **Popular**: Most favorited manga
- **Statistics**: Total manga, chapters, and users

### Mangas Page

Click **Mangas** in the navigation to browse your full collection.

**Features:**

- **Search**: Type to filter by title or author
- **Sort**: By name, date added, favorites
- **Filter**: By library, tags, content rating, type
- **Tags**: Select multiple tags to narrow results

### Reading Manga

1. Click on a manga cover to view details
2. Browse the chapter list
3. Click a chapter to start reading

## Reading Modes

Magi offers three reading modes:

### Webtoon Mode

- Vertical scrolling
- All pages loaded in sequence
- Best for: Webtoons and continuous reading
- **Shortcut**: Select "Webtoon" in reader toolbar

### Single Page Mode

- One page at a time
- Navigate with arrows or keyboard
- Best for: Traditional manga
- **Shortcuts**: 
  - `→` or `Space`: Next page
  - `←`: Previous page

### Side-by-Side Mode

- Two pages displayed together
- Mimics physical manga reading
- Best for: Double-page spreads
- **Navigation**: Same as single page mode

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `→` | Next page |
| `←` | Previous page |
| `Space` | Next page |
| `Home` | First page |
| `End` | Last page |
| `Esc` | Exit reader |

## User Features

### Favorites

Mark manga as favorites:

1. Open a manga page
2. Click the **heart icon** ★
3. Access your favorites from the **Account** menu

### Reading Progress

Magi automatically tracks:

- Which chapters you've read
- Your current position in each manga
- Recently read manga

View your reading list: **Account > Reading**

### Voting

Show your preferences by voting:

- **Upvote** ▲: Manga you love
- **Downvote** ▼: Manga you dislike

This helps you organize your collection and can influence future features.

## Managing Metadata

### Automatic Metadata

When indexing, Magi automatically:

1. Searches MangaDex for matching manga
2. Downloads metadata (title, author, description, tags)
3. Fetches cover art
4. Applies content rating and status

### Manual Metadata Editing

If automatic metadata is incorrect:

1. Open the manga page
2. Click **Update Metadata** (moderator/admin only)
3. Choose an option:
   - **Search MangaDex**: Find and apply different metadata
   - **Manual Edit**: Enter custom information
   - **Refresh**: Re-scan the folder for new chapters

### Refresh vs Re-Index

- **Refresh**: Detects new chapters, keeps existing metadata
- **Re-Index**: Completely rescans, refetches metadata

## User Roles

Magi has three user roles:

| Role | Permissions |
|------|-------------|
| **Reader** | Browse, read, track progress, favorite |
| **Moderator** | Reader + edit metadata, manage manga |
| **Admin** | Moderator + manage users, libraries, configuration |

Admins can change user roles from **Admin > Users**.

## Next Steps

Now that you're set up, explore these features:

- [Configure advanced settings](configuration.md)
- [Set up user accounts for family/friends](configuration.md#user-management)
- [Troubleshoot common issues](troubleshooting.md)
- [Learn about web-based scrapers](#) (coming soon)

## Tips for a Better Experience

### Performance

- Use **CBZ/ZIP** instead of RAR for better performance
- Store manga on fast storage (SSD if possible)
- For large collections, increase indexing intervals

### Organization

- Use consistent naming conventions
- Group manga by series in folders
- Consider separating completed vs ongoing series
- Tag manga appropriately for easy filtering

### Maintenance

- Regularly check **Job Status** for indexing issues
- Monitor disk space usage
- Back up your database periodically
- Update Magi to get new features and fixes

## Common Questions

**Q: How long does initial indexing take?**  
A: Depends on collection size. Roughly 1-2 seconds per manga. A 1000-manga library takes 15-30 minutes.

**Q: Can I have multiple libraries?**  
A: Yes! Create as many as you need for different collections.

**Q: What if MangaDex doesn't have my manga?**  
A: Use manual metadata editing to add custom information.

**Q: Can I import my existing metadata?**  
A: Currently, Magi fetches fresh metadata. Manual editing is available.

**Q: How do I add more users?**  
A: They can register themselves (if enabled) or admins can create accounts.

**Q: Is there a mobile app?**  
A: Not yet, but the web interface is mobile-responsive.

## Getting Help

- **Documentation**: You're reading it!
- **GitHub Issues**: [Report bugs or request features](https://github.com/alexander-bruun/magi/issues)
- **Discussions**: [Ask questions and share ideas](https://github.com/alexander-bruun/magi/discussions)

Enjoy reading your manga with Magi! 📚✨
