package main

import (
	"io"

	"golang.org/x/net/html"
)

func ParseBookmarksHTML(r io.Reader) ([]Bookmark, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var bookmarks []Bookmark
	var walk func(*html.Node, *int)
	walk = func(n *html.Node, parentID *int) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var b Bookmark
			for _, attr := range n.Attr {
				switch attr.Key {
				case "href":
					b.URL = attr.Val
				}
			}
			if n.FirstChild != nil {
				b.Title = n.FirstChild.Data
			}
			b.ParentID = parentID
			bookmarks = append(bookmarks, b)
		}

		if n.Type == html.ElementNode && n.Data == "h3" {
			folder := Bookmark{Title: n.FirstChild.Data, URL: ""}
			folder.ParentID = parentID
			bookmarks = append(bookmarks, folder)
			newParentID := len(bookmarks)
			for c := n.NextSibling; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "dl" {
					walk(c, &newParentID)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, parentID)
		}
	}

	walk(doc, nil)
	return bookmarks, nil
}
