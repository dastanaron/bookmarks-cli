package main

type Folder struct {
	ID       int
	Name     string
	ParentID *int
}

type Bookmark struct {
	ID          int
	Title       string
	URL         string
	Description string
	FolderID    *int
	FolderName  *string
}
