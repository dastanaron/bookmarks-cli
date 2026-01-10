package service

import (
	"strings"

	"github.com/dastanaron/bookmarks/internal/models"
	"github.com/dastanaron/bookmarks/internal/repository"
)

// BookmarkService provides business logic for bookmarks
type BookmarkService struct {
	repo repository.Repository
}

// NewBookmarkService creates a new bookmark service
func NewBookmarkService(repo repository.Repository) *BookmarkService {
	return &BookmarkService{repo: repo}
}

// ListAll returns all bookmarks
func (s *BookmarkService) ListAll() ([]models.Bookmark, error) {
	return s.repo.Bookmarks().List()
}

// Search filters bookmarks by query string
func (s *BookmarkService) Search(query string) ([]models.Bookmark, error) {
	all, err := s.repo.Bookmarks().List()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return all, nil
	}

	queryLower := strings.ToLower(query)
	var filtered []models.Bookmark
	for _, b := range all {
		if strings.Contains(strings.ToLower(b.Title), queryLower) ||
			strings.Contains(strings.ToLower(b.URL), queryLower) ||
			strings.Contains(strings.ToLower(b.Description), queryLower) {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

// GetByFolderID returns bookmarks in a specific folder
func (s *BookmarkService) GetByFolderID(folderID *int) ([]models.Bookmark, error) {
	all, err := s.repo.Bookmarks().List()
	if err != nil {
		return nil, err
	}

	if folderID == nil {
		// Если folderID == nil, возвращаем все закладки
		return all, nil
	}

	// Фильтруем закладки по folderID
	var filtered []models.Bookmark
	targetID := *folderID
	for i := range all {
		b := &all[i]
		// Проверяем, что у закладки есть FolderID и он совпадает с искомым
		if b.FolderID != nil && *b.FolderID == targetID {
			filtered = append(filtered, *b)
		}
	}
	return filtered, nil
}

// SearchInFolder filters bookmarks by query string within a specific folder
func (s *BookmarkService) SearchInFolder(query string, folderID *int) ([]models.Bookmark, error) {
	var all []models.Bookmark
	var err error

	if folderID != nil {
		all, err = s.GetByFolderID(folderID)
	} else {
		all, err = s.ListAll()
	}
	if err != nil {
		return nil, err
	}

	if query == "" {
		return all, nil
	}

	queryLower := strings.ToLower(query)
	var filtered []models.Bookmark
	for _, b := range all {
		if strings.Contains(strings.ToLower(b.Title), queryLower) ||
			strings.Contains(strings.ToLower(b.URL), queryLower) ||
			strings.Contains(strings.ToLower(b.Description), queryLower) {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

// GetByID returns a bookmark by ID
func (s *BookmarkService) GetByID(id int) (*models.Bookmark, error) {
	return s.repo.Bookmarks().GetByID(id)
}

// GetByURL returns a bookmark by URL
func (s *BookmarkService) GetByURL(url string) (*models.Bookmark, error) {
	return s.repo.Bookmarks().GetByURL(url)
}

// Create creates a new bookmark
func (s *BookmarkService) Create(b *models.Bookmark) error {
	return s.repo.Bookmarks().Create(b)
}

// Update updates an existing bookmark
func (s *BookmarkService) Update(b *models.Bookmark) error {
	return s.repo.Bookmarks().Update(b)
}

// Upsert creates a new bookmark if URL doesn't exist, otherwise updates the existing one.
// Returns true if created, false if updated.
func (s *BookmarkService) Upsert(b *models.Bookmark) (bool, error) {
	return s.repo.Bookmarks().Upsert(b)
}

// Delete deletes a bookmark by ID
func (s *BookmarkService) Delete(id int) error {
	return s.repo.Bookmarks().Delete(id)
}

// FolderService provides business logic for folders
type FolderService struct {
	repo repository.Repository
}

// NewFolderService creates a new folder service
func NewFolderService(repo repository.Repository) *FolderService {
	return &FolderService{repo: repo}
}

// ListAll returns all folders
func (s *FolderService) ListAll() ([]models.Folder, error) {
	return s.repo.Folders().List()
}

// GetByID returns a folder by ID
func (s *FolderService) GetByID(id int) (*models.Folder, error) {
	return s.repo.Folders().GetByID(id)
}

// Create creates a new folder
func (s *FolderService) Create(name string, parentID *int) (*models.Folder, error) {
	return s.repo.Folders().Create(name, parentID)
}

// Upsert creates or returns existing folder
func (s *FolderService) Upsert(name string, parentID *int) (*models.Folder, error) {
	return s.repo.Folders().Upsert(name, parentID)
}

// Update updates an existing folder
func (s *FolderService) Update(f *models.Folder) error {
	return s.repo.Folders().Update(f)
}

// Delete deletes a folder by ID
func (s *FolderService) Delete(id int) error {
	return s.repo.Folders().Delete(id)
}

// GetFolderContent returns all items (bookmarks and subfolders) in a folder
// If folderID is nil, returns all root items (bookmarks without folder and root folders)
func (s *FolderService) GetFolderContent(folderID *int) ([]models.Item, error) {
	return s.repo.Folders().GetFolderContent(folderID)
}
