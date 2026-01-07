package models

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
	FolderID    *int
	FolderName  *string
}
