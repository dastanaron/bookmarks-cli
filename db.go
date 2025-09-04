package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		url TEXT,
		description TEXT,
		parent_id INTEGER,
		FOREIGN KEY(parent_id) REFERENCES bookmarks(id)
	);`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

// db.go (добавить в конец файла)

func ListBookmarks(db *sql.DB) ([]Bookmark, error) {
	rows, err := db.Query(`SELECT id, title, url, description FROM bookmarks WHERE url <> '' ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.Title, &b.URL, &b.Description); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func UpdateBookmark(db *sql.DB, b Bookmark) error {
	_, err := db.Exec(`UPDATE bookmarks SET title = ?, url = ?, description = ? WHERE id = ?`,
		b.Title, b.URL, b.Description, b.ID)
	return err
}

func StoreBookmark(db *sql.DB, b Bookmark) error {
	_, err := db.Exec(`INSERT INTO bookmarks(title, url, description, parent_id) VALUES (?, ?, ?, ?)`,
		b.Title, b.URL, b.Description, b.ParentID)
	return err
}

func DeleteBookmark(db *sql.DB, id int) error {
	_, err := db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}
