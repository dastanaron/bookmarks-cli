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

type App struct {
	db       *sql.DB
	app      *tview.Application
	list     *tview.List
	detail   *tview.TextView
	search   *tview.InputField
	pages    *tview.Pages
	mode     string // normal | search | form
	all      []Bookmark
	filtered []Bookmark
	current  *Bookmark
	status   *tview.TextView
}

func NewApp(db *sql.DB) *App {
	return &App{
		db:     db,
		app:    tview.NewApplication(),
		list:   tview.NewList(),
		detail: tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		search: tview.NewInputField().SetLabel("Search: "),
		pages:  tview.NewPages(),
		mode:   "normal",
		status: tview.NewTextView().SetDynamicColors(true),
	}
}
func (a *App) Run() error {
	a.list.SetBorder(true).SetTitle("Bookmarks")
	a.detail.SetBorder(true).SetTitle("Details")
	cols := tview.NewFlex().
		AddItem(a.list, 0, 1, true).
		AddItem(a.detail, 0, 1, false)
	main := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.search, 1, 0, false).
		AddItem(cols, 0, 1, true).
		AddItem(a.status, 1, 0, false)
	a.pages.AddPage("main", main, true, true)

	if err := a.reloadBookmarks(); err != nil {
		return err
	}

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
		"[::b]/[::-] search  [::b]a[::-] add  [::b]e[::-] edit  [::b]d[::-] del  [::b]Enter[::-] open",
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
func (a *App) showDetails() {
	if a.current == nil {
		a.detail.SetText("")
		return
	}
	b := a.current
	pid := ""
	if b.ParentID != nil && *b.ParentID != 0 {
		pid = fmt.Sprintf("%d", *b.ParentID)
	}
	text := fmt.Sprintf(
		"[::b]Title:[::-]\n%s\n\n[::b]URL:[::-]\n%s\n\n[::b]Description:[::-]\n%s\n\n[::b]Folder ID:[::-]\n%s",
		b.Title, b.URL, b.Description, pid)
	a.detail.SetText(text)
}

// ----------------------- режимы -----------------------
func (a *App) setMode(m string) {
	a.mode = m
	switch m {
	case "search":
		a.app.SetFocus(a.search)
	case "normal":
		a.app.SetFocus(a.list)
	}
}
func (a *App) onSearchChange(text string) {
	a.applyFilter(text)
}
func (a *App) onSearchDone(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		a.setMode("normal")
	case tcell.KeyEscape:
		a.search.SetText("")
		a.applyFilter("")
		a.setMode("normal")
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
	case "normal":
		switch event.Key() {
		case tcell.KeyEnter:
			if a.current != nil && a.current.URL != "" {
				openURL(a.current.URL)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				a.setMode("search")
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
	case "form":
		switch event.Key() {
		case tcell.KeyEscape:
			a.pages.RemovePage("form")
			a.setMode("normal")
		}
	}
	return event
}

func (a *App) showForm(b *Bookmark, edit bool) {
	title := b.Title
	url := b.URL
	desc := b.Description
	var folder string
	if b.ParentID != nil && *b.ParentID != 0 {
		folder = fmt.Sprintf("%d", *b.ParentID)
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
				b.ParentID = nil
			} else {
				b.ParentID = &id
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
		a.setMode("normal")
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("form")
		a.setMode("normal")
	})

	form.SetBorder(true).SetTitle("Bookmark")
	a.pages.AddPage("form", form, true, true)
	a.app.SetFocus(form)
	a.mode = "form"
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
