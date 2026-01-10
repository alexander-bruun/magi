# MAGI Scrapers

Interactive manga/comic scraper with support for multiple sources.

## Supported Sources

| Scraper | Site |
|---------|------|
| asmotoon | AsmoToon |
| asurascans | Asura Scans |
| demonicscans | Demonic Scans |
| drakecomic | DrakeComic |
| ezmanga | EzManga |
| flamecomics | Flame Comics |
| hivetoons | HiveToons |
| kunmanga | KunManga |
| lhtranslation | LHTranslation |
| magustoon | MagusToon |
| manga18 | Manga18 |
| mangakatana | MangaKatana |
| mangayy | MangaYY |
| manhuafast | ManhuaFast |
| manhwagalaxy | ManhwaGalaxy |
| manhwahub | ManhwaHub |
| omegascans | Omega Scans |
| qiscans | Qi Scans |
| resetscans | Reset Scans |
| setsuscans | Setsu Scans |
| spiderscans | Spider Scans |
| stonescape | StoneScape |
| thunderscans | Thunder Scans |
| toongod | ToonGod |
| tritinia | Tritinia |
| utoon | UToon |
| vortexscans | Vortex Scans |
| zscans | Z Scans |
| nexcomic | NexComic |
| genzupdates | GenzUpdates |
| luacomic | LuaComic |

## Installation

```bash
cd scripts/scrapers
pip install -r requirements.txt
```

## Usage

### Interactive Mode

```bash
python interactive_scraper.py
```

Use arrow keys or numbers to select a scraper, then follow the prompts.

### Direct Scraper

```bash
python asurascans.py
```

## Configuration

Create `config.json` for default settings (optional):

```json
{
  "scrapers": {
    "asurascans": {
      "folder": "/media/manga",
      "dry_run": false,
      "convert_to_webp": true
    }
  }
}
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `folder` | Download directory | prompted |
| `dry_run` | Skip downloads, show what would happen | `false` |
| `convert_to_webp` | Convert images to WebP | `true` |
| `webp_quality` | WebP quality (env var) | `100` |
| `priority` | Priority level for duplicate prevention (higher = preferred source) | `1` |

### Priority System

The priority system prevents duplicate downloads by allowing higher priority scrapers to download series, while lower priority scrapers skip series that already exist in higher priority providers.

**Use Cases:**
- Set high priority (e.g., 1000) for original/official sources
- Set low priority (e.g., 1) for aggregator sites
- This ensures you get content from preferred sources instead of copies

**How it works:**
- When a scraper encounters a series, it checks if the same series exists in any folder with higher priority
- If found, the scraper skips that series
- Series titles are normalized for matching (removes special characters, handles variations)

**Example Configuration:**
```json
{
  "scrapers": {
    "asurascans": {
      "priority": 1000  // Original source - highest priority
    },
    "manhwahub": {
      "priority": 1     // Aggregator - low priority
    }
  }
}
```

## Output

```
Target_Folder/
└── Series Name [Source]/
    ├── Series Name Ch.001 [Source].cbz
    ├── Series Name Ch.002 [Source].cbz
    └── ...
```

## Log Levels

- `[INFO]` - Progress information
- `[SUCCESS]` - Completed operations
- `[WARNING]` - Non-critical issues
- `[ERROR]` - Failed operations</content>
