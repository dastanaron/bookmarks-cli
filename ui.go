package main

import (
	"database/sql"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var MODE = map[string]uint8{
	"normal": 1,
	"search": 2,
	"form":   3,
}

type App struct {
	db       *sql.DB
	app      *tview.Application
	tree     *tview.TreeView
	list     *tview.List
	detail   *tview.TextView
	search   *tview.InputField
	pages    *tview.Pages
	mode     uint8
	all      []Bookmark
	filtered []Bookmark
	current  *Bookmark
	status   *tview.TextView
}

func NewApp(db *sql.DB) *App {
	return &App{
		db:     db,
		app:    tview.NewApplication(),
		tree:   tview.NewTreeView(),
		list:   tview.NewList(),
		detail: tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		search: tview.NewInputField().SetLabel("Search: "),
		pages:  tview.NewPages(),
		mode:   MODE["normal"],
		status: tview.NewTextView().SetDynamicColors(true),
	}
}
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

	a.fillTree()

	a.search.SetChangedFunc(a.onSearchChange)
	a.search.SetDoneFunc(a.onSearchDone)
	a.list.SetChangedFunc(a.onSelect)

	a.app.SetRoot(a.pages, true)
	a.app.SetInputCapture(a.globalInput)
	a.updateStatus()
	return a.app.Run()
}

func (a *App) updateStatus() {
	a.status.SetText(
		"[::b]/[::-] search  [::b]a[::-] add  [::b]e[::-] edit  [::b]d[::-] del  [::b]Enter[::-] open  [::b]q[::-] close",
	)
}

func (a *App) reloadBookmarks() error {
	var err error
	a.all, err = ListBookmarks(a.db)
	if err != nil {
		return err
	}
	a.applyFilter(a.search.GetText())
	return nil
}
func (a *App) applyFilter(text string) {
	low := strings.ToLower(text)
	a.filtered = a.filtered[:0]
	for _, b := range a.all {
		if text == "" ||
			strings.Contains(strings.ToLower(b.Title), low) ||
			strings.Contains(strings.ToLower(b.URL), low) ||
			strings.Contains(strings.ToLower(b.Description), low) {
			a.filtered = append(a.filtered, b)
		}
	}
	a.fillList()
}
func (a *App) fillList() {
	a.list.Clear()
	for _, b := range a.filtered {
		bookmark := b
		a.list.AddItem(b.Title, b.URL, 0, func() {
			a.current = &bookmark
			a.showDetails()
		})
	}
	if len(a.filtered) > 0 {
		a.current = &a.filtered[0]
		a.showDetails()
	}
}

func (a *App) fillTree() error {
	folders, err := ListFolders(a.db)
	if err != nil {
		return err
	}

	nodes := make(map[int]*tview.TreeNode, len(folders))
	for _, f := range folders {
		node := tview.NewTreeNode(f.Name).
			SetReference(f.ID)
		nodes[f.ID] = node
	}

	rootAdded := false
	for _, f := range folders {
		n := nodes[f.ID]
		if f.ParentID == nil || *f.ParentID == 0 {
			if !rootAdded {
				a.tree.SetRoot(n)
				rootAdded = true
			} else {
				if a.tree.GetRoot() == nil {
					r := tview.NewTreeNode("./")
					a.tree.SetRoot(r)
					rootAdded = true
				}
				a.tree.GetRoot().AddChild(n)
			}
		} else {
			if p, ok := nodes[*f.ParentID]; ok {
				p.AddChild(n)
			} else {
				if a.tree.GetRoot() == nil {
					r := tview.NewTreeNode("./")
					a.tree.SetRoot(r)
					rootAdded = true
				}
				a.tree.GetRoot().AddChild(n)
			}
		}
	}

	if a.tree.GetRoot() == nil {
		a.tree.SetRoot(tview.NewTreeNode("./"))
	}

	a.tree.GetRoot().SetExpanded(true)
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

// ----------------------- режимы -----------------------
func (a *App) setMode(m uint8) {
	a.mode = m
	switch m {
	case MODE["search"]:
		a.app.SetFocus(a.search)
	case MODE["normal"]:
		a.app.SetFocus(a.list)
	}
}
func (a *App) onSearchChange(text string) {
	a.applyFilter(text)
}
func (a *App) onSearchDone(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		a.setMode(MODE["normal"])
	case tcell.KeyEscape:
		a.search.SetText("")
		a.applyFilter("")
		a.setMode(MODE["normal"])
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
	case MODE["normal"]:
		switch event.Key() {
		case tcell.KeyEnter:
			if a.current != nil && a.current.URL != "" {
				openURL(a.current.URL)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				a.setMode(MODE["search"])
				return nil
			case 'a':
				a.showForm(&Bookmark{}, false)
				return nil
			case 'e':
				if a.current != nil {
					a.showForm(a.current, true)
				}
				return nil
			case 'd':
				if a.current != nil {
					_ = DeleteBookmark(a.db, a.current.ID)
					a.reloadBookmarks()
				}
				return nil
			case 'q':
				a.app.Stop()
				return nil
			}
		}
	case MODE["form"]:
		switch event.Key() {
		case tcell.KeyEscape:
			a.pages.RemovePage("form")
			a.setMode(MODE["normal"])
		}
	}
	return event
}

func (a *App) showForm(b *Bookmark, edit bool) {
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
			err = UpdateBookmark(a.db, *b)
		} else {
			err = StoreBookmark(a.db, *b)
		}
		if err == nil {
			a.reloadBookmarks()
		}
		a.pages.RemovePage("form")
		a.setMode(MODE["normal"])
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("form")
		a.setMode(MODE["normal"])
	})

	form.SetBorder(true).SetTitle("Bookmark")
	a.pages.AddPage("form", form, true, true)
	a.app.SetFocus(form)
	a.mode = MODE["form"]
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
