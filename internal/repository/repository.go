package repository

import "github.com/dastanaron/bookmarks/internal/models"

// BookmarkRepository defines operations for bookmarks
type BookmarkRepository interface {
	List() ([]models.Bookmark, error)
	GetByID(id int) (*models.Bookmark, error)
	GetByURL(url string) (*models.Bookmark, error)
	Create(b *models.Bookmark) error
	Update(b *models.Bookmark) error
	// Upsert creates a new bookmark if URL doesn't exist, otherwise updates the existing one.
	// Returns true if created, false if updated.
	Upsert(b *models.Bookmark) (bool, error)
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
