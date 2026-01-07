package repository

import "github.com/dastanaron/bookmarks/internal/models"

// BookmarkRepository defines operations for bookmarks
type BookmarkRepository interface {
	List() ([]models.Bookmark, error)
	GetByID(id int) (*models.Bookmark, error)
	GetByURL(url string) (*models.Bookmark, error)
	Create(b *models.Bookmark) error
	Update(b *models.Bookmark) error
	Delete(id int) error
}

// FolderRepository defines operations for folders
type FolderRepository interface {
	List() ([]models.Folder, error)
	GetByID(id int) (*models.Folder, error)
	Create(name string, parentID *int) (*models.Folder, error)
	Update(f *models.Folder) error
	Delete(id int) error
	Upsert(name string, parentID *int) (*models.Folder, error)
}

// Repository combines all repositories
type Repository interface {
	Bookmarks() BookmarkRepository
	Folders() FolderRepository
	Close() error
}
