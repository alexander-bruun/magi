# Similar Mangas Logic Rework - Implementation Summary

## Overview
The "Better" page has been completely reworked to use a database-driven approach for detecting manga duplicates instead of the previous runtime folder comparison logic.

## Key Changes

### 1. Database Schema (New Migration)
**Files Created:**
- `/home/alexa/magi/migrations/0011_create_manga_duplicates_table.up.sql`
- `/home/alexa/magi/migrations/0011_drop_manga_duplicates_table.down.sql`

**New Table:** `manga_duplicates`
- Tracks when different folders are detected as adding chapters to the same manga
- Fields: id, manga_slug, library_slug, folder_path_1, folder_path_2, dismissed, created_at
- Includes indexes for performance and foreign key constraints

### 2. Model Updates
**File:** `/home/alexa/magi/models/folder_similarity.go`

**New Structure:** `MangaDuplicate`
```go
type MangaDuplicate struct {
    ID           int64
    MangaSlug    string
    MangaName    string
    LibrarySlug  string
    LibraryName  string
    FolderPath1  string
    FolderPath2  string
    Dismissed    bool
    CreatedAt    int64
}
```

**New Functions:**
- `CreateMangaDuplicate()` - Records a new duplicate detection
- `GetActiveMangaDuplicates(page, limit)` - Retrieves non-dismissed duplicates with pagination
- `DismissMangaDuplicate(id)` - Marks a duplicate as dismissed
- `ClearMangaDuplicatesForLibrary(librarySlug)` - Clears all duplicates for a library
- `GetMangaDuplicateByFolders()` - Checks if a duplicate record already exists

### 3. Indexer Logic
**File:** `/home/alexa/magi/indexer/indexer.go`

**Changes:**
- Added duplicate detection in `IndexManga()` function
- When indexing a manga, if an existing manga is found with a different path, it's recorded as a duplicate
- The indexer automatically creates a `MangaDuplicate` entry when this occurs
- Chapters from both folders are still indexed normally

### 4. Better Page View
**File:** `/home/alexa/magi/views/better.templ`

**Complete Rewrite:**
- Now uses a UIKit striped table layout
- Shows manga name, library, and both folder paths in a clean table format
- Each row has a "Dismiss" button using HTMX for inline actions
- Includes pagination controls with page numbers
- Shows total count of duplicates
- Folder paths display only the basename for cleaner UI

**Table Columns:**
1. Manga (with link to manga page)
2. Library (with badge)
3. Folder 1 (basename)
4. Folder 2 (basename)
5. Actions (Dismiss button)

### 5. Handler Updates
**File:** `/home/alexa/magi/handlers/library_handler.go`

**Modified:** `HandleBetter()`
- Now fetches manga duplicates from database instead of runtime folder comparison
- Supports pagination (20 items per page)
- Calculates total pages
- Removed old `findDuplicatesInLibrary()` and `getSubdirectories()` functions

**Added:** `HandleDismissDuplicate()`
- Handles HTMX POST requests to dismiss duplicates
- Returns empty response to remove the table row via HTMX

### 6. Routes
**File:** `/home/alexa/magi/handlers/routes.go`

**Added:**
- `/api/duplicates/:id/dismiss` (POST) - Admin-only endpoint to dismiss duplicates

## How It Works

### Detection Flow
1. During library scanning, when a manga folder is indexed
2. The indexer checks if a manga with the same slug already exists
3. If it exists with a different path, a duplicate record is created
4. The duplicate is stored in the database with both folder paths

### User Experience
1. Admin visits the "Better" page
2. Sees a table of all detected duplicates
3. Can navigate through pages if there are many duplicates
4. Can dismiss individual duplicates by clicking the "Dismiss" button
5. Dismissed duplicates are hidden from the view

### Benefits Over Previous Approach
- **Performance:** No runtime folder comparison needed
- **Persistence:** Duplicate detections are stored and tracked over time
- **Accuracy:** Detects actual manga duplicates (same manga, different folders) rather than similar folder names
- **User Control:** Admin can dismiss false positives
- **Scalability:** Pagination handles large numbers of duplicates

## Migration Steps

To apply these changes:

1. **Run database migrations** to create the new table
2. **Rebuild the templ files** if needed: `templ generate`
3. **Restart the application**
4. **Re-index libraries** to populate the new duplicate detection

## Notes

- The old `FolderSimilarity` struct is kept for backward compatibility but is no longer actively used
- The old `LibraryDuplicates` and `DuplicateFolder` structs in library.go remain but are unused
- Dismissed duplicates remain in the database but won't appear on the Better page
- Running a library scan will automatically detect and record new duplicates
