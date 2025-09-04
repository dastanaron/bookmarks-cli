package main

import (
	"database/sql"
	"io"
	"strings"

	"golang.org/x/net/html"
)

type folderRec struct {
	id   int
	name string
}

func ParseBookmarksHTML(r io.Reader, db *sql.DB) ([]Bookmark, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var out []Bookmark
	var folderStack []*folderRec

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// встретили заголовок папки <H3 ...>
		if n.Type == html.ElementNode && n.Data == "h3" && n.FirstChild != nil {
			folderName := strings.TrimSpace(n.FirstChild.Data)
			var parentID *int
			if len(folderStack) > 0 {
				parentID = &folderStack[len(folderStack)-1].id
			}
			// создаём или находим папку в БД
			folderID, err := UpsertFolder(db, folderName, parentID)
			if err != nil { // на ошибке просто пропускаем
				return
			}
			folderStack = append(folderStack, &folderRec{id: folderID, name: folderName})
			// идём дальше, но при выходе из DL будем убирать из стека
		}

		// встретили закладку <A HREF=...>
		if n.Type == html.ElementNode && n.Data == "a" {
			var b Bookmark
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					b.URL = attr.Val
				}
			}
			if n.FirstChild != nil {
				b.Title = strings.TrimSpace(n.FirstChild.Data)
			}
			// определяем folder_id
			if len(folderStack) > 0 {
				b.FolderID = &folderStack[len(folderStack)-1].id
			}
			out = append(out, b)
		}

		// рекурсивно обходим потомков
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		// когда выходим из контейнера DL — «закрываем» текущую папку
		if n.Type == html.ElementNode && n.Data == "dl" {
			if len(folderStack) > 0 {
				folderStack = folderStack[:len(folderStack)-1]
			}
		}
	}

	walk(doc)
	return out, nil
}
