package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/dastanaron/bookmarks/internal/models"
	"github.com/dastanaron/bookmarks/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	ModeNormal = 1
	ModeSearch = 2
	ModeForm   = 3
	ModeModal  = 4
)

// folderItem represents a folder item in the list
type folderItem struct {
	ID    *int // nil for "All Bookmarks"
	Name  string
	Level int // nesting level (0 = root)
}

// App represents the TUI application
type App struct {
	app            *tview.Application
	folderList     *tview.List // list of folders instead of tree
	list           *tview.List // list of items (bookmarks and folders)
	detail         *tview.TextView
	search         *tview.InputField
	pages          *tview.Pages
	mode           uint8
	allItems       []models.Item    // all items in current folder (unfiltered)
	items          []models.Item    // filtered items for display
	currentItem    *models.Item     // currently selected item (bookmark or folder)
	current        *models.Bookmark // for compatibility (used in showDetails)
	status         *tview.TextView
	bookmarkSvc    *service.BookmarkService
	folderSvc      *service.FolderService
	selectedFolder *int         // ID of selected folder, nil = root folder
	focusOnFolders bool         // true = focus on folder list, false = on item list
	folderItems    []folderItem // list of folders for quick access
}

// NewApp creates a new application instance
func NewApp(bookmarkSvc *service.BookmarkService, folderSvc *service.FolderService) *App {
	return &App{
		app:            tview.NewApplication(),
		folderList:     tview.NewList(),
		list:           tview.NewList(),
		detail:         tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		search:         tview.NewInputField().SetLabel("Search: "),
		pages:          tview.NewPages(),
		mode:           ModeNormal,
		status:         tview.NewTextView().SetDynamicColors(true),
		bookmarkSvc:    bookmarkSvc,
		folderSvc:      folderSvc,
		selectedFolder: nil, // By default show all bookmarks
		focusOnFolders: false,
		folderItems:    []folderItem{},
	}
}

// Run starts the application
func (a *App) Run() error {
	a.list.SetBorder(true).SetTitle("Items")
	a.detail.SetBorder(true).SetTitle("Details")
	a.folderList.SetBorder(true).SetTitle("Folders")

	cols := tview.NewFlex().
		AddItem(a.folderList, 0, 1, false).
		AddItem(a.list, 0, 3, true).
		AddItem(a.detail, 0, 1, false)

	main := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.search, 1, 0, false).
		AddItem(cols, 0, 1, true).
		AddItem(a.status, 1, 0, false)

	a.pages.AddPage("main", main, true, true)

	if err := a.fillFolderList(); err != nil {
		return err
	}

	if err := a.loadFolderContent(); err != nil {
		return err
	}

	a.search.SetChangedFunc(a.onSearchChange)
	a.search.SetDoneFunc(a.onSearchDone)
	a.list.SetChangedFunc(a.onSelect)

	// SetSelectedFunc is not used - selection is handled via Enter in globalInput
	// This avoids accidental selection when navigating with arrows

	a.app.SetRoot(a.pages, true)
	a.app.SetInputCapture(a.globalInput)
	a.updateStatus()

	// Initial focus on bookmark list
	a.focusOnFolders = false
	a.app.SetFocus(a.list)
	return a.app.Run()
}

func (a *App) updateStatus() {
	// Get total count of items (bookmarks and folders)
	itemCount := len(a.items)
	var bookmarkCount, folderCount int
	for _, item := range a.items {
		if item.Type == models.ItemTypeBookmark {
			bookmarkCount++
		} else {
			folderCount++
		}
	}

	// Build status text with counts
	var countText string
	if itemCount > 0 {
		if folderCount > 0 {
			countText = fmt.Sprintf(" [::b]%d[::r] items (%d bookmarks, %d folders)", itemCount, bookmarkCount, folderCount)
		} else {
			countText = fmt.Sprintf(" [::b]%d[::r] items", itemCount)
		}
	} else {
		countText = " [::b]0[::r] items"
	}

	statusText := "[::b]Tab[::r] switch  [::b]/[::r] search  [::b]a[::r] add  [::b]e[::r] edit  [::b]d[::r] del  [::b]Enter[::r] open/select  [::b]q[::r] quit" + countText
	if a.focusOnFolders {
		statusText = "[::b]Tab[::r] switch  [::b]Enter[::r] select  [::b]a[::r] add folder  [::b]e[::r] edit folder  [::b]d[::r] del folder  [::b]q[::r] quit" + countText
	}
	a.status.SetText(statusText)
}

func (a *App) reloadBookmarks() error {
	// Reload contents of current folder
	return a.loadFolderContent()
}

func (a *App) loadFolderContent() error {
	var err error
	// Get contents of selected folder (bookmarks and subfolders)
	a.allItems, err = a.folderSvc.GetFolderContent(a.selectedFolder)
	if err != nil {
		// On error show empty list
		a.allItems = []models.Item{}
		a.items = []models.Item{}
		a.fillList()
		return err
	}

	// Apply search filter if present
	if a.search.GetText() != "" {
		a.applyFilter(a.search.GetText())
	} else {
		// Without filter show all items
		a.items = a.allItems
		a.fillList()
	}

	// Update list title
	if a.selectedFolder == nil {
		a.list.SetTitle("Items (Root)")
	} else {
		folder, err := a.folderSvc.GetByID(*a.selectedFolder)
		if err == nil && folder != nil {
			a.list.SetTitle(fmt.Sprintf("Items (%s)", folder.Name))
		} else {
			a.list.SetTitle("Items")
		}
	}

	return nil
}

func (a *App) applyFilter(text string) {
	// If no search query, show all items in current folder
	if text == "" {
		a.items = a.allItems
		a.fillList()
		return
	}

	// If folder not selected, search all bookmarks via SearchInFolder
	if a.selectedFolder == nil {
		bookmarks, err := a.bookmarkSvc.SearchInFolder(text, nil)
		if err != nil {
			a.items = []models.Item{}
			a.fillList()
			return
		}

		// Convert bookmarks to Items
		var items []models.Item
		for _, b := range bookmarks {
			item := models.Item{
				Type:        models.ItemTypeBookmark,
				ID:          b.ID,
				Name:        b.Title,
				URL:         &b.URL,
				Description: &b.Description,
				Icon:        b.Icon,
				ParentID:    b.FolderID,
			}
			items = append(items, item)
		}
		a.items = items
		a.fillList()
		return
	}

	// If folder selected, filter items within that folder
	textLower := strings.ToLower(text)
	var filtered []models.Item
	for _, item := range a.allItems {
		itemNameLower := strings.ToLower(item.Name)

		// Check name
		if strings.Contains(itemNameLower, textLower) {
			filtered = append(filtered, item)
			continue
		}

		// For bookmarks also check URL and description
		if item.Type == models.ItemTypeBookmark {
			if item.URL != nil {
				urlLower := strings.ToLower(*item.URL)
				if strings.Contains(urlLower, textLower) {
					filtered = append(filtered, item)
					continue
				}
			}
			if item.Description != nil {
				descLower := strings.ToLower(*item.Description)
				if strings.Contains(descLower, textLower) {
					filtered = append(filtered, item)
					continue
				}
			}
		}
	}
	a.items = filtered
	a.fillList()
}

// onFolderSelect handles folder selection in the list (called when Enter is pressed)
func (a *App) onFolderSelect(item folderItem) {
	// Set selected folder
	// Important: create a new variable for ID to avoid pointer issues
	var newSelectedFolder *int
	if item.ID != nil {
		folderID := *item.ID
		newSelectedFolder = &folderID
	} else {
		newSelectedFolder = nil
	}
	a.selectedFolder = newSelectedFolder

	// Update folder list title
	if item.ID == nil {
		a.folderList.SetTitle("Folders (All)")
	} else {
		a.folderList.SetTitle(fmt.Sprintf("Folders (%s)", item.Name))
	}

	// Load contents of selected folder
	if err := a.loadFolderContent(); err != nil {
		// On error show empty list
		a.allItems = []models.Item{}
		a.items = []models.Item{}
		a.fillList()
	}

	// Update status bar
	a.updateStatus()

	// Switch focus to item list for convenience
	a.focusOnFolders = false
	a.app.SetFocus(a.list)

	// Do NOT call app.Draw() here - this may cause freezing
	// UI will update automatically on the next event processing cycle
}

func (a *App) fillList() {
	a.list.Clear()
	for i := range a.items {
		index := i
		item := a.items[i]

		// Build display name and secondary text
		var mainText, secondaryText string
		if item.Type == models.ItemTypeFolder {
			// For folders show folder icon
			mainText = fmt.Sprintf("📁 %s", item.Name)
			secondaryText = "Folder"
		} else {
			// For bookmarks show name and URL
			mainText = item.Name
			if item.URL != nil {
				secondaryText = *item.URL
			}
		}

		a.list.AddItem(mainText, secondaryText, 0, func() {
			if index >= 0 && index < len(a.items) {
				a.currentItem = &a.items[index]
				// For compatibility create bookmark if it's a bookmark
				if a.items[index].Type == models.ItemTypeBookmark {
					a.convertItemToBookmark(&a.items[index])
				} else {
					a.current = nil
				}
				a.showDetails()
			}
		})
	}

	// Set current item if present
	if len(a.items) > 0 {
		a.currentItem = &a.items[0]
		if a.items[0].Type == models.ItemTypeBookmark {
			a.convertItemToBookmark(&a.items[0])
		} else {
			a.current = nil
		}
		a.showDetails()
	} else {
		a.currentItem = nil
		a.current = nil
		a.showDetails()
	}
}

// convertItemToBookmark converts Item to Bookmark for compatibility
func (a *App) convertItemToBookmark(item *models.Item) {
	if item.Type != models.ItemTypeBookmark {
		return
	}

	var folderName *string
	if item.ParentID != nil {
		// Get folder name by ID
		folder, err := a.folderSvc.GetByID(*item.ParentID)
		if err == nil && folder != nil {
			folderName = &folder.Name
		}
	}

	bookmark := models.Bookmark{
		ID:          item.ID,
		Title:       item.Name,
		URL:         "",
		Description: "",
		FolderID:    item.ParentID,
		FolderName:  folderName,
	}

	if item.URL != nil {
		bookmark.URL = *item.URL
	}
	if item.Description != nil {
		bookmark.Description = *item.Description
	}
	if item.Icon != nil {
		bookmark.Icon = item.Icon
	}

	a.current = &bookmark
}

// reloadFolders reloads the folder list
func (a *App) reloadFolders() error {
	if err := a.fillFolderList(); err != nil {
		return err
	}
	return nil
}

// fillFolderList fills the folder list with indentation to show hierarchy
func (a *App) fillFolderList() error {
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		return err
	}

	// Create folder map for quick access
	folderMap := make(map[int]*models.Folder)
	for i := range folders {
		folderMap[folders[i].ID] = &folders[i]
	}

	a.folderList.Clear()
	a.folderItems = []folderItem{}

	// Add root element "All Bookmarks"
	allItem := folderItem{ID: nil, Name: "All Bookmarks", Level: 0}
	a.folderItems = append(a.folderItems, allItem)
	a.folderList.AddItem(allItem.Name, "", 0, nil)

	// Recursive function to add folders with proper hierarchy
	var buildList func(parentID *int, level int)
	buildList = func(parentID *int, level int) {
		for _, folder := range folders {
			// Check if this folder is a child of current parent
			var isChild bool
			if parentID == nil {
				// Look for folders without parent or with parentID = 0
				isChild = folder.ParentID == nil || *folder.ParentID == 0
			} else {
				// Look for folders with specified parent
				isChild = folder.ParentID != nil && *folder.ParentID == *parentID
			}

			if isChild {
				// Create indent based on level
				indent := ""
				for i := 0; i < level; i++ {
					indent += "  " // 2 spaces per level
				}
				if level > 0 {
					indent += "└─ " // symbol to show nesting
				}

				// Important: create a copy of ID to avoid pointer issues
				folderID := folder.ID
				item := folderItem{
					ID:    &folderID,
					Name:  folder.Name,
					Level: level,
				}
				a.folderItems = append(a.folderItems, item)
				a.folderList.AddItem(indent+folder.Name, "", 0, nil)

				// Recursively add child folders
				childParentID := &folder.ID
				buildList(childParentID, level+1)
			}
		}
	}

	// Start from root level (parentID = nil)
	buildList(nil, 1)

	a.folderList.SetTitle("Folders (All)")
	return nil
}

func (a *App) showDetails() {
	if a.currentItem == nil {
		a.detail.SetText("")
		return
	}

	item := a.currentItem
	var text string

	if item.Type == models.ItemTypeFolder {
		// Show folder information
		parentName := "Root"
		if item.ParentID != nil {
			folder, err := a.folderSvc.GetByID(*item.ParentID)
			if err == nil && folder != nil {
				parentName = folder.Name
			}
		}
		text = fmt.Sprintf(
			"[::b]Type:[::-]\nFolder\n\n[::b]Name:[::-]\n%s\n\n[::b]Parent:[::-]\n%s",
			item.Name, parentName)
	} else {
		// Show bookmark information
		b := a.current
		if b == nil {
			// If current is not set, use data from item
			url := ""
			if item.URL != nil {
				url = *item.URL
			}
			desc := ""
			if item.Description != nil {
				desc = *item.Description
			}
			folderName := "/"
			if item.ParentID != nil {
				folder, err := a.folderSvc.GetByID(*item.ParentID)
				if err == nil && folder != nil {
					folderName = folder.Name
				}
			}
			text = fmt.Sprintf(
				"[::b]Type:[::-]\nBookmark\n\n[::b]Title:[::-]\n%s\n\n[::b]URL:[::-]\n%s\n\n[::b]Description:[::-]\n%s\n\n[::b]Folder:[::-]\n%s",
				item.Name, url, desc, folderName)
		} else {
			folderName := "/"
			if b.FolderName != nil {
				folderName = *b.FolderName
			}
			text = fmt.Sprintf(
				"[::b]Type:[::-]\nBookmark\n\n[::b]Title:[::-]\n%s\n\n[::b]URL:[::-]\n%s\n\n[::b]Description:[::-]\n%s\n\n[::b]Folder:[::-]\n%s",
				b.Title, b.URL, b.Description, folderName)
		}
	}

	a.detail.SetText(text)
}

func (a *App) setMode(m uint8) {
	a.mode = m
	switch m {
	case ModeSearch:
		a.app.SetFocus(a.search)
	case ModeNormal:
		if a.focusOnFolders {
			a.app.SetFocus(a.folderList)
		} else {
			a.app.SetFocus(a.list)
		}
	}
}

// toggleFocus switches focus between folder list and bookmark list
func (a *App) toggleFocus() {
	a.focusOnFolders = !a.focusOnFolders
	if a.focusOnFolders {
		a.app.SetFocus(a.folderList)
		// Update title with selected folder information
		if a.selectedFolder != nil {
			// Find selected folder name
			for _, item := range a.folderItems {
				if item.ID != nil && *item.ID == *a.selectedFolder {
					a.folderList.SetTitle(fmt.Sprintf("Folders (%s)", item.Name))
					break
				}
			}
		} else {
			a.folderList.SetTitle("Folders (All)")
		}
	} else {
		a.app.SetFocus(a.list)
		if a.selectedFolder != nil {
			// Update folder list title
			for _, item := range a.folderItems {
				if item.ID != nil && *item.ID == *a.selectedFolder {
					a.folderList.SetTitle(fmt.Sprintf("Folders (%s)", item.Name))
					break
				}
			}
		} else {
			a.folderList.SetTitle("Folders (All)")
		}
	}
	// Update status bar
	a.updateStatus()
}

func (a *App) onSearchChange(text string) {
	a.applyFilter(text)
}

func (a *App) onSearchDone(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		a.setMode(ModeNormal)
	case tcell.KeyEscape:
		a.search.SetText("")
		// When clearing search reload folder contents
		if err := a.loadFolderContent(); err != nil {
			a.allItems = []models.Item{}
			a.items = []models.Item{}
			a.fillList()
		}
		a.setMode(ModeNormal)
	}
}

func (a *App) onSelect(index int, mainText, secondaryText string, shortcut rune) {
	if index >= 0 && index < len(a.items) {
		a.currentItem = &a.items[index]
		if a.items[index].Type == models.ItemTypeBookmark {
			a.convertItemToBookmark(&a.items[index])
		} else {
			a.current = nil
		}
		a.showDetails()
	}
}

func (a *App) globalInput(event *tcell.EventKey) *tcell.EventKey {
	// Check if modal window is open before checking mode
	if a.pages.HasPage("confirm") || a.pages.HasPage("error") {
		// In modal window Tab switches between buttons, Enter selects
		// Don't handle these events here so modal window can handle them
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEnter, tcell.KeyLeft, tcell.KeyRight, tcell.KeyEscape:
			// Pass these events to modal window
			return event
		default:
			// For other events also pass them to modal window
			return event
		}
	}

	switch a.mode {
	case ModeNormal:
		// Tab to switch between tree and list
		if event.Key() == tcell.KeyTab {
			a.toggleFocus()
			return nil
		}

		// If focus on folder list, handle hotkeys
		if a.focusOnFolders {
			switch event.Key() {
			case tcell.KeyEnter:
				// Enter - select folder
				// Get current index from folder list
				currentIndex := a.folderList.GetCurrentItem()
				if currentIndex >= 0 && currentIndex < len(a.folderItems) {
					item := a.folderItems[currentIndex]
					// Call folder selection handler
					a.onFolderSelect(item)
					// Return nil so event is not passed further
					return nil
				}
				return nil
			case tcell.KeyRune:
				switch event.Rune() {
				case 'q':
					// Exit application
					a.app.Stop()
					return nil
				case '/':
					// Search
					a.setMode(ModeSearch)
					return nil
				case 'a':
					// Add new folder
					a.showFolderForm(&models.Folder{}, false)
					return nil
				case 'e':
					// Edit current folder
					currentIndex := a.folderList.GetCurrentItem()
					if currentIndex >= 0 && currentIndex < len(a.folderItems) {
						item := a.folderItems[currentIndex]
						if item.ID != nil {
							// Get folder from database
							folder, err := a.folderSvc.GetByID(*item.ID)
							if err == nil && folder != nil {
								f := *folder
								a.showFolderForm(&f, true)
							}
						}
					}
					return nil
				case 'd':
					// Delete current folder
					currentIndex := a.folderList.GetCurrentItem()
					if currentIndex >= 0 && currentIndex < len(a.folderItems) {
						item := a.folderItems[currentIndex]
						if item.ID != nil {
							// Show confirmation before deletion
							confirmMessage := fmt.Sprintf("Are you sure you want to delete folder '%s'?", item.Name)
							a.showConfirm(confirmMessage, func() {
								if err := a.folderSvc.Delete(*item.ID); err != nil {
									a.showError(fmt.Sprintf("Error deleting folder: %v", err))
								} else {
									a.reloadFolders()
									a.reloadBookmarks() // Reload bookmarks as they may be in deleted folder
								}
							})
						}
					}
					return nil
				}
			}
			// Pass other events to list for navigation
			return event
		}

		// If focus on item list, handle normal commands
		switch event.Key() {
		case tcell.KeyEnter:
			if a.currentItem != nil {
				if a.currentItem.Type == models.ItemTypeBookmark {
					// Open bookmark
					if a.currentItem.URL != nil && *a.currentItem.URL != "" {
						openURL(*a.currentItem.URL)
					}
				} else if a.currentItem.Type == models.ItemTypeFolder {
					// Navigate into folder
					folderID := a.currentItem.ID
					a.selectedFolder = &folderID
					if err := a.loadFolderContent(); err != nil {
						a.allItems = []models.Item{}
						a.items = []models.Item{}
						a.fillList()
					}
					a.updateStatus()
				}
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				a.setMode(ModeSearch)
				return nil
			case 'a':
				// Create new bookmark
				newBookmark := models.Bookmark{}
				// If folder selected, set it as default
				// Important: create a copy of pointer to avoid issues
				if a.selectedFolder != nil {
					folderID := *a.selectedFolder
					newBookmark.FolderID = &folderID
				}
				a.showForm(&newBookmark, false)
				return nil
			case 'e':
				if a.currentItem != nil {
					if a.currentItem.Type == models.ItemTypeBookmark {
						// Edit bookmark
						if a.current == nil {
							a.convertItemToBookmark(a.currentItem)
						}
						if a.current != nil {
							b := *a.current
							a.showForm(&b, true)
						}
					} else if a.currentItem.Type == models.ItemTypeFolder {
						// Edit folder
						folder, err := a.folderSvc.GetByID(a.currentItem.ID)
						if err == nil && folder != nil {
							f := *folder
							a.showFolderForm(&f, true)
						}
					}
				}
				return nil
			case 'd':
				if a.currentItem != nil {
					if a.currentItem.Type == models.ItemTypeBookmark {
						// Delete bookmark
						if a.current == nil {
							a.convertItemToBookmark(a.currentItem)
						}
						if a.current != nil {
							confirmMessage := fmt.Sprintf("Are you sure you want to delete bookmark '%s'?", a.current.Title)
							a.showConfirm(confirmMessage, func() {
								if err := a.bookmarkSvc.Delete(a.current.ID); err != nil {
									a.showError(fmt.Sprintf("Error deleting bookmark: %v", err))
								} else {
									a.reloadBookmarks()
								}
							})
						}
					} else if a.currentItem.Type == models.ItemTypeFolder {
						// Delete folder
						confirmMessage := fmt.Sprintf("Are you sure you want to delete folder '%s'?", a.currentItem.Name)
						a.showConfirm(confirmMessage, func() {
							if err := a.folderSvc.Delete(a.currentItem.ID); err != nil {
								a.showError(fmt.Sprintf("Error deleting folder: %v", err))
							} else {
								a.reloadFolders()
								a.reloadBookmarks()
							}
						})
					}
				}
				return nil
			case 'q':
				a.app.Stop()
				return nil
			}
		}
	case ModeForm:
		switch event.Key() {
		case tcell.KeyEscape:
			a.pages.RemovePage("form")
			a.pages.RemovePage("folderForm")
			a.setMode(ModeNormal)
		}
	}
	return event
}

func (a *App) showForm(b *models.Bookmark, edit bool) {
	title := b.Title
	url := b.URL
	desc := b.Description

	// Get list of all folders for dropdown
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		// On error use empty list
		folders = []models.Folder{}
	}

	// Create list of options for dropdown
	// First option - "None" (no folder)
	folderOptions := []string{"None"}
	folderIDs := make([]*int, 1, len(folders)+1)
	folderIDs[0] = nil // nil means no folder

	// Add all folders
	// Important: create copies of IDs in separate slice to avoid pointer issues
	folderIDValues := make([]int, len(folders))
	for i, folder := range folders {
		folderOptions = append(folderOptions, folder.Name)
		folderIDValues[i] = folder.ID                     // Save copy of ID
		folderIDs = append(folderIDs, &folderIDValues[i]) // Pointer to slice element
	}

	// Find index of currently selected folder
	selectedIndex := 0 // Default "None"
	// If this is a new bookmark and FolderID is not set, but selectedFolder exists, use it
	if !edit && b.FolderID == nil && a.selectedFolder != nil {
		// Create copy of pointer
		folderID := *a.selectedFolder
		b.FolderID = &folderID
	}
	if b.FolderID != nil {
		for i, folderID := range folderIDs {
			if folderID != nil && *folderID == *b.FolderID {
				selectedIndex = i
				break
			}
		}
	}

	form := tview.NewForm()
	form.AddInputField("Title", title, 60, nil, func(t string) { b.Title = t })
	form.AddInputField("URL", url, 60, nil, func(t string) { b.URL = t })
	form.AddInputField("Description", desc, 60, nil, func(t string) { b.Description = t })

	// Add dropdown for folder selection
	form.AddDropDown("Folder", folderOptions, selectedIndex, func(option string, index int) {
		// Set FolderID based on selected option
		if index >= 0 && index < len(folderIDs) {
			b.FolderID = folderIDs[index]
		} else {
			b.FolderID = nil
		}
	})

	form.AddButton("Save", func() {
		// Validation: URL is required
		if b.URL == "" {
			a.showError("Error: URL is required")
			return // Don't close form so user can fix it
		}

		// Validation: Title is desirable but not required
		if b.Title == "" {
			// Can use URL as title, but better to warn
			// For now just continue
		}

		var err error
		if edit {
			err = a.bookmarkSvc.Update(b)
		} else {
			err = a.bookmarkSvc.Create(b)
		}

		if err != nil {
			// Show error to user
			a.showError(fmt.Sprintf("Error saving bookmark: %v", err))
			return // Don't close form on error
		}

		// Successfully saved
		a.reloadBookmarks()
		a.pages.RemovePage("form")
		a.setMode(ModeNormal)
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("form")
		a.setMode(ModeNormal)
	})

	form.SetBorder(true).SetTitle("Bookmark")
	a.pages.AddPage("form", form, true, true)
	a.app.SetFocus(form)
	a.mode = ModeForm
}

// showFolderForm shows form for creating/editing folder
func (a *App) showFolderForm(f *models.Folder, edit bool) {
	name := f.Name
	var parentFolderID *int
	if f.ParentID != nil {
		parentFolderID = f.ParentID
	}

	// Get list of all folders for parent folder dropdown
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		folders = []models.Folder{}
	}

	// Create list of options for dropdown
	// First option - "None" (root folder)
	parentOptions := []string{"None (Root)"}
	parentIDs := make([]*int, 1, len(folders)+1)
	parentIDs[0] = nil // nil means root folder

	// Add all folders (excluding current one when editing)
	folderIDValues := make([]int, 0, len(folders))
	for i := range folders {
		folder := &folders[i]
		// When editing exclude the folder itself and its child folders (to avoid circular references)
		if edit && f.ID != 0 && folder.ID == f.ID {
			continue
		}
		// Also exclude child folders (simplified check)
		if edit && f.ID != 0 && folder.ParentID != nil && *folder.ParentID == f.ID {
			continue
		}
		parentOptions = append(parentOptions, folder.Name)
		folderIDValues = append(folderIDValues, folder.ID)
		parentIDs = append(parentIDs, &folderIDValues[len(folderIDValues)-1])
	}

	// Find index of current parent folder
	selectedParentIndex := 0 // Default "None (Root)"
	if parentFolderID != nil {
		for i, pid := range parentIDs {
			if pid != nil && *pid == *parentFolderID {
				selectedParentIndex = i
				break
			}
		}
	}

	form := tview.NewForm()
	form.AddInputField("Name", name, 60, nil, func(t string) { f.Name = t })

	// Add dropdown for parent folder selection
	form.AddDropDown("Parent Folder", parentOptions, selectedParentIndex, func(option string, index int) {
		// Set ParentID based on selected option
		if index >= 0 && index < len(parentIDs) {
			f.ParentID = parentIDs[index]
		} else {
			f.ParentID = nil
		}
	})

	form.AddButton("Save", func() {
		// Validation: folder name is required
		if f.Name == "" {
			a.showError("Error: Folder name is required")
			return // Don't close form
		}

		var err error
		if edit {
			err = a.folderSvc.Update(f)
		} else {
			_, err = a.folderSvc.Create(f.Name, f.ParentID)
		}

		if err != nil {
			// Show error to user
			a.showError(fmt.Sprintf("Error saving folder: %v", err))
			return // Don't close form on error
		}

		// Successfully saved
		// Reload folder and bookmark lists
		a.reloadFolders()
		a.reloadBookmarks()
		a.pages.RemovePage("folderForm")
		a.setMode(ModeNormal)
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("folderForm")
		a.setMode(ModeNormal)
	})

	formTitle := "New Folder"
	if edit {
		formTitle = "Edit Folder"
	}
	form.SetBorder(true).SetTitle(formTitle)
	a.pages.AddPage("folderForm", form, true, true)
	a.app.SetFocus(form)
	a.mode = ModeForm
}

// showError shows modal window with error
func (a *App) showError(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("error")
			// Restore mode and focus
			if a.pages.HasPage("form") || a.pages.HasPage("folderForm") {
				a.mode = ModeForm
			} else {
				a.mode = ModeNormal
				if a.focusOnFolders {
					a.app.SetFocus(a.folderList)
				} else {
					a.app.SetFocus(a.list)
				}
			}
		})

	modal.SetBorder(true).SetTitle("Error")
	a.pages.AddPage("error", modal, true, true)
	oldMode := a.mode
	a.mode = ModeModal
	a.app.SetFocus(modal)
	_ = oldMode
}

func (a *App) showConfirm(message string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Cancel", "OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("confirm")
			if buttonIndex == 1 && onConfirm != nil {
				onConfirm()
			}
			// Restore mode and focus
			if a.pages.HasPage("form") || a.pages.HasPage("folderForm") {
				a.mode = ModeForm
			} else {
				a.mode = ModeNormal
				if a.focusOnFolders {
					a.app.SetFocus(a.folderList)
				} else {
					a.app.SetFocus(a.list)
				}
			}
		})

	modal.SetBorder(true).SetTitle("Confirm")
	a.pages.AddPage("confirm", modal, true, true)
	a.mode = ModeModal
	a.app.SetFocus(modal)
}

func openURL(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}
