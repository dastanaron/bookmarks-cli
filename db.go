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

	createTables := `
  CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		parent_id INTEGER,
		FOREIGN KEY(parent_id) REFERENCES folders(id)
	);

  CREATE TABLE IF NOT EXISTS "bookmarks" (
    "id"	INTEGER,
    "title"	TEXT,
    "url"	TEXT,
    "description"	TEXT,
    "folder_id"	INTEGER,
    FOREIGN KEY("folder_id") REFERENCES "bookmarks"("id"),
    PRIMARY KEY("id" AUTOINCREMENT)
  );
  `
	_, err = db.Exec(createTables)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func ListBookmarks(db *sql.DB) ([]Bookmark, error) {
	rows, err := db.Query(`
    SELECT b.id, b.title, b.url, b.description, b.folder_id, f.name FROM bookmarks as b
    left join folders as f on f.id = b.folder_id 
    WHERE b.url <> '' ORDER BY title
  `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.Title, &b.URL, &b.Description, &b.FolderID, &b.FolderName); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func ListFolders(db *sql.DB) ([]Folder, error) {
	rows, err := db.Query(`
		SELECT id, name, parent_id FROM folders
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.ParentID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func UpdateBookmark(db *sql.DB, b Bookmark) error {
	_, err := db.Exec(`UPDATE bookmarks SET title = ?, url = ?, description = ? WHERE id = ?`,
		b.Title, b.URL, b.Description, b.ID)
	return err
}

func StoreBookmark(db *sql.DB, b Bookmark) error {
	_, err := db.Exec(`INSERT INTO bookmarks(title, url, description, folder_id) VALUES (?, ?, ?, ?)`,
		b.Title, b.URL, b.Description, b.FolderID)
	return err
}

func DeleteBookmark(db *sql.DB, id int) error {
	_, err := db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}

func UpsertFolder(db *sql.DB, name string, parentID *int) (int, error) {
	var id int
	err := db.QueryRow(
		"SELECT id FROM folders WHERE name = ? AND (parent_id IS ? OR parent_id = ?)",
		name, parentID, parentID).Scan(&id)
	if err == nil {
		return id, nil
	}

	res, err := db.Exec("INSERT INTO folders(name, parent_id) VALUES (?, ?)", name, parentID)
	if err != nil {
		return 0, err
	}
	newID, err := res.LastInsertId()
	return int(newID), err
}
