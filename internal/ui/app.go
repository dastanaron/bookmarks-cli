package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/dastanaron/bookmarks/internal/models"
	"github.com/dastanaron/bookmarks/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	ModeNormal = 1
	ModeSearch = 2
	ModeForm   = 3
)

// App represents the TUI application
type App struct {
	app            *tview.Application
	tree           *tview.TreeView
	list           *tview.List
	detail         *tview.TextView
	search         *tview.InputField
	pages          *tview.Pages
	mode           uint8
	all            []models.Bookmark
	filtered       []models.Bookmark
	current        *models.Bookmark
	status         *tview.TextView
	bookmarkSvc    *service.BookmarkService
	folderSvc      *service.FolderService
	selectedFolder *int // ID выбранной папки, nil = все закладки
	focusOnTree    bool // true = фокус на дереве, false = на списке
}

// NewApp creates a new application instance
func NewApp(bookmarkSvc *service.BookmarkService, folderSvc *service.FolderService) *App {
	return &App{
		app:            tview.NewApplication(),
		tree:           tview.NewTreeView(),
		list:           tview.NewList(),
		detail:         tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		search:         tview.NewInputField().SetLabel("Search: "),
		pages:          tview.NewPages(),
		mode:           ModeNormal,
		status:         tview.NewTextView().SetDynamicColors(true),
		bookmarkSvc:    bookmarkSvc,
		folderSvc:      folderSvc,
		selectedFolder: nil, // По умолчанию показываем все закладки
		focusOnTree:    false,
	}
}

// Run starts the application
func (a *App) Run() error {
	a.list.SetBorder(true).SetTitle("Bookmarks")
	a.detail.SetBorder(true).SetTitle("Details")
	a.tree.SetBorder(true).SetTitle("Folders")

	cols := tview.NewFlex().
		AddItem(a.tree, 0, 1, false).
		AddItem(a.list, 0, 3, true).
		AddItem(a.detail, 0, 1, false)

	main := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.search, 1, 0, false).
		AddItem(cols, 0, 1, true).
		AddItem(a.status, 1, 0, false)

	a.pages.AddPage("main", main, true, true)

	if err := a.reloadBookmarks(); err != nil {
		return err
	}

	if err := a.fillTree(); err != nil {
		return err
	}

	a.search.SetChangedFunc(a.onSearchChange)
	a.search.SetDoneFunc(a.onSearchDone)
	a.list.SetChangedFunc(a.onSelect)

	// Обработчик выбора в дереве - вызывается при изменении выделения
	// Но мы будем применять фильтр только при явном нажатии Enter/Space
	a.tree.SetSelectedFunc(func(node *tview.TreeNode) {
		// Этот обработчик вызывается при навигации, но мы не применяем фильтр здесь
		// Фильтр применяется только при нажатии Enter/Space в globalInput
	})

	// Обработчик ввода для дерева - передаем все события в globalInput
	a.tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Просто передаем событие дальше, обработка будет в globalInput
		return event
	})

	a.app.SetRoot(a.pages, true)
	a.app.SetInputCapture(a.globalInput)
	a.updateStatus()

	// Начальный фокус на списке
	a.focusOnTree = false
	a.app.SetFocus(a.list)
	return a.app.Run()
}

func (a *App) updateStatus() {
	statusText := "[::b]Tab[::r] switch  [::b]/[::r] search  [::b]a[::r] add  [::b]e[::r] edit  [::b]d[::r] del  [::b]Enter[::r] open/select  [::b]q[::r] quit"
	if a.focusOnTree {
		statusText = "[::b]Tab[::r] switch  [::b]Enter/Space[::r] select folder  [::b]q[::r] quit"
	}
	a.status.SetText(statusText)
}

func (a *App) reloadBookmarks() error {
	var err error
	a.all, err = a.bookmarkSvc.ListAll()
	if err != nil {
		return err
	}
	a.applyFilter(a.search.GetText())
	return nil
}

func (a *App) applyFilter(text string) {
	var err error
	// Используем SearchInFolder для учета выбранной папки
	a.filtered, err = a.bookmarkSvc.SearchInFolder(text, a.selectedFolder)
	if err != nil {
		return
	}
	a.fillList()
}

// onFolderSelect обрабатывает выбор папки в дереве (вызывается при нажатии Enter/Space)
func (a *App) onFolderSelect(node *tview.TreeNode) {
	if node == nil {
		return
	}

	ref := node.GetReference()
	if ref == nil {
		// Выбрана корневая папка "All Bookmarks" - показать все закладки
		a.selectedFolder = nil
		a.tree.SetTitle("Folders (All)")
	} else {
		// Выбрана конкретная папка
		folderID, ok := ref.(int)
		if !ok {
			// Если не int, значит это не папка - пропускаем
			return
		}
		// Устанавливаем выбранную папку
		a.selectedFolder = &folderID
		folderName := node.GetText()
		a.tree.SetTitle(fmt.Sprintf("Folders (%s)", folderName))
	}

	// Применить фильтр с учетом выбранной папки и текущего поискового запроса
	searchText := a.search.GetText()
	a.applyFilter(searchText)

	// Обновить статус бар
	a.updateStatus()

	// Оставить фокус на дереве, чтобы можно было выбрать другую папку
	// Пользователь может переключиться на список через Tab
}

func (a *App) fillList() {
	a.list.Clear()
	for i := range a.filtered {
		index := i
		a.list.AddItem(a.filtered[i].Title, a.filtered[i].URL, 0, func() {
			if index >= 0 && index < len(a.filtered) {
				a.current = &a.filtered[index]
				a.showDetails()
			}
		})
	}
	if len(a.filtered) > 0 {
		a.current = &a.filtered[0]
		a.showDetails()
	} else {
		a.current = nil
		a.showDetails()
	}
}

func (a *App) fillTree() error {
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		return err
	}

	nodes := make(map[int]*tview.TreeNode, len(folders))
	for _, folder := range folders {
		node := tview.NewTreeNode(folder.Name).
			SetReference(folder.ID)
		nodes[folder.ID] = node
	}

	// Создаем корневой узел "All Bookmarks"
	rootNode := tview.NewTreeNode("All Bookmarks")
	rootNode.SetReference(nil) // nil означает "все закладки"

	for _, folder := range folders {
		n := nodes[folder.ID]
		if folder.ParentID == nil || *folder.ParentID == 0 {
			rootNode.AddChild(n)
		} else {
			if p, ok := nodes[*folder.ParentID]; ok {
				p.AddChild(n)
			} else {
				// Если родитель не найден, добавляем в корень
				rootNode.AddChild(n)
			}
		}
	}

	a.tree.SetRoot(rootNode)

	a.tree.GetRoot().SetExpanded(true)
	a.tree.SetTitle("Folders (All)")
	return nil
}

func (a *App) showDetails() {
	if a.current == nil {
		a.detail.SetText("")
		return
	}

	b := a.current
	folderName := "/"
	if b.FolderName != nil {
		folderName = *b.FolderName
	}

	text := fmt.Sprintf(
		"[::b]Title:[::-]\n%s\n\n[::b]URL:[::-]\n%s\n\n[::b]Description:[::-]\n%s\n\n[::b]Folder:[::-]\n%s",
		b.Title, b.URL, b.Description, folderName)
	a.detail.SetText(text)
}

func (a *App) setMode(m uint8) {
	a.mode = m
	switch m {
	case ModeSearch:
		a.app.SetFocus(a.search)
	case ModeNormal:
		if a.focusOnTree {
			a.app.SetFocus(a.tree)
		} else {
			a.app.SetFocus(a.list)
		}
	}
}

// toggleFocus переключает фокус между деревом и списком
func (a *App) toggleFocus() {
	a.focusOnTree = !a.focusOnTree
	if a.focusOnTree {
		a.app.SetFocus(a.tree)
		// Обновим заголовок с информацией о выбранной папке
		if a.selectedFolder != nil {
			node := a.tree.GetCurrentNode()
			if node != nil {
				folderName := node.GetText()
				a.tree.SetTitle(fmt.Sprintf("Folders (%s) - Enter/Space to select", folderName))
			} else {
				a.tree.SetTitle("Folders - Enter/Space to select")
			}
		} else {
			a.tree.SetTitle("Folders (All) - Enter/Space to select")
		}
	} else {
		a.app.SetFocus(a.list)
		if a.selectedFolder != nil {
			// Обновим заголовок дерева
			node := a.tree.GetCurrentNode()
			if node != nil {
				folderName := node.GetText()
				a.tree.SetTitle(fmt.Sprintf("Folders (%s)", folderName))
			}
		} else {
			a.tree.SetTitle("Folders (All)")
		}
	}
	// Обновить статус бар
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
		a.applyFilter("")
		a.setMode(ModeNormal)
	}
}

func (a *App) onSelect(index int, mainText, secondaryText string, shortcut rune) {
	if index >= 0 && index < len(a.filtered) {
		a.current = &a.filtered[index]
		a.showDetails()
	}
}

func (a *App) globalInput(event *tcell.EventKey) *tcell.EventKey {
	switch a.mode {
	case ModeNormal:
		// Tab для переключения между деревом и списком
		if event.Key() == tcell.KeyTab {
			a.toggleFocus()
			return nil
		}

		// Если фокус на дереве, обрабатываем горячие клавиши
		if a.focusOnTree {
			switch event.Key() {
			case tcell.KeyEnter:
				// Enter - выбрать папку и применить фильтр
				node := a.tree.GetCurrentNode()
				if node != nil {
					a.onFolderSelect(node)
					// Обновить UI после выбора
					a.app.Draw()
				}
				return nil
			case tcell.KeyRune:
				switch event.Rune() {
				case 'q':
					// Выход из приложения
					a.app.Stop()
					return nil
				case '/':
					// Поиск
					a.setMode(ModeSearch)
					return nil
				case ' ': // Space - альтернативный способ выбора папки
					node := a.tree.GetCurrentNode()
					if node != nil {
						a.onFolderSelect(node)
						a.app.Draw()
					}
					return nil
				}
			}
			// Остальные события передаем дереву для навигации
			return event
		}

		// Если фокус на списке, обрабатываем обычные команды
		switch event.Key() {
		case tcell.KeyEnter:
			if a.current != nil && a.current.URL != "" {
				openURL(a.current.URL)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				a.setMode(ModeSearch)
				return nil
			case 'a':
				a.showForm(&models.Bookmark{}, false)
				return nil
			case 'e':
				if a.current != nil {
					// Create a copy to avoid modifying the original
					b := *a.current
					a.showForm(&b, true)
				}
				return nil
			case 'd':
				if a.current != nil {
					if err := a.bookmarkSvc.Delete(a.current.ID); err == nil {
						a.reloadBookmarks()
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
			a.setMode(ModeNormal)
		}
	}
	return event
}

func (a *App) showForm(b *models.Bookmark, edit bool) {
	title := b.Title
	url := b.URL
	desc := b.Description
	var folder string
	if b.FolderID != nil && *b.FolderID != 0 {
		folder = fmt.Sprintf("%d", *b.FolderID)
	} else {
		folder = "0"
	}

	form := tview.NewForm()
	form.AddInputField("Title", title, 60, nil, func(t string) { b.Title = t })
	form.AddInputField("URL", url, 60, nil, func(t string) { b.URL = t })
	form.AddInputField("Description", desc, 60, nil, func(t string) { b.Description = t })
	form.AddInputField("Folder (ID)", folder, 10, nil, func(t string) {
		if id, err := strconv.Atoi(t); err == nil {
			if id == 0 {
				b.FolderID = nil
			} else {
				b.FolderID = &id
			}
		}
	})

	form.AddButton("Save", func() {
		var err error
		if edit {
			err = a.bookmarkSvc.Update(b)
		} else {
			err = a.bookmarkSvc.Create(b)
		}
		if err == nil {
			a.reloadBookmarks()
		}
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
