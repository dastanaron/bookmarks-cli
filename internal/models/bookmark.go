package models

// ItemType represents the type of item (bookmark or folder)
type ItemType string

const (
	ItemTypeBookmark ItemType = "bookmark"
	ItemTypeFolder   ItemType = "folder"
)

// Folder represents a bookmark folder
type Folder struct {
	ID       int
	Name     string
	ParentID *int
}

// Bookmark represents a bookmark entry
type Bookmark struct {
	ID          int
	Title       string
	URL         string
	Description string
	Icon        *string // Base64-encoded icon image (nullable)
	FolderID    *int
	FolderName  *string
}

// Item represents a unified item that can be either a bookmark or a folder
// Used for displaying folder contents with both bookmarks and subfolders
type Item struct {
	Type        ItemType // "bookmark" or "folder"
	ID          int
	Name        string  // Title for bookmarks, Name for folders
	URL         *string // Only for bookmarks, nil for folders
	Description *string // Only for bookmarks, nil for folders
	Icon        *string // Only for bookmarks, nil for folders
	ParentID    *int    // folder_id for bookmarks, parent_id for folders
}
