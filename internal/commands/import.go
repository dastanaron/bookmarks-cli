package commands

import (
	"fmt"
	"os"

	"github.com/dastanaron/bookmarks/internal/parser"
	"github.com/dastanaron/bookmarks/internal/repository"
	"github.com/dastanaron/bookmarks/internal/service"
)

// ImportCommand handles bookmark import from HTML file
type ImportCommand struct {
	repo          repository.Repository
	bookmarkSvc   *service.BookmarkService
	parser        *parser.Parser
}

// NewImportCommand creates a new import command
func NewImportCommand(repo repository.Repository) *ImportCommand {
	folderSvc := service.NewFolderService(repo)
	bookmarkSvc := service.NewBookmarkService(repo)
	return &ImportCommand{
		repo:        repo,
		bookmarkSvc: bookmarkSvc,
		parser:      parser.NewParser(folderSvc),
	}
}

// Execute imports bookmarks from HTML file
func (c *ImportCommand) Execute(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	bookmarks, err := c.parser.ParseBookmarksHTML(file)
	if err != nil {
		return fmt.Errorf("failed to parse HTML: %w", err)
	}

	imported := 0
	for _, b := range bookmarks {
		if err := c.bookmarkSvc.Create(&b); err != nil {
			fmt.Printf("Warning: failed to import bookmark '%s': %v\n", b.Title, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d bookmarks.\n", imported)
	return nil
}
