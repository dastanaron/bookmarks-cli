package main

type Bookmark struct {
	ID          int
	Title       string
	URL         string
	Description string
	ParentID    *int
}
