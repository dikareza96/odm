//go:build windows

// Package ui implements the ODM desktop interface with Fyne: a menu bar, an
// icon toolbar, a category/queue tree, a sortable report table and dialogs,
// driving the engine in-process (no web server, no embedded browser).
package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// Run starts the desktop UI and blocks until the window closes.
func Run(mgr *engine.Manager) error {
	a := &application{
		mgr:             mgr,
		progressWindows: map[string]*progressView{},
		selectedRow:     -1,
		sortCol:         -1,
	}
	a.fyneApp = app.NewWithID("com.ochi.odm")
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("Ochi Download Manager")
	a.win.Resize(fyne.NewSize(1080, 660))
	a.win.SetIcon(theme.DownloadIcon())

	a.build()
	a.refresh()
	go a.loop()
	a.win.ShowAndRun()
	return nil
}

type application struct {
	mgr     *engine.Manager
	fyneApp fyne.App
	win     fyne.Window

	table  *downloadTable
	tree   *widget.Tree
	split  *container.Split
	status *widget.Label

	mu          sync.Mutex
	items       []engine.Info
	selectedRow int
	sortCol     int
	sortAsc     bool
	counts      map[string]int
	total       int
	unfinished  int
	finished    int
	queues      []string
	filter      struct {
		kind  string
		value string
	}

	progressWindows map[string]*progressView
}

// downloadTable adds a double-click and a right-click menu to the report table.
type downloadTable struct {
	*widget.Table
	app *application
}

func (t *downloadTable) DoubleTapped(*fyne.PointEvent) {
	if d, ok := t.app.current(); ok {
		t.app.openProgressWindow(d.ID)
	}
}

func (t *downloadTable) TappedSecondary(e *fyne.PointEvent) {
	t.app.showRowMenu(e.AbsolutePosition)
}

// headerButton is a clickable column header that toggles sorting.
type headerButton struct {
	*widget.Button
	app *application
	col int
}

var columnTitles = []string{"Q", "File Name", "Size", "Status", "Time left", "Transfer rate", "Last Try…", "Description"}

const (
	colQueue = iota
	colName
	colSize
	colStatus
	colETA
	colRate
	colLastTry
	colDescription
)

func (a *application) build() {
	top := container.NewVBox(
		container.NewPadded(container.NewHBox(
			newToolButton(theme.ContentAddIcon(), "Add URL", a.openAddDialog),
			newToolButton(theme.MediaPlayIcon(), "Resume", a.resumeSelected),
			newToolButton(theme.MediaPauseIcon(), "Stop", a.stopSelected),
			newToolButton(theme.MediaStopIcon(), "Stop All", func() { a.mgr.PauseAll(); a.refresh() }),
			newToolButton(theme.ContentRemoveIcon(), "Delete", a.deleteSelected),
			newToolButton(theme.ConfirmIcon(), "Del Completed", func() { a.mgr.ClearCompleted(); a.refresh() }),
			newToolButton(theme.SettingsIcon(), "Options", a.openOptionsDialog),
			widget.NewSeparator(),
			newToolButton(theme.MediaPlayIcon(), "Start Queue", func() { a.startQueue() }),
			newToolButton(theme.MediaStopIcon(), "Stop Queue", func() { a.stopQueue() }),
			widget.NewSeparator(),
			newToolButton(theme.InfoIcon(), "Progress", func() {
				if d, ok := a.current(); ok {
					a.openProgressWindow(d.ID)
				}
			}),
			newToolButton(theme.FolderOpenIcon(), "Open folder", func() {
				if d, ok := a.current(); ok {
					revealPath(d.FilePath)
				}
			}),
		)),
		widget.NewSeparator(),
	)

	base := widget.NewTableWithHeaders(
		func() (int, int) {
			a.mu.Lock()
			defer a.mu.Unlock()
			return len(a.items), len(columnTitles)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewLabel(""), newMiniBar())
		},
		a.updateCell,
	)
	base.ShowHeaderColumn = false
	base.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 {
			a.selectedRow = id.Row
		}
	}
	base.CreateHeader = func() fyne.CanvasObject {
		b := widget.NewButton("", nil)
		b.Importance = widget.LowImportance
		return &headerButton{Button: b}
	}
	base.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		hb := o.(*headerButton)
		hb.app = a
		hb.col = id.Col
		hb.SetText(a.headerText(id.Col))
		hb.OnTapped = func() { a.sortBy(hb.col) }
	}
	for i, w := range []float32{50, 240, 90, 120, 95, 115, 130, 250} {
		base.SetColumnWidth(i, w)
	}
	a.table = &downloadTable{Table: base, app: a}

	a.tree = widget.NewTree(
		a.treeChildren,
		a.treeIsBranch,
		func(branch bool) fyne.CanvasObject {
			icon := widget.NewIcon(theme.FolderIcon())
			if !branch {
				icon.Hide()
			}
			return container.NewHBox(icon, widget.NewLabel(""))
		},
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			icon := box.Objects[0].(*widget.Icon)
			lbl := box.Objects[1].(*widget.Label)
			if branch {
				icon.Show()
			} else {
				icon.Hide()
			}
			lbl.SetText(a.treeLabel(uid))
		},
	)
	a.tree.OnSelected = func(uid widget.TreeNodeID) { a.selectNode(string(uid)) }
	a.tree.OpenAllBranches()

	hideBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), a.toggleSidebar)
	hideBtn.Importance = widget.LowImportance
	catHeader := container.NewBorder(nil, nil,
		container.NewPadded(widget.NewLabelWithStyle("Categories", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		hideBtn,
	)
	sidebar := container.NewBorder(catHeader, nil, nil, nil, a.tree)

	a.split = container.NewHSplit(sidebar, a.table)
	a.split.Offset = 0.22

	a.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{})
	a.win.SetContent(container.NewBorder(top, container.NewPadded(a.status), nil, nil, a.split))
	a.win.SetMainMenu(a.buildMenu())
}

/* ---------- menu bar ---------- */

func (a *application) buildMenu() *fyne.MainMenu {
	sep := &fyne.MenuItem{IsSeparator: true}

	tasks := fyne.NewMenu("Tasks",
		fyne.NewMenuItem("Add new download…", a.openAddDialog),
		fyne.NewMenuItem("Grab site…", a.openGrabberDialog),
		sep,
		fyne.NewMenuItem("Export download list…", a.exportList),
		fyne.NewMenuItem("Import download list…", a.importList),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Exit", func() { a.fyneApp.Quit() }),
	)
	downloads := fyne.NewMenu("Downloads",
		fyne.NewMenuItem("Pause all", func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem("Stop all", func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem("Clear completed", func() { a.mgr.ClearCompleted(); a.refresh() }),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Start queue", a.startQueue),
		fyne.NewMenuItem("Stop queue", a.stopQueue),
		fyne.NewMenuItem("New queue…", a.openNewQueueDialog),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Options…", a.openOptionsDialog),
	)
	view := fyne.NewMenu("View",
		fyne.NewMenuItem("Hide categories", a.toggleSidebar),
		fyne.NewMenuItem("Sort by name", func() { a.sortBy(colName) }),
		fyne.NewMenuItem("Sort by size", func() { a.sortBy(colSize) }),
		fyne.NewMenuItem("Sort by status", func() { a.sortBy(colStatus) }),
		fyne.NewMenuItem("Sort by time left", func() { a.sortBy(colETA) }),
		fyne.NewMenuItem("Sort by transfer rate", func() { a.sortBy(colRate) }),
		fyne.NewMenuItem("Sort by description", func() { a.sortBy(colDescription) }),
	)
	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("About ODM", a.showAbout),
	)
	return fyne.NewMainMenu(tasks, downloads, view, help)
}

func (a *application) showAbout() {
	dialog.ShowInformation("About",
		"Ochi Download Manager (ODM)\n\nA segmented, resumable download manager.\nBuilt with Go and Fyne.",
		a.win)
}

func (a *application) exportList() {
	data, err := a.mgr.Export()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()
		if _, err := w.Write(data); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	d.SetFileName("odm-downloads.json")
	d.Show()
}

func (a *application) importList() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		n, err := a.mgr.Import(data)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refresh()
		dialog.ShowInformation("Import", fmt.Sprintf("Imported %d download(s).", n), a.win)
	}, a.win)
	d.Show()
}

/* ---------- sidebar ---------- */

func (a *application) toggleSidebar() {
	if a.split.Offset > 0.001 {
		a.split.Offset = 0
	} else {
		a.split.Offset = 0.22
	}
	a.split.Refresh()
}

func (a *application) treeChildren(uid widget.TreeNodeID) []widget.TreeNodeID {
	s := string(uid)
	switch {
	case s == "":
		return []widget.TreeNodeID{"all", "unfinished", "finished", "queues"}
	case s == "all":
		out := make([]widget.TreeNodeID, 0, len(engine.Categories()))
		for _, c := range engine.Categories() {
			out = append(out, widget.TreeNodeID("cat:"+c))
		}
		return out
	case s == "queues":
		a.mu.Lock()
		defer a.mu.Unlock()
		out := make([]widget.TreeNodeID, 0, len(a.queues))
		for _, q := range a.queues {
			out = append(out, widget.TreeNodeID("queue:"+q))
		}
		return out
	}
	return nil
}

func (a *application) treeIsBranch(uid widget.TreeNodeID) bool {
	s := string(uid)
	return s == "all" || s == "queues" || strings.HasPrefix(s, "cat:")
}

func (a *application) treeLabel(uid widget.TreeNodeID) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := string(uid)
	switch {
	case s == "all":
		return fmt.Sprintf("All Downloads (%d)", a.total)
	case strings.HasPrefix(s, "cat:"):
		name := strings.TrimPrefix(s, "cat:")
		return fmt.Sprintf("%s (%d)", name, a.counts[name])
	case s == "unfinished":
		return fmt.Sprintf("Unfinished (%d)", a.unfinished)
	case s == "finished":
		return fmt.Sprintf("Finished (%d)", a.finished)
	case s == "queues":
		return "Queues"
	case strings.HasPrefix(s, "queue:"):
		return strings.TrimPrefix(s, "queue:")
	}
	return s
}

func (a *application) selectNode(uid string) {
	a.mu.Lock()
	switch {
	case strings.HasPrefix(uid, "cat:"):
		a.filter.kind, a.filter.value = "category", strings.TrimPrefix(uid, "cat:")
	case strings.HasPrefix(uid, "queue:"):
		a.filter.kind, a.filter.value = "queue", strings.TrimPrefix(uid, "queue:")
	case uid == "unfinished":
		a.filter.kind, a.filter.value = "unfinished", ""
	case uid == "finished":
		a.filter.kind, a.filter.value = "finished", ""
	default:
		a.filter.kind, a.filter.value = "all", ""
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *application) currentQueueFilter() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.filter.kind == "queue" {
		return a.filter.value
	}
	return "*"
}

/* ---------- queues ---------- */

func (a *application) startQueue() {
	a.mgr.StartQueue(a.currentQueueFilter())
	a.refresh()
}

func (a *application) stopQueue() {
	a.mgr.StopQueue(a.currentQueueFilter())
	a.refresh()
}

func (a *application) openNewQueueDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Queue name")
	d := dialog.NewCustomConfirm("New queue", "Create", "Cancel",
		container.NewVBox(widget.NewLabel("Enter a name for the new queue:"), entry),
		func(ok bool) {
			if !ok || strings.TrimSpace(entry.Text) == "" {
				return
			}
			if !a.mgr.AddQueue(entry.Text) {
				dialog.ShowInformation("Queue", "That queue already exists.", a.win)
			}
			a.refresh()
		}, a.win)
	d.Show()
}

/* ---------- table ---------- */

func (a *application) updateCell(id widget.TableCellID, o fyne.CanvasObject) {
	box := o.(*fyne.Container)
	lbl := box.Objects[0].(*widget.Label)
	bar := box.Objects[1].(*miniBar)

	a.mu.Lock()
	if id.Row < 0 || id.Row >= len(a.items) {
		a.mu.Unlock()
		lbl.SetText("")
		bar.Hide()
		return
	}
	d := a.items[id.Row]
	a.mu.Unlock()

	if id.Col == colName && d.Total > 0 {
		bar.Show()
		bar.set(d.Progress / 100)
	} else {
		bar.Hide()
	}

	switch id.Col {
	case colQueue:
		lbl.SetText(d.Queue)
	case colName:
		lbl.SetText(d.FileName)
	case colSize:
		lbl.SetText(humanBytes(d.Total))
	case colStatus:
		if d.Error != "" {
			lbl.SetText(d.Error)
		} else if d.Total > 0 && d.Status == engine.StatusDownloading {
			lbl.SetText(fmt.Sprintf("%s %.0f%%", d.Status, d.Progress))
		} else {
			lbl.SetText(string(d.Status))
		}
	case colETA:
		lbl.SetText(etaText(d))
	case colRate:
		lbl.SetText(rateText(d))
	case colLastTry:
		lbl.SetText(d.UpdatedAt.Format("15:04:05"))
	case colDescription:
		if d.Description != "" {
			lbl.SetText(d.Description)
		} else {
			lbl.SetText(d.URL)
		}
	}
}

func (a *application) headerText(col int) string {
	if col < 0 || col >= len(columnTitles) {
		return ""
	}
	a.mu.Lock()
	sc, asc := a.sortCol, a.sortAsc
	a.mu.Unlock()
	if sc == col {
		if asc {
			return columnTitles[col] + " ▲"
		}
		return columnTitles[col] + " ▼"
	}
	return columnTitles[col]
}

func (a *application) sortBy(col int) {
	a.mu.Lock()
	if a.sortCol == col {
		a.sortAsc = !a.sortAsc
	} else {
		a.sortCol = col
		a.sortAsc = true
	}
	a.mu.Unlock()
	a.refresh()
}

/* ---------- refresh ---------- */

func (a *application) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fyne.Do(a.refresh)
	}
}

func (a *application) refresh() {
	a.mu.Lock()
	col, asc := a.sortCol, a.sortAsc
	kind, value := a.filter.kind, a.filter.value
	a.mu.Unlock()

	all := a.mgr.List()
	queues := a.mgr.Queues()
	filtered := make([]engine.Info, 0, len(all))
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range all {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
		if matchesFilter(d, kind, value) {
			filtered = append(filtered, d)
		}
	}
	if col >= 0 {
		sortSlice(filtered, col, asc)
	}

	a.mu.Lock()
	a.items = filtered
	a.counts = counts
	a.queues = queues
	a.total = len(all)
	a.unfinished = unfinished
	a.finished = finished
	a.mu.Unlock()

	if a.table != nil {
		a.table.Refresh()
	}
	if a.tree != nil {
		a.tree.Refresh()
	}

	active, speed := 0, int64(0)
	for _, d := range all {
		if d.Status == engine.StatusDownloading {
			active++
			speed += d.Speed
		}
	}
	if a.status != nil {
		if active > 0 {
			a.status.SetText(fmt.Sprintf("  %d active · %s/s · %d items", active, humanBytes(speed), len(all)))
		} else {
			a.status.SetText(fmt.Sprintf("  %d items", len(all)))
		}
	}

	for id, v := range a.progressWindows {
		if v.win == nil {
			continue
		}
		a.updateProgressWindow(id)
	}
}

func sortSlice(items []engine.Info, col int, asc bool) {
	less := func(i, j int) bool { return compareInfo(items[i], items[j], col) }
	sort.SliceStable(items, func(i, j int) bool {
		if asc {
			return less(i, j)
		}
		return less(j, i)
	})
}

func compareInfo(x, y engine.Info, col int) bool {
	switch col {
	case colQueue:
		return strings.ToLower(x.Queue) < strings.ToLower(y.Queue)
	case colName:
		return strings.ToLower(x.FileName) < strings.ToLower(y.FileName)
	case colSize:
		return x.Total < y.Total
	case colStatus:
		return x.Status < y.Status
	case colETA:
		return etaSeconds(x) < etaSeconds(y)
	case colRate:
		return x.Speed < y.Speed
	case colLastTry:
		return x.UpdatedAt.Before(y.UpdatedAt)
	case colDescription:
		return strings.ToLower(x.Description+x.URL) < strings.ToLower(y.Description+y.URL)
	}
	return false
}

func matchesFilter(d engine.Info, kind, value string) bool {
	switch kind {
	case "unfinished":
		return d.Status != engine.StatusComplete
	case "finished":
		return d.Status == engine.StatusComplete
	case "category":
		return d.Category == value
	case "queue":
		q := d.Queue
		if q == "" {
			q = engine.DefaultQueue
		}
		return strings.EqualFold(q, value)
	default:
		return true
	}
}

/* ---------- selection and actions ---------- */

func (a *application) current() (*engine.Info, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := a.selectedRow
	if idx < 0 || idx >= len(a.items) {
		return nil, false
	}
	cp := a.items[idx]
	return &cp, true
}

func (a *application) byID(id string) (*engine.Info, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.items {
		if a.items[i].ID == id {
			cp := a.items[i]
			return &cp, true
		}
	}
	return nil, false
}

func (a *application) showRowMenu(pos fyne.Position) {
	d, ok := a.current()
	if !ok {
		return
	}
	items := []*fyne.MenuItem{fyne.NewMenuItem("Show progress", func() { a.openProgressWindow(d.ID) })}
	if d.Status == engine.StatusComplete {
		items = append(items, fyne.NewMenuItem("Open file", func() { openPath(d.FilePath) }))
	}
	items = append(items,
		fyne.NewMenuItem("Open folder", func() { revealPath(d.FilePath) }),
		fyne.NewMenuItem("Pause / Resume", func() {
			if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
				a.mgr.Pause(d.ID)
			} else {
				a.mgr.Resume(d.ID)
			}
			a.refresh()
		}),
	)
	move := fyne.NewMenuItem("Move to queue", nil)
	move.ChildMenu = a.queueSubmenu(d.ID)
	items = append(items, move)
	items = append(items,
		fyne.NewMenuItem("Remove from list", func() { a.removeSelected(false) }),
		fyne.NewMenuItem("Remove + delete file", func() { a.removeSelected(true) }),
	)
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), a.win.Canvas())
	pop.ShowAtPosition(pos)
}

func (a *application) queueSubmenu(id string) *fyne.Menu {
	out := []*fyne.MenuItem{}
	for _, q := range a.mgr.Queues() {
		q := q
		out = append(out, fyne.NewMenuItem(q, func() {
			a.mgr.MoveToQueue(id, q)
			a.refresh()
		}))
	}
	if len(out) == 0 {
		out = append(out, &fyne.MenuItem{Label: "(no queues)", Disabled: true})
	}
	return fyne.NewMenu("", out...)
}

func (a *application) resumeSelected() {
	if d, ok := a.current(); ok {
		a.mgr.Resume(d.ID)
	} else {
		a.mgr.ResumeAll()
	}
	a.refresh()
}

func (a *application) stopSelected() {
	if d, ok := a.current(); ok {
		a.mgr.Pause(d.ID)
	} else {
		a.mgr.PauseAll()
	}
	a.refresh()
}

func (a *application) deleteSelected() {
	d, ok := a.current()
	if !ok {
		return
	}
	dialogConfirm(a.win, "Delete", fmt.Sprintf("Remove \"%s\" from the list? (file kept)", d.FileName), func(yes bool) {
		if yes {
			a.mgr.Remove(d.ID)
			a.refresh()
		}
	})
}

func (a *application) removeSelected(withFile bool) {
	d, ok := a.current()
	if !ok {
		return
	}
	msg := fmt.Sprintf("Remove \"%s\" from the list? (file kept)", d.FileName)
	if withFile {
		msg = fmt.Sprintf("Delete \"%s\" from disk too?", d.FileName)
	}
	dialogConfirm(a.win, "Delete", msg, func(yes bool) {
		if yes {
			a.mgr.RemoveWithFile(d.ID, withFile)
			a.refresh()
		}
	})
}

/* ---------- formatting ---------- */

func humanBytes(n int64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func rateText(d engine.Info) string {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 {
		return ""
	}
	return humanBytes(d.Speed) + "/s"
}

func etaSeconds(d engine.Info) float64 {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 || d.Total <= 0 {
		return -1
	}
	return float64(d.Total-d.Done) / float64(d.Speed)
}

func etaText(d engine.Info) string {
	secs := etaSeconds(d)
	if secs < 0 {
		return ""
	}
	m, s := int(secs)/60, int(secs)%60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
