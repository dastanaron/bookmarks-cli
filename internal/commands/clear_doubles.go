package commands

import (
	"fmt"

	"github.com/dastanaron/bookmarks/internal/repository"
	"github.com/dastanaron/bookmarks/internal/service"
)

// ClearDoublesCommand handles removal of duplicate bookmarks
type ClearDoublesCommand struct {
	repo        repository.Repository
	bookmarkSvc *service.BookmarkService
}

// NewClearDoublesCommand creates a new clear doubles command
func NewClearDoublesCommand(repo repository.Repository) *ClearDoublesCommand {
	bookmarkSvc := service.NewBookmarkService(repo)
	return &ClearDoublesCommand{
		repo:        repo,
		bookmarkSvc: bookmarkSvc,
	}
}

// Execute removes duplicate bookmarks (keeps the first one found, deletes others)
func (c *ClearDoublesCommand) Execute() error {
	// Get all bookmarks
	allBookmarks, err := c.bookmarkSvc.ListAll()
	if err != nil {
		return fmt.Errorf("failed to get bookmarks: %w", err)
	}

	// Track seen URLs and duplicates to delete
	seenURLs := make(map[string]int) // URL -> ID of bookmark to keep
	duplicatesToDelete := make([]int, 0)

	for _, bookmark := range allBookmarks {
		if bookmark.URL == "" {
			continue // Skip bookmarks without URL
		}

		// Check if we've seen this URL before
		if existingID, exists := seenURLs[bookmark.URL]; exists {
			// This is a duplicate - mark for deletion
			duplicatesToDelete = append(duplicatesToDelete, bookmark.ID)
			fmt.Printf("Found duplicate: '%s' (ID: %d, keeping ID: %d)\n", bookmark.Title, bookmark.ID, existingID)
		} else {
			// First occurrence of this URL - keep it
			seenURLs[bookmark.URL] = bookmark.ID
		}
	}

	if len(duplicatesToDelete) == 0 {
		fmt.Println("No duplicate bookmarks found.")
		return nil
	}

	// Delete duplicates
	deleted := 0
	for _, id := range duplicatesToDelete {
		if err := c.bookmarkSvc.Delete(id); err != nil {
			fmt.Printf("Warning: failed to delete bookmark ID %d: %v\n", id, err)
			continue
		}
		deleted++
	}

	fmt.Printf("Deleted %d duplicate bookmark(s).\n", deleted)
	return nil
}
