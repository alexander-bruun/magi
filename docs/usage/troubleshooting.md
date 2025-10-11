# Troubleshooting and debugging Magi

## Sqlite inspection

If you want to inspect the data stored in the Sqlite database, the `sqlite3` CLI can be used.

```bash
sudo dnf install sqlite-tools
sqlite3 ~/magi/magi.db
```

This will open a interactive console browser, here you can explore individual buckets, and the data contained within them.

Inspecting the current table schemas:

```bash
sqlite> .schema
```

Output (for syntax highlight):

```sql
CREATE TABLE schema_migrations (
                version INTEGER PRIMARY KEY
        );
CREATE TABLE jwt_keys (
    key TEXT PRIMARY KEY
);
CREATE TABLE libraries (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    cron TEXT NOT NULL,
    folders TEXT,  -- Serialized JSON array as text
    created_at INTEGER NOT NULL,  -- Unix timestamp
    updated_at INTEGER NOT NULL   -- Unix timestamp
);
CREATE TABLE mangas (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    author TEXT,
    description TEXT,
    year INTEGER,
    original_language TEXT,
    status TEXT,
    content_rating TEXT,
    library_slug TEXT,
    cover_art_url TEXT,
    path TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE chapters (
    manga_slug TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    file TEXT,
    chapter_cover_url TEXT,
    PRIMARY KEY (manga_slug, slug)
);
CREATE TABLE users (
    username TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    refresh_token_version INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL CHECK (role IN ('reader', 'moderator', 'admin')),
    banned BOOLEAN NOT NULL DEFAULT FALSE
);
```

## Sqlite migration testing

```bash
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Migrate the database up:

```bash
migrate -path ./migrations -database "sqlite3://~/magi/magi.db" up
```

Migrate the database down:

```bash
migrate -path ./migrations -database "sqlite3://~/magi/magi.db" down
```
