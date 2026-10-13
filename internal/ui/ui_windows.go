//go:build windows

// Package ui implements the native Win32 interface (github.com/lxn/walk),
// mirroring the classic download-manager layout: toolbar, category tree, report
// table and dialogs. It drives the engine in-process, with no web server.
package ui

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"odm/internal/engine"
)

// Run starts the native UI and blocks until the window closes.
func Run(mgr *engine.Manager) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	app := &application{mgr: mgr}
	app.model = &downloadModel{app: app}
	return app.run()
}

type application struct {
	mgr *engine.Manager

	mw   *walk.MainWindow
	tv   *walk.TableView
	tree *walk.TreeView
	stat *walk.Label

	model   *downloadModel
	treeMod *catModel

	filter struct {
		kind  string
		value string
	}
	rowSig string

	mu    sync.Mutex
	items []engine.Info
}

func (a *application) run() error {
	a.treeMod = buildCategoryTree(nil)

	mw := MainWindow{
		AssignTo: &a.mw,
		Title:    "Ochi Download Manager",
		Size:     Size{Width: 1000, Height: 620},
		Layout:   VBox{MarginsZero: true, SpacingZero: true},
		ToolBar: ToolBar{
			ButtonStyle: ToolBarButtonTextOnly,
			Items:       a.toolbarItems(),
		},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					TreeView{
						AssignTo:             &a.tree,
						Model:                a.treeMod,
						MinSize:              Size{Width: 200},
						OnCurrentItemChanged: a.onCategoryChanged,
					},
					TableView{
						AssignTo:         &a.tv,
						AlternatingRowBG: true,
						Columns: []TableViewColumn{
							{Title: "File Name", Width: 250},
							{Title: "Size", Width: 90, Alignment: AlignFar},
							{Title: "Progress", Width: 80, Alignment: AlignFar},
							{Title: "Status", Width: 95},
							{Title: "Time left", Width: 95, Alignment: AlignFar},
							{Title: "Transfer rate", Width: 110, Alignment: AlignFar},
							{Title: "Path", Width: 260},
						},
						Model:                 a.model,
						OnCurrentIndexChanged: a.onRowChanged,
						OnItemActivated:       a.openProgressDialog,
					},
				},
			},
			Label{AssignTo: &a.stat, Text: ""},
		},
	}
	if err := mw.Create(); err != nil {
		return err
	}

	if menu := a.contextMenu(); menu != nil {
		a.tv.SetContextMenu(menu)
	}

	go a.refreshLoop()
	a.refresh()
	a.mw.Run()
	return nil
}

func (a *application) toolbarItems() []MenuItem {
	act := func(text string, fn func()) MenuItem {
		return Action{Text: text, OnTriggered: fn}
	}
	return []MenuItem{
		act("Add URL", a.openAddDialog),
		act("Resume", a.resumeSelected),
		act("Stop", a.stopSelected),
		act("Stop All", func() { a.mgr.PauseAll(); a.refresh() }),
		act("Delete", a.deleteSelected),
		act("Del Completed", func() { a.mgr.ClearCompleted(); a.refresh() }),
		act("Options", a.openOptionsDialog),
		Separator{},
		act("Pause All", func() { a.mgr.PauseAll(); a.refresh() }),
		act("Resume All", func() { a.mgr.ResumeAll(); a.refresh() }),
	}
}

func (a *application) contextMenu() *walk.Menu {
	menu, err := walk.NewMenu()
	if err != nil {
		return nil
	}
	add := func(text string, fn func()) {
		act := walk.NewAction()
		_ = act.SetText(text)
		act.Triggered().Attach(fn)
		_ = menu.Actions().Add(act)
	}
	add("Show progress", a.openProgressDialog)
	add("Open file", func() {
		if info, ok := a.currentDownload(); ok && info.Status == engine.StatusComplete {
			openPath(info.FilePath)
		}
	})
	add("Open folder", func() {
		if info, ok := a.currentDownload(); ok {
			revealPath(info.FilePath)
		}
	})
	add("Pause / Resume", a.toggleSelected)
	add("Remove from list", func() { a.removeSelected(false) })
	add("Remove + delete file", func() { a.removeSelected(true) })
	return menu
}

func (a *application) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		mw := a.mw
		if mw == nil {
			return
		}
		mw.Synchronize(func() {
			if mw.IsDisposed() {
				return
			}
			a.refresh()
		})
	}
}

func (a *application) refresh() {
	all := a.mgr.List()
	filtered := make([]engine.Info, 0, len(all))
	for _, d := range all {
		if a.matches(d) {
			filtered = append(filtered, d)
		}
	}
	ids := make([]string, len(filtered))
	for i, d := range filtered {
		ids[i] = d.ID
	}
	sig := strings.Join(ids, ",")

	a.mu.Lock()
	a.items = filtered
	a.mu.Unlock()

	if sig != a.rowSig {
		a.rowSig = sig
		a.model.PublishRowsReset()
	} else if len(filtered) > 0 {
		a.model.PublishRowsChanged(0, len(filtered)-1)
	}

	if next := buildCategoryTree(all); next.sig != a.treeMod.sig {
		a.treeMod.roots = next.roots
		a.treeMod.sig = next.sig
		a.treeMod.PublishItemsReset(nil)
	}

	active, speed := 0, int64(0)
	for _, d := range all {
		if d.Status == engine.StatusDownloading {
			active++
			speed += d.Speed
		}
	}
	if a.stat != nil {
		if active > 0 {
			a.stat.SetText(fmt.Sprintf("  %d active · %s/s · %d items", active, humanBytes(speed), len(all)))
		} else {
			a.stat.SetText(fmt.Sprintf("  %d items", len(all)))
		}
	}
}

func (a *application) matches(d engine.Info) bool {
	switch a.filter.kind {
	case "unfinished":
		return d.Status != engine.StatusComplete
	case "finished":
		return d.Status == engine.StatusComplete
	case "category":
		return d.Category == a.filter.value
	default:
		return true
	}
}

func (a *application) onCategoryChanged() {
	item, ok := a.tree.CurrentItem().(*catItem)
	if !ok || item == nil {
		return
	}
	a.filter.kind, a.filter.value = item.kind, item.value
	a.refresh()
}

func (a *application) onRowChanged() {
	if _, ok := a.currentDownload(); ok {
		return
	}
}

func (a *application) currentDownload() (*engine.Info, bool) {
	idx := a.tv.CurrentIndex()
	a.mu.Lock()
	defer a.mu.Unlock()
	if idx < 0 || idx >= len(a.items) {
		return nil, false
	}
	cp := a.items[idx]
	return &cp, true
}

/* ---------- actions ---------- */

func (a *application) resumeSelected() {
	if info, ok := a.currentDownload(); ok {
		a.mgr.Resume(info.ID)
	} else {
		a.mgr.ResumeAll()
	}
	a.refresh()
}

func (a *application) stopSelected() {
	if info, ok := a.currentDownload(); ok {
		a.mgr.Pause(info.ID)
	} else {
		a.mgr.PauseAll()
	}
	a.refresh()
}

func (a *application) toggleSelected() {
	if info, ok := a.currentDownload(); ok {
		if info.Status == engine.StatusDownloading || info.Status == engine.StatusQueued {
			a.mgr.Pause(info.ID)
		} else {
			a.mgr.Resume(info.ID)
		}
		a.refresh()
	}
}

func (a *application) deleteSelected() {
	info, ok := a.currentDownload()
	if !ok {
		return
	}
	yes := walk.MsgBox(a.mw, "Delete",
		fmt.Sprintf("Remove \"%s\" from the list? (file kept)", info.FileName),
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
	if yes {
		a.mgr.Remove(info.ID)
		a.refresh()
	}
}

func (a *application) removeSelected(withFile bool) {
	info, ok := a.currentDownload()
	if !ok {
		return
	}
	msg := fmt.Sprintf("Remove \"%s\" from the list? (file kept)", info.FileName)
	if withFile {
		msg = fmt.Sprintf("Delete \"%s\" from disk too?", info.FileName)
	}
	yes := walk.MsgBox(a.mw, "Delete", msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
	if yes {
		a.mgr.RemoveWithFile(info.ID, withFile)
		a.refresh()
	}
}
