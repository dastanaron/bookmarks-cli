package commands

import (
	"fmt"
	"html"
	"os"
	"sort"

	"github.com/dastanaron/bookmarks/internal/models"
	"github.com/dastanaron/bookmarks/internal/repository"
	"github.com/dastanaron/bookmarks/internal/service"
)

// ExportCommand handles bookmark export to HTML file
type ExportCommand struct {
	repo        repository.Repository
	bookmarkSvc *service.BookmarkService
	folderSvc   *service.FolderService
}

// NewExportCommand creates a new export command
func NewExportCommand(repo repository.Repository) *ExportCommand {
	bookmarkSvc := service.NewBookmarkService(repo)
	folderSvc := service.NewFolderService(repo)
	return &ExportCommand{
		repo:        repo,
		bookmarkSvc: bookmarkSvc,
		folderSvc:   folderSvc,
	}
}

// Execute exports bookmarks to HTML file
func (c *ExportCommand) Execute(filePath string) error {
	// Get all folders and bookmarks
	folders, err := c.folderSvc.ListAll()
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}

	bookmarks, err := c.bookmarkSvc.ListAll()
	if err != nil {
		return fmt.Errorf("failed to get bookmarks: %w", err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer file.Close()

	// Write HTML header
	fmt.Fprintf(file, "<!DOCTYPE NETSCAPE-Bookmark-file-1>\n")
	fmt.Fprintf(file, "<META HTTP-EQUIV=\"Content-Type\" CONTENT=\"text/html; charset=UTF-8\">\n")
	fmt.Fprintf(file, "<TITLE>Bookmarks</TITLE>\n")
	fmt.Fprintf(file, "<H1>Bookmarks</H1>\n")
	fmt.Fprintf(file, "<DL><p>\n")

	// Build folder hierarchy
	folderMap := make(map[int]*models.Folder)
	for i := range folders {
		folderMap[folders[i].ID] = &folders[i]
	}

	// Group bookmarks by folder ID (use -1 for nil folder)
	bookmarksByFolder := make(map[int][]models.Bookmark)
	for i := range bookmarks {
		b := &bookmarks[i]
		var folderKey int = -1 // -1 represents nil folder
		if b.FolderID != nil {
			folderKey = *b.FolderID
		}
		bookmarksByFolder[folderKey] = append(bookmarksByFolder[folderKey], *b)
	}

	// Sort bookmarks in each folder by title
	for folderID := range bookmarksByFolder {
		sort.Slice(bookmarksByFolder[folderID], func(i, j int) bool {
			return bookmarksByFolder[folderID][i].Title < bookmarksByFolder[folderID][j].Title
		})
	}

	// Write root bookmarks (without folder)
	if rootBookmarks, ok := bookmarksByFolder[-1]; ok {
		for _, b := range rootBookmarks {
			c.writeBookmark(file, &b)
		}
	}

	// Write folders recursively
	rootFolders := c.getRootFolders(folders)
	for _, folder := range rootFolders {
		c.writeFolder(file, folder, folders, folderMap, bookmarksByFolder, 0)
	}

	// Write HTML footer
	fmt.Fprintf(file, "</DL><p>\n")

	fmt.Printf("Exported %d bookmarks to %s\n", len(bookmarks), filePath)
	return nil
}

// getRootFolders returns folders without parent or with parent_id = 0
func (c *ExportCommand) getRootFolders(folders []models.Folder) []models.Folder {
	var rootFolders []models.Folder
	for _, f := range folders {
		if f.ParentID == nil || *f.ParentID == 0 {
			rootFolders = append(rootFolders, f)
		}
	}
	// Sort by name
	sort.Slice(rootFolders, func(i, j int) bool {
		return rootFolders[i].Name < rootFolders[j].Name
	})
	return rootFolders
}

// writeFolder writes a folder and its contents recursively
func (c *ExportCommand) writeFolder(file *os.File, folder models.Folder, allFolders []models.Folder, folderMap map[int]*models.Folder, bookmarksByFolder map[int][]models.Bookmark, indent int) {
	// Write folder header
	fmt.Fprintf(file, "    <DT><H3>%s</H3>\n", html.EscapeString(folder.Name))
	fmt.Fprintf(file, "    <DL><p>\n")

	// Write bookmarks in this folder
	if folderBookmarks, ok := bookmarksByFolder[folder.ID]; ok {
		for _, b := range folderBookmarks {
			c.writeBookmark(file, &b)
		}
	}

	// Write child folders
	childFolders := c.getChildFolders(folder.ID, allFolders)
	for _, child := range childFolders {
		c.writeFolder(file, child, allFolders, folderMap, bookmarksByFolder, indent+1)
	}

	// Close folder
	fmt.Fprintf(file, "    </DL><p>\n")
}

// getChildFolders returns child folders of a given parent
func (c *ExportCommand) getChildFolders(parentID int, allFolders []models.Folder) []models.Folder {
	var children []models.Folder
	for _, f := range allFolders {
		if f.ParentID != nil && *f.ParentID == parentID {
			children = append(children, f)
		}
	}
	// Sort by name
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	return children
}

// writeBookmark writes a single bookmark
func (c *ExportCommand) writeBookmark(file *os.File, b *models.Bookmark) {
	escapedURL := html.EscapeString(b.URL)
	escapedTitle := html.EscapeString(b.Title)
	
	// Write bookmark with icon if available
	if b.Icon != nil && *b.Icon != "" {
		escapedIcon := html.EscapeString(*b.Icon)
		fmt.Fprintf(file, "    <DT><A HREF=\"%s\" ICON=\"%s\">%s</A>\n", escapedURL, escapedIcon, escapedTitle)
	} else {
		fmt.Fprintf(file, "    <DT><A HREF=\"%s\">%s</A>\n", escapedURL, escapedTitle)
	}
}
