package repository

import (
	"database/sql"

	"github.com/dastanaron/bookmarks/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteRepository implements Repository using SQLite
type SQLiteRepository struct {
	db        *sql.DB
	bookmarks *bookmarkRepo
	folders   *folderRepo
}

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	repo := &SQLiteRepository{
		db: db,
	}
	repo.bookmarks = &bookmarkRepo{db: db}
	repo.folders = &folderRepo{db: db}

	return repo, nil
}

func initSchema(db *sql.DB) error {
	createTables := `
	CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		parent_id INTEGER,
		FOREIGN KEY(parent_id) REFERENCES folders(id)
	);

	CREATE TABLE IF NOT EXISTS bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		description TEXT,
		icon TEXT,
		folder_id INTEGER,
		FOREIGN KEY(folder_id) REFERENCES folders(id)
	);

	CREATE INDEX IF NOT EXISTS idx_bookmarks_folder ON bookmarks(folder_id);
	CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders(parent_id);
	`
	if _, err := db.Exec(createTables); err != nil {
		return err
	}

	// Migration: add icon column if it doesn't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN,
	// so we check if the column exists first
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('bookmarks') WHERE name = 'icon'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.Exec(`ALTER TABLE bookmarks ADD COLUMN icon TEXT`)
		if err != nil {
			return err
		}
	}

	return nil
}

// Bookmarks returns the bookmark repository
func (r *SQLiteRepository) Bookmarks() BookmarkRepository {
	return r.bookmarks
}

// Folders returns the folder repository
func (r *SQLiteRepository) Folders() FolderRepository {
	return r.folders
}

// Close closes the database connection
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// bookmarkRepo implements BookmarkRepository
type bookmarkRepo struct {
	db *sql.DB
}

func (r *bookmarkRepo) List() ([]models.Bookmark, error) {
	rows, err := r.db.Query(`
		SELECT b.id, b.title, b.url, b.description, b.icon, b.folder_id, f.name 
		FROM bookmarks AS b
		LEFT JOIN folders AS f ON f.id = b.folder_id 
		WHERE b.url <> '' 
		ORDER BY b.title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []models.Bookmark
	for rows.Next() {
		var b models.Bookmark
		if err := rows.Scan(&b.ID, &b.Title, &b.URL, &b.Description, &b.Icon, &b.FolderID, &b.FolderName); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, rows.Err()
}

func (r *bookmarkRepo) GetByID(id int) (*models.Bookmark, error) {
	var b models.Bookmark
	err := r.db.QueryRow(`
		SELECT b.id, b.title, b.url, b.description, b.icon, b.folder_id, f.name 
		FROM bookmarks AS b
		LEFT JOIN folders AS f ON f.id = b.folder_id 
		WHERE b.id = ?
	`, id).Scan(&b.ID, &b.Title, &b.URL, &b.Description, &b.Icon, &b.FolderID, &b.FolderName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *bookmarkRepo) GetByURL(url string) (*models.Bookmark, error) {
	var b models.Bookmark
	err := r.db.QueryRow(`
		SELECT b.id, b.title, b.url, b.description, b.icon, b.folder_id, f.name 
		FROM bookmarks AS b
		LEFT JOIN folders AS f ON f.id = b.folder_id 
		WHERE b.url = ?
	`, url).Scan(&b.ID, &b.Title, &b.URL, &b.Description, &b.Icon, &b.FolderID, &b.FolderName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *bookmarkRepo) Create(b *models.Bookmark) error {
	res, err := r.db.Exec(
		`INSERT INTO bookmarks(title, url, description, icon, folder_id) VALUES (?, ?, ?, ?, ?)`,
		b.Title, b.URL, b.Description, b.Icon, b.FolderID,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = int(id)
	return nil
}

func (r *bookmarkRepo) Update(b *models.Bookmark) error {
	_, err := r.db.Exec(
		`UPDATE bookmarks SET title = ?, url = ?, description = ?, icon = ?, folder_id = ? WHERE id = ?`,
		b.Title, b.URL, b.Description, b.Icon, b.FolderID, b.ID,
	)
	return err
}

func (r *bookmarkRepo) Upsert(b *models.Bookmark) (bool, error) {
	var id int
	err := r.db.QueryRow(`SELECT id FROM bookmarks WHERE url = ?`, b.URL).Scan(&id)
	switch err {
	case nil:
		b.ID = id
		return false, r.Update(b)
	case sql.ErrNoRows:
		return true, r.Create(b)
	default:
		return false, err
	}
}

func (r *bookmarkRepo) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}

// folderRepo implements FolderRepository
type folderRepo struct {
	db *sql.DB
}

func (r *folderRepo) List() ([]models.Folder, error) {
	rows, err := r.db.Query(`SELECT id, name, parent_id FROM folders ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []models.Folder
	for rows.Next() {
		var f models.Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.ParentID); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (r *folderRepo) GetByID(id int) (*models.Folder, error) {
	var f models.Folder
	err := r.db.QueryRow(`SELECT id, name, parent_id FROM folders WHERE id = ?`, id).
		Scan(&f.ID, &f.Name, &f.ParentID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *folderRepo) Create(name string, parentID *int) (*models.Folder, error) {
	res, err := r.db.Exec(`INSERT INTO folders(name, parent_id) VALUES (?, ?)`, name, parentID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &models.Folder{ID: int(id), Name: name, ParentID: parentID}, nil
}

func (r *folderRepo) Update(f *models.Folder) error {
	_, err := r.db.Exec(`UPDATE folders SET name = ?, parent_id = ? WHERE id = ?`,
		f.Name, f.ParentID, f.ID)
	return err
}

func (r *folderRepo) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM folders WHERE id = ?`, id)
	return err
}

func (r *folderRepo) Upsert(name string, parentID *int) (*models.Folder, error) {
	var id int
	var err error

	// SQL для проверки NULL требует разных запросов
	if parentID == nil {
		err = r.db.QueryRow(
			`SELECT id FROM folders WHERE name = ? AND parent_id IS NULL`,
			name,
		).Scan(&id)
	} else {
		err = r.db.QueryRow(
			`SELECT id FROM folders WHERE name = ? AND parent_id = ?`,
			name, *parentID,
		).Scan(&id)
	}

	if err == nil {
		return &models.Folder{ID: id, Name: name, ParentID: parentID}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new folder
	return r.Create(name, parentID)
}

func (r *folderRepo) GetFolderContent(folderID *int) ([]models.Item, error) {
	var query string
	var args []interface{}

	if folderID == nil {
		// Получаем корневые элементы: закладки без папки и папки без родителя
		query = `
			SELECT 
				'bookmark' as type,
				b.id,
				b.title as name,
				b.url,
				b.description,
				b.icon,
				b.folder_id as parent_id
			FROM bookmarks AS b
			WHERE b.folder_id IS NULL
			UNION ALL
			SELECT 
				'folder' as type,
				f.id,
				f.name,
				NULL as url,
				NULL as description,
				NULL as icon,
				f.parent_id as parent_id
			FROM folders AS f
			WHERE f.parent_id IS NULL OR f.parent_id = 0
			ORDER BY type, name
		`
		args = []interface{}{}
	} else {
		// Получаем содержимое конкретной папки
		query = `
			SELECT 
				'bookmark' as type,
				b.id,
				b.title as name,
				b.url,
				b.description,
				b.icon,
				b.folder_id as parent_id
			FROM bookmarks AS b
			WHERE b.folder_id = ?
			UNION ALL
			SELECT 
				'folder' as type,
				f.id,
				f.name,
				NULL as url,
				NULL as description,
				NULL as icon,
				f.parent_id as parent_id
			FROM folders AS f
			WHERE f.parent_id = ?
			ORDER BY type, name
		`
		args = []interface{}{*folderID, *folderID}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var item models.Item
		var typeStr string
		var url, description, icon sql.NullString
		var parentID sql.NullInt64

		err := rows.Scan(
			&typeStr,
			&item.ID,
			&item.Name,
			&url,
			&description,
			&icon,
			&parentID,
		)
		if err != nil {
			return nil, err
		}

		item.Type = models.ItemType(typeStr)
		if url.Valid {
			item.URL = &url.String
		}
		if description.Valid {
			item.Description = &description.String
		}
		if icon.Valid {
			item.Icon = &icon.String
		}
		if parentID.Valid {
			pid := int(parentID.Int64)
			item.ParentID = &pid
		}

		items = append(items, item)
	}

	return items, rows.Err()
}
