# Delete Duplicate Folder Feature

## Overview
Added a "Delete" button next to the "Dismiss" button in the Better (duplicates) page that allows administrators to delete one of the duplicate folders after reviewing detailed information about each folder.

## Features Implemented

### 1. Delete Button in UI
- **Location**: Right next to the "Dismiss" button in each duplicate row
- **Icon**: Trash2 icon
- **Action**: Opens a modal with folder details

### 2. Folder Information Modal
The modal displays detailed information about both duplicate folders:

#### For Each Folder:
- **Folder Name**: Base name of the folder
- **Full Path**: Complete filesystem path
- **File Count**: Total number of files in the folder
- **Last Modified**: Timestamp of last modification
- **Exists Status**: Whether the folder currently exists on disk

#### Visual Layout:
- Side-by-side cards for easy comparison
- Color-coded status (danger alert if folder doesn't exist)
- Individual delete button under each folder card

### 3. Double Confirmation
Two levels of confirmation before deletion:
1. **Modal Selection**: User must actively click the delete button for a specific folder
2. **Browser Confirmation**: Native browser confirm dialog with clear warning message showing:
   - The exact folder name
   - Warning about permanent deletion
   - Notification that action cannot be undone

### 4. Backend Implementation

#### New API Endpoints:
- `GET /api/duplicates/:id/folder-info` - Retrieves detailed folder information
- `DELETE /api/duplicates/:id/folder` - Deletes the specified folder

#### New Model Functions (`models/manga_duplicates.go`):
- `GetDuplicateFolderInfo(duplicateID int64)` - Gets folder details for both folders
- `getFolderInfo(path string)` - Gets detailed info for a single folder
- `DeleteDuplicateFolder(duplicateID, folderPath)` - Deletes folder and cleans up

#### Folder Information Structure:
```go
type FolderInfo struct {
    Path         string // Full path
    BaseName     string // Folder name
    FileCount    int    // Number of files
    LastModified int64  // Unix timestamp
    Exists       bool   // Existence check
}
```

### 5. Smart Cleanup Logic

After deleting a folder, the system automatically:
1. **Removes folder from disk** using `os.RemoveAll()`
2. **Deletes duplicate entry** from database (since one folder is gone)
3. **Updates manga path** if the deleted folder was the primary path
   - Sets manga path to the remaining folder
   - Ensures manga still has valid path reference

### 6. User Feedback

- **Loading State**: Spinner while fetching folder information
- **Success Notification**: Green notification on successful deletion
- **Error Handling**: Red notification with error message if deletion fails
- **Row Removal**: Duplicate row is automatically removed from table after deletion

## Technical Details

### Files Modified:
1. **views/better.templ** - Added delete button and modal UI
2. **handlers/library_handler.go** - Added handler functions
3. **handlers/routes.go** - Added new API routes
4. **models/manga_duplicates.go** - Added folder info and deletion logic

### JavaScript Functions:
- `openDeleteModal(duplicateId)` - Opens modal and loads folder info
- `createFolderCard(folder, duplicateId, title)` - Generates folder card HTML
- `confirmDeleteFolder(folderPath, duplicateId)` - Shows confirmation dialog
- `deleteFolder(folderPath, duplicateId)` - Performs actual deletion via API

### Security Considerations:
- All endpoints require admin authentication
- Folder path validation ensures only valid duplicate folders can be deleted
- Server-side verification of folder ownership before deletion

## Usage Flow

1. Admin navigates to Better page (`/better`)
2. Sees list of duplicate manga folders
3. Clicks "Delete" button on a duplicate entry
4. Modal opens showing detailed comparison of both folders
5. Reviews information:
   - File counts
   - Last modified dates
   - Paths
6. Clicks "Delete This Folder" under the folder to remove
7. Browser confirmation dialog appears with folder name
8. Confirms deletion
9. Folder is deleted from disk
10. Database is updated
11. Row disappears from table
12. Success notification appears

## Benefits

- **Informed Decision**: See file count and last modified before deleting
- **Safety**: Double confirmation prevents accidental deletions
- **Cleanup**: Automatically removes duplicate entry after deletion
- **User Experience**: Smooth UI with loading states and notifications
- **Data Integrity**: Updates manga paths to prevent broken references

## Example Modal Display

```
┌─────────────────────────────────────────┐
│ Delete Duplicate Folder             [X] │
├─────────────────────────────────────────┤
│ ⚠️ Select which folder to delete.       │
│    This action cannot be undone!        │
│                                         │
│ ┌──────────────┐  ┌──────────────┐     │
│ │  Folder 1    │  │  Folder 2    │     │
│ │              │  │              │     │
│ │ my-manga     │  │ my-manga-v2  │     │
│ │ /path/to/... │  │ /path/to/... │     │
│ │              │  │              │     │
│ │ 📄 Files: 45 │  │ 📄 Files: 47 │     │
│ │ 🕐 Modified: │  │ 🕐 Modified: │     │
│ │ Oct 10, 2025 │  │ Oct 14, 2025 │     │
│ │              │  │              │     │
│ │ [Delete]     │  │ [Delete]     │     │
│ └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────┘
```
