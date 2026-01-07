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

type folderRec struct {
	id   int
	name string
}

// ParseBookmarksHTML parses an HTML bookmark file
func (p *Parser) ParseBookmarksHTML(r io.Reader) ([]models.Bookmark, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var bookmarks []models.Bookmark
	var folderStack []*folderRec

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Found folder header <H3 ...>
		if n.Type == html.ElementNode && n.Data == "h3" && n.FirstChild != nil {
			folderName := strings.TrimSpace(n.FirstChild.Data)
			var parentID *int
			if len(folderStack) > 0 {
				parentID = &folderStack[len(folderStack)-1].id
			}

			// Create or find folder in DB
			folder, err := p.folderService.Upsert(folderName, parentID)
			if err != nil {
				return // Skip on error
			}
			folderStack = append(folderStack, &folderRec{id: folder.ID, name: folder.Name})
		}

		// Found bookmark <A HREF=...>
		if n.Type == html.ElementNode && n.Data == "a" {
			var b models.Bookmark
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					b.URL = attr.Val
				}
			}
			if n.FirstChild != nil {
				b.Title = strings.TrimSpace(n.FirstChild.Data)
			}

			// Determine folder_id
			if len(folderStack) > 0 {
				b.FolderID = &folderStack[len(folderStack)-1].id
			}

			if b.URL != "" {
				bookmarks = append(bookmarks, b)
			}
		}

		// Recursively traverse children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		// When exiting DL container - "close" current folder
		if n.Type == html.ElementNode && n.Data == "dl" {
			if len(folderStack) > 0 {
				folderStack = folderStack[:len(folderStack)-1]
			}
		}
	}

	walk(doc)
	return bookmarks, nil
}
