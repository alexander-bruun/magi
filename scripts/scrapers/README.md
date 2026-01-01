# MAGI Scrapers

Interactive manga/comic scraper with support for multiple sources.

## Supported Sources

| Scraper | Site |
|---------|------|
| asurascans | Asura Scans |
| demonicscans | Demonic Scans |
| flamecomics | Flame Comics |
| hivetoons | HiveToons |
| kunmanga | KunManga |
| lhtranslation | LHTranslation |
| manga18 | Manga18 |
| mangakatana | MangaKatana |
| manhwagalaxy | ManhwaGalaxy |
| omegascans | Omega Scans |
| qiscans | Qi Scans |
| resetscans | Reset Scans |
| thunderscans | Thunder Scans |
| tritinia | Tritinia |
| utoon | UToon |
| vortexscans | Vortex Scans |
| zscans | Z Scans |

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
  },
  "defaults": {
    "folder_base": "/media",
    "dry_run": false,
    "convert_to_webp": true
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
<parameter name="filePath">/home/alexa/magi/scripts/scrapers/README.md