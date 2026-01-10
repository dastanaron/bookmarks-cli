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

// folderItem представляет элемент папки в списке
type folderItem struct {
	ID    *int // nil для "All Bookmarks"
	Name  string
	Level int // уровень вложенности (0 = корень)
}

// App represents the TUI application
type App struct {
	app            *tview.Application
	folderList     *tview.List // список папок вместо дерева
	list           *tview.List // список элементов (закладки и папки)
	detail         *tview.TextView
	search         *tview.InputField
	pages          *tview.Pages
	mode           uint8
	allItems       []models.Item    // все элементы текущей папки (без фильтрации)
	items          []models.Item    // отфильтрованные элементы для отображения
	currentItem    *models.Item     // текущий выбранный элемент (закладка или папка)
	current        *models.Bookmark // для совместимости (используется в showDetails)
	status         *tview.TextView
	bookmarkSvc    *service.BookmarkService
	folderSvc      *service.FolderService
	selectedFolder *int         // ID выбранной папки, nil = корневая папка
	focusOnFolders bool         // true = фокус на списке папок, false = на списке элементов
	folderItems    []folderItem // список папок для быстрого доступа
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
		selectedFolder: nil, // По умолчанию показываем все закладки
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

	// SetSelectedFunc не используем - выбор обрабатывается через Enter в globalInput
	// Это позволяет избежать случайного выбора при навигации стрелками

	a.app.SetRoot(a.pages, true)
	a.app.SetInputCapture(a.globalInput)
	a.updateStatus()

	// Начальный фокус на списке закладок
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
	// Перезагружаем содержимое текущей папки
	return a.loadFolderContent()
}

func (a *App) loadFolderContent() error {
	var err error
	// Получаем содержимое выбранной папки (закладки и подпапки)
	a.allItems, err = a.folderSvc.GetFolderContent(a.selectedFolder)
	if err != nil {
		// В случае ошибки показываем пустой список
		a.allItems = []models.Item{}
		a.items = []models.Item{}
		a.fillList()
		return err
	}

	// Применяем поисковый фильтр, если есть
	if a.search.GetText() != "" {
		a.applyFilter(a.search.GetText())
	} else {
		// Без фильтра показываем все элементы
		a.items = a.allItems
		a.fillList()
	}

	// Обновляем заголовок списка
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
	// Если нет поискового запроса, показываем все элементы текущей папки
	if text == "" {
		a.items = a.allItems
		a.fillList()
		return
	}

	// Если папка не выбрана, ищем по всем закладкам через SearchInFolder
	if a.selectedFolder == nil {
		bookmarks, err := a.bookmarkSvc.SearchInFolder(text, nil)
		if err != nil {
			a.items = []models.Item{}
			a.fillList()
			return
		}

		// Преобразуем закладки в Items
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

	// Если папка выбрана, фильтруем элементы внутри этой папки
	textLower := strings.ToLower(text)
	var filtered []models.Item
	for _, item := range a.allItems {
		itemNameLower := strings.ToLower(item.Name)

		// Проверяем название
		if strings.Contains(itemNameLower, textLower) {
			filtered = append(filtered, item)
			continue
		}

		// Для закладок также проверяем URL и описание
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

// onFolderSelect обрабатывает выбор папки в списке (вызывается при нажатии Enter)
func (a *App) onFolderSelect(item folderItem) {
	// Устанавливаем выбранную папку
	// Важно: создаем новую переменную для ID, чтобы избежать проблем с указателями
	var newSelectedFolder *int
	if item.ID != nil {
		folderID := *item.ID
		newSelectedFolder = &folderID
	} else {
		newSelectedFolder = nil
	}
	a.selectedFolder = newSelectedFolder

	// Обновить заголовок списка папок
	if item.ID == nil {
		a.folderList.SetTitle("Folders (All)")
	} else {
		a.folderList.SetTitle(fmt.Sprintf("Folders (%s)", item.Name))
	}

	// Загружаем содержимое выбранной папки
	if err := a.loadFolderContent(); err != nil {
		// В случае ошибки показываем пустой список
		a.allItems = []models.Item{}
		a.items = []models.Item{}
		a.fillList()
	}

	// Обновить статус бар
	a.updateStatus()

	// Переключить фокус на список элементов для удобства
	a.focusOnFolders = false
	a.app.SetFocus(a.list)

	// НЕ вызываем app.Draw() здесь - это может вызвать зависание
	// UI обновится автоматически при следующем цикле обработки событий
}

func (a *App) fillList() {
	a.list.Clear()
	for i := range a.items {
		index := i
		item := a.items[i]

		// Формируем отображаемое название и вторичный текст
		var mainText, secondaryText string
		if item.Type == models.ItemTypeFolder {
			// Для папок показываем иконку папки
			mainText = fmt.Sprintf("📁 %s", item.Name)
			secondaryText = "Folder"
		} else {
			// Для закладок показываем название и URL
			mainText = item.Name
			if item.URL != nil {
				secondaryText = *item.URL
			}
		}

		a.list.AddItem(mainText, secondaryText, 0, func() {
			if index >= 0 && index < len(a.items) {
				a.currentItem = &a.items[index]
				// Для совместимости создаем bookmark, если это закладка
				if a.items[index].Type == models.ItemTypeBookmark {
					a.convertItemToBookmark(&a.items[index])
				} else {
					a.current = nil
				}
				a.showDetails()
			}
		})
	}

	// Устанавливаем текущий элемент, если есть
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

// convertItemToBookmark преобразует Item в Bookmark для совместимости
func (a *App) convertItemToBookmark(item *models.Item) {
	if item.Type != models.ItemTypeBookmark {
		return
	}

	var folderName *string
	if item.ParentID != nil {
		// Получаем название папки по ID
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

// reloadFolders перезагружает список папок
func (a *App) reloadFolders() error {
	if err := a.fillFolderList(); err != nil {
		return err
	}
	return nil
}

// fillFolderList заполняет список папок с отступами для показа иерархии
func (a *App) fillFolderList() error {
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		return err
	}

	// Создаем карту папок для быстрого доступа
	folderMap := make(map[int]*models.Folder)
	for i := range folders {
		folderMap[folders[i].ID] = &folders[i]
	}

	a.folderList.Clear()
	a.folderItems = []folderItem{}

	// Добавляем корневой элемент "All Bookmarks"
	allItem := folderItem{ID: nil, Name: "All Bookmarks", Level: 0}
	a.folderItems = append(a.folderItems, allItem)
	a.folderList.AddItem(allItem.Name, "", 0, nil)

	// Рекурсивная функция для добавления папок с правильной иерархией
	var buildList func(parentID *int, level int)
	buildList = func(parentID *int, level int) {
		for _, folder := range folders {
			// Проверяем, является ли эта папка дочерней для текущего родителя
			var isChild bool
			if parentID == nil {
				// Ищем папки без родителя или с parentID = 0
				isChild = folder.ParentID == nil || *folder.ParentID == 0
			} else {
				// Ищем папки с указанным родителем
				isChild = folder.ParentID != nil && *folder.ParentID == *parentID
			}

			if isChild {
				// Создаем отступ в зависимости от уровня
				indent := ""
				for i := 0; i < level; i++ {
					indent += "  " // 2 пробела на уровень
				}
				if level > 0 {
					indent += "└─ " // символ для показа вложенности
				}

				// Важно: создаем копию ID, чтобы избежать проблем с указателями
				folderID := folder.ID
				item := folderItem{
					ID:    &folderID,
					Name:  folder.Name,
					Level: level,
				}
				a.folderItems = append(a.folderItems, item)
				a.folderList.AddItem(indent+folder.Name, "", 0, nil)

				// Рекурсивно добавляем дочерние папки
				childParentID := &folder.ID
				buildList(childParentID, level+1)
			}
		}
	}

	// Начинаем с корневого уровня (parentID = nil)
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
		// Показываем информацию о папке
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
		// Показываем информацию о закладке
		b := a.current
		if b == nil {
			// Если current не установлен, используем данные из item
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

// toggleFocus переключает фокус между списком папок и списком закладок
func (a *App) toggleFocus() {
	a.focusOnFolders = !a.focusOnFolders
	if a.focusOnFolders {
		a.app.SetFocus(a.folderList)
		// Обновим заголовок с информацией о выбранной папке
		if a.selectedFolder != nil {
			// Найти имя выбранной папки
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
			// Обновим заголовок списка папок
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
		// При очистке поиска перезагружаем содержимое папки
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
	// Проверяем, открыто ли модальное окно, перед проверкой режима
	if a.pages.HasPage("confirm") || a.pages.HasPage("error") {
		// В модальном окне Tab переключает между кнопками, Enter выбирает
		// Не обрабатываем эти события здесь, чтобы модальное окно могло их обработать
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEnter, tcell.KeyLeft, tcell.KeyRight, tcell.KeyEscape:
			// Передаем эти события модальному окну
			return event
		default:
			// Для остальных событий также передаем их модальному окну
			return event
		}
	}

	switch a.mode {
	case ModeNormal:
		// Tab для переключения между деревом и списком
		if event.Key() == tcell.KeyTab {
			a.toggleFocus()
			return nil
		}

		// Если фокус на списке папок, обрабатываем горячие клавиши
		if a.focusOnFolders {
			switch event.Key() {
			case tcell.KeyEnter:
				// Enter - выбрать папку
				// Получаем текущий индекс из списка папок
				currentIndex := a.folderList.GetCurrentItem()
				if currentIndex >= 0 && currentIndex < len(a.folderItems) {
					item := a.folderItems[currentIndex]
					// Вызываем обработчик выбора папки
					a.onFolderSelect(item)
					// Возвращаем nil, чтобы событие не передавалось дальше
					return nil
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
				case 'a':
					// Добавить новую папку
					a.showFolderForm(&models.Folder{}, false)
					return nil
				case 'e':
					// Редактировать текущую папку
					currentIndex := a.folderList.GetCurrentItem()
					if currentIndex >= 0 && currentIndex < len(a.folderItems) {
						item := a.folderItems[currentIndex]
						if item.ID != nil {
							// Получаем папку из базы
							folder, err := a.folderSvc.GetByID(*item.ID)
							if err == nil && folder != nil {
								f := *folder
								a.showFolderForm(&f, true)
							}
						}
					}
					return nil
				case 'd':
					// Удалить текущую папку
					currentIndex := a.folderList.GetCurrentItem()
					if currentIndex >= 0 && currentIndex < len(a.folderItems) {
						item := a.folderItems[currentIndex]
						if item.ID != nil {
							// Показываем подтверждение перед удалением
							confirmMessage := fmt.Sprintf("Are you sure you want to delete folder '%s'?", item.Name)
							a.showConfirm(confirmMessage, func() {
								if err := a.folderSvc.Delete(*item.ID); err != nil {
									a.showError(fmt.Sprintf("Error deleting folder: %v", err))
								} else {
									a.reloadFolders()
									a.reloadBookmarks() // Обновляем закладки, так как они могут быть в удаленной папке
								}
							})
						}
					}
					return nil
				}
			}
			// Остальные события передаем списку для навигации
			return event
		}

		// Если фокус на списке элементов, обрабатываем обычные команды
		switch event.Key() {
		case tcell.KeyEnter:
			if a.currentItem != nil {
				if a.currentItem.Type == models.ItemTypeBookmark {
					// Открываем закладку
					if a.currentItem.URL != nil && *a.currentItem.URL != "" {
						openURL(*a.currentItem.URL)
					}
				} else if a.currentItem.Type == models.ItemTypeFolder {
					// Переходим в папку
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
				// Создаем новую закладку
				newBookmark := models.Bookmark{}
				// Если выбрана папка, устанавливаем её по умолчанию
				// Важно: создаем копию указателя, чтобы избежать проблем
				if a.selectedFolder != nil {
					folderID := *a.selectedFolder
					newBookmark.FolderID = &folderID
				}
				a.showForm(&newBookmark, false)
				return nil
			case 'e':
				if a.currentItem != nil {
					if a.currentItem.Type == models.ItemTypeBookmark {
						// Редактируем закладку
						if a.current == nil {
							a.convertItemToBookmark(a.currentItem)
						}
						if a.current != nil {
							b := *a.current
							a.showForm(&b, true)
						}
					} else if a.currentItem.Type == models.ItemTypeFolder {
						// Редактируем папку
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
						// Удаляем закладку
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
						// Удаляем папку
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

	// Получаем список всех папок для выпадающего списка
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		// В случае ошибки используем пустой список
		folders = []models.Folder{}
	}

	// Создаем список опций для выпадающего списка
	// Первая опция - "None" (нет папки)
	folderOptions := []string{"None"}
	folderIDs := make([]*int, 1, len(folders)+1)
	folderIDs[0] = nil // nil означает отсутствие папки

	// Добавляем все папки
	// Важно: создаем копии ID в отдельном слайсе, чтобы избежать проблем с указателями
	folderIDValues := make([]int, len(folders))
	for i, folder := range folders {
		folderOptions = append(folderOptions, folder.Name)
		folderIDValues[i] = folder.ID                     // Сохраняем копию ID
		folderIDs = append(folderIDs, &folderIDValues[i]) // Указатель на элемент слайса
	}

	// Находим индекс текущей выбранной папки
	selectedIndex := 0 // По умолчанию "None"
	// Если это новая закладка и FolderID не установлен, но есть selectedFolder, используем его
	if !edit && b.FolderID == nil && a.selectedFolder != nil {
		// Создаем копию указателя
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

	// Добавляем выпадающий список для выбора папки
	form.AddDropDown("Folder", folderOptions, selectedIndex, func(option string, index int) {
		// Устанавливаем FolderID в зависимости от выбранной опции
		if index >= 0 && index < len(folderIDs) {
			b.FolderID = folderIDs[index]
		} else {
			b.FolderID = nil
		}
	})

	form.AddButton("Save", func() {
		// Валидация: URL обязателен
		if b.URL == "" {
			a.showError("Error: URL is required")
			return // Не закрываем форму, чтобы пользователь мог исправить
		}

		// Валидация: Title желателен, но не обязателен
		if b.Title == "" {
			// Можно использовать URL как заголовок, но лучше предупредить
			// Пока просто продолжаем
		}

		var err error
		if edit {
			err = a.bookmarkSvc.Update(b)
		} else {
			err = a.bookmarkSvc.Create(b)
		}

		if err != nil {
			// Показываем ошибку пользователю
			a.showError(fmt.Sprintf("Error saving bookmark: %v", err))
			return // Не закрываем форму при ошибке
		}

		// Успешно сохранено
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

// showFolderForm показывает форму для создания/редактирования папки
func (a *App) showFolderForm(f *models.Folder, edit bool) {
	name := f.Name
	var parentFolderID *int
	if f.ParentID != nil {
		parentFolderID = f.ParentID
	}

	// Получаем список всех папок для выпадающего списка родительской папки
	folders, err := a.folderSvc.ListAll()
	if err != nil {
		folders = []models.Folder{}
	}

	// Создаем список опций для выпадающего списка
	// Первая опция - "None" (корневая папка)
	parentOptions := []string{"None (Root)"}
	parentIDs := make([]*int, 1, len(folders)+1)
	parentIDs[0] = nil // nil означает корневую папку

	// Добавляем все папки (исключая текущую при редактировании)
	folderIDValues := make([]int, 0, len(folders))
	for i := range folders {
		folder := &folders[i]
		// При редактировании исключаем саму папку и её дочерние папки (чтобы избежать циклических ссылок)
		if edit && f.ID != 0 && folder.ID == f.ID {
			continue
		}
		// Также исключаем дочерние папки (упрощенная проверка)
		if edit && f.ID != 0 && folder.ParentID != nil && *folder.ParentID == f.ID {
			continue
		}
		parentOptions = append(parentOptions, folder.Name)
		folderIDValues = append(folderIDValues, folder.ID)
		parentIDs = append(parentIDs, &folderIDValues[len(folderIDValues)-1])
	}

	// Находим индекс текущей родительской папки
	selectedParentIndex := 0 // По умолчанию "None (Root)"
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

	// Добавляем выпадающий список для выбора родительской папки
	form.AddDropDown("Parent Folder", parentOptions, selectedParentIndex, func(option string, index int) {
		// Устанавливаем ParentID в зависимости от выбранной опции
		if index >= 0 && index < len(parentIDs) {
			f.ParentID = parentIDs[index]
		} else {
			f.ParentID = nil
		}
	})

	form.AddButton("Save", func() {
		// Валидация: название папки обязательно
		if f.Name == "" {
			a.showError("Error: Folder name is required")
			return // Не закрываем форму
		}

		var err error
		if edit {
			err = a.folderSvc.Update(f)
		} else {
			_, err = a.folderSvc.Create(f.Name, f.ParentID)
		}

		if err != nil {
			// Показываем ошибку пользователю
			a.showError(fmt.Sprintf("Error saving folder: %v", err))
			return // Не закрываем форму при ошибке
		}

		// Успешно сохранено
		// Перезагружаем список папок и закладок
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

// showError показывает модальное окно с ошибкой
func (a *App) showError(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("error")
			// Восстанавливаем режим и фокус
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
			// Восстанавливаем режим и фокус
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
