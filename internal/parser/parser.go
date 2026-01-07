package parser

import (
	"io"
	"strings"

	"github.com/dastanaron/bookmarks/internal/models"
	"github.com/dastanaron/bookmarks/internal/service"

	"golang.org/x/net/html"
)

// Parser parses HTML bookmark files
type Parser struct {
	folderService *service.FolderService
}

// NewParser creates a new parser
func NewParser(folderService *service.FolderService) *Parser {
	return &Parser{folderService: folderService}
}

// folderRec represents a folder in the parsing context
type folderRec struct {
	id   int
	name string
}

// ParseBookmarksHTML parses an HTML bookmark file and returns all found bookmarks
func (p *Parser) ParseBookmarksHTML(r io.Reader) ([]models.Bookmark, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	bookmarks := make([]models.Bookmark, 0)
	folderStack := make([]*folderRec, 0)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Process element nodes
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h3":
				// Folder header: <H3>Folder Name</H3>
				p.processFolderNode(n, &folderStack)
			case "a":
				// Bookmark link: <A HREF="..." ICON="...">Title</A>
				if bookmark := p.processBookmarkNode(n, folderStack); bookmark != nil {
					bookmarks = append(bookmarks, *bookmark)
				}
			case "dl":
				// Closing folder container: </DL>
				p.processFolderClose(&folderStack)
			}
		}

		// Recursively traverse all children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(doc)
	return bookmarks, nil
}

// processFolderNode processes an <H3> node representing a folder
func (p *Parser) processFolderNode(n *html.Node, folderStack *[]*folderRec) {
	if n.FirstChild == nil {
		return
	}

	folderName := strings.TrimSpace(n.FirstChild.Data)
	if folderName == "" {
		return
	}

	// Determine parent folder ID
	var parentID *int
	if len(*folderStack) > 0 {
		parentID = &(*folderStack)[len(*folderStack)-1].id
	}

	// Create or find folder in database
	folder, err := p.folderService.Upsert(folderName, parentID)
	if err != nil {
		// Skip folder on error, but continue parsing
		return
	}

	// Add folder to stack
	*folderStack = append(*folderStack, &folderRec{
		id:   folder.ID,
		name: folder.Name,
	})
}

// processBookmarkNode processes an <A> node representing a bookmark
func (p *Parser) processBookmarkNode(n *html.Node, folderStack []*folderRec) *models.Bookmark {
	bookmark := &models.Bookmark{}

	// Extract attributes (href, icon)
	for _, attr := range n.Attr {
		switch attr.Key {
		case "href":
			bookmark.URL = attr.Val
		case "icon":
			iconVal := attr.Val
			bookmark.Icon = &iconVal
		}
	}

	// Extract title from text content
	if n.FirstChild != nil {
		bookmark.Title = strings.TrimSpace(n.FirstChild.Data)
	}

	// Skip bookmarks without URL
	if bookmark.URL == "" {
		return nil
	}

	// Assign folder ID from current folder stack
	if len(folderStack) > 0 {
		currentFolderID := folderStack[len(folderStack)-1].id
		bookmark.FolderID = &currentFolderID
	}

	return bookmark
}

// processFolderClose handles closing of a <DL> container (end of folder)
func (p *Parser) processFolderClose(folderStack *[]*folderRec) {
	if len(*folderStack) > 0 {
		*folderStack = (*folderStack)[:len(*folderStack)-1]
	}
}
