package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	importPath := flag.String("import", "", "Path to HTML bookmarks file to import")
	flag.Parse()

	db := InitDB("bookmarks.db")
	defer db.Close()

	if *importPath != "" {
		file, err := os.Open(*importPath)
		if err != nil {
			log.Fatalf("Cannot open file: %v", err)
		}
		defer file.Close()

		bookmarks, err := ParseBookmarksHTML(file)
		if err != nil {
			log.Fatalf("Failed to parse HTML: %v", err)
		}

		for _, b := range bookmarks {
			_, err := db.Exec(
				"INSERT INTO bookmarks (title, url, description, parent_id) VALUES (?, ?, ?, ?)",
				b.Title, b.URL, b.Description, b.ParentID,
			)
			if err != nil {
				log.Printf("Insert error: %v", err)
			}
		}

		fmt.Printf("Imported %d bookmarks.\n", len(bookmarks))
		return
	}

	// Запуск UI
	app := NewApp(db)
	app.Run()
}
