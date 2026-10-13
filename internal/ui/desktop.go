//go:build fyne

// Package ui implements the ODM desktop interface with Fyne: a menu bar, an
// icon toolbar, a category/queue sidebar, a sortable report table and dialogs,
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

// sidebarEntry is one row of the category/queue sidebar.
type sidebarEntry struct {
	text   string
	kind   string
	value  string
	indent int
	icon   fyne.Resource
	header bool
}

type application struct {
	mgr     *engine.Manager
	fyneApp fyne.App
	win     fyne.Window

	table  *downloadTable
	cats   *widget.List
	split  *container.Split
	status *widget.Label

	mu          sync.Mutex
	items       []engine.Info
	entries     []sidebarEntry
	selectedRow int
	sortCol     int
	sortAsc     bool
	counts      map[string]int
	total       int
	unfinished  int
	finished    int
	queues      []string
	colWidths   []float32
	filter      struct {
		kind  string
		value string
	}

	progressWindows map[string]*progressView
}

// downloadTable keeps the columns filling the available width.
type downloadTable struct {
	*widget.Table
	app *application
}

func (t *downloadTable) Resize(size fyne.Size) {
	t.Table.Resize(size)
	t.app.layoutColumns(size.Width)
}

func (t *downloadTable) DoubleTapped(*fyne.PointEvent) {
	if d, ok := t.app.current(); ok {
		t.app.openProgressWindow(d.ID)
	}
}

func (t *downloadTable) TappedSecondary(e *fyne.PointEvent) {
	t.app.showRowMenu(e.AbsolutePosition)
}

var columnTitles = []string{"File Name", "Q", "Size", "Status", "Time left", "Transfer rate", "Last Try…", "Description"}

const (
	colName = iota
	colQueue
	colSize
	colStatus
	colETA
	colRate
	colLastTry
	colDescription
	columnCount
)

// fixed column widths; the name and description columns share the slack.
var columnFixed = map[int]float32{
	colQueue:   44,
	colSize:    80,
	colStatus:  112,
	colETA:     78,
	colRate:    100,
	colLastTry: 92,
}

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
			newToolButton(theme.MediaPlayIcon(), "Start Queue", a.startQueue),
			newToolButton(theme.MediaStopIcon(), "Stop Queue", a.stopQueue),
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
			return len(a.items), columnCount
		},
		func() fyne.CanvasObject { return newTableCell() },
		a.updateCell,
	)
	base.ShowHeaderColumn = false
	base.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 {
			a.selectedRow = id.Row
		}
	}
	base.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(a) }
	base.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := o.(*headerLabel)
		h.col = id.Col
		h.SetText(a.headerText(id.Col))
	}
	a.table = &downloadTable{Table: base, app: a}
	a.layoutColumns(760)

	a.cats = widget.NewList(
		func() int {
			a.mu.Lock()
			defer a.mu.Unlock()
			return len(a.entries)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.FolderIcon())
			label := widget.NewLabel("")
			return container.NewHBox(newIndentSpacer(), icon, label)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			a.mu.Lock()
			if id < 0 || id >= len(a.entries) {
				a.mu.Unlock()
				return
			}
			e := a.entries[id]
			a.mu.Unlock()
			box := o.(*fyne.Container)
			spacer := box.Objects[0].(*indentSpacer)
			icon := box.Objects[1].(*widget.Icon)
			label := box.Objects[2].(*widget.Label)

			spacer.set(float32(e.indent * 14))
			icon.Resource = e.icon
			icon.Refresh()
			if e.header {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
			label.SetText(e.text)
		},
	)
	a.cats.OnSelected = func(id widget.ListItemID) {
		a.mu.Lock()
		if id < 0 || id >= len(a.entries) {
			a.mu.Unlock()
			return
		}
		e := a.entries[id]
		a.mu.Unlock()
		if e.header {
			return
		}
		a.applyFilter(e.kind, e.value)
	}

	hideBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), a.toggleSidebar)
	hideBtn.Importance = widget.LowImportance
	sideHeader := container.NewBorder(nil, nil,
		container.NewPadded(widget.NewLabelWithStyle("Categories", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		hideBtn,
	)
	sidebar := container.NewBorder(sideHeader, nil, nil, nil, a.cats)

	a.split = container.NewHSplit(sidebar, a.table)
	a.split.Offset = 0.21

	a.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{})
	a.win.SetContent(container.NewBorder(top, container.NewPadded(a.status), nil, nil, a.split))
	a.win.SetMainMenu(a.buildMenu())
}

// layoutColumns sizes the table columns so they fill the available width.
func (a *application) layoutColumns(total float32) {
	if total <= 0 {
		return
	}
	var fixedSum float32
	for _, w := range columnFixed {
		fixedSum += w
	}
	slack := total - fixedSum
	nameW, descW := float32(200), float32(150)
	if slack > 360 {
		nameW = slack * 0.55
		descW = slack * 0.45
	} else if slack > 0 {
		nameW = slack * 0.6
		descW = slack * 0.4
	}
	widths := make([]float32, columnCount)
	for col, w := range columnFixed {
		widths[col] = w
	}
	widths[colName] = nameW
	widths[colDescription] = descW

	a.mu.Lock()
	if len(a.colWidths) == len(widths) {
		same := true
		for i := range widths {
			if a.colWidths[i] != widths[i] {
				same = false
				break
			}
		}
		if same {
			a.mu.Unlock()
			return
		}
	}
	a.colWidths = widths
	a.mu.Unlock()

	for i, w := range widths {
		a.table.SetColumnWidth(i, w)
	}
}

/* ---------- menu bar ---------- */

func (a *application) buildMenu() *fyne.MainMenu {
	tasks := fyne.NewMenu("Tasks",
		fyne.NewMenuItem("Add new download…", a.openAddDialog),
		fyne.NewMenuItem("Grab site…", a.openGrabberDialog),
		&fyne.MenuItem{IsSeparator: true},
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
		a.split.Offset = 0.21
	}
	a.split.Refresh()
}

func (a *application) buildEntries(all []engine.Info) []sidebarEntry {
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range all {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
	}
	folder := theme.FolderIcon()
	out := []sidebarEntry{
		{text: fmt.Sprintf("All Downloads (%d)", len(all)), kind: "all", icon: folder},
	}
	for _, c := range engine.Categories() {
		out = append(out, sidebarEntry{
			text: fmt.Sprintf("%s (%d)", c, counts[c]), kind: "category", value: c, indent: 1, icon: iconForCategory(c),
		})
	}
	out = append(out,
		sidebarEntry{text: fmt.Sprintf("Unfinished (%d)", unfinished), kind: "unfinished", icon: folder},
		sidebarEntry{text: fmt.Sprintf("Finished (%d)", finished), kind: "finished", icon: folder},
	)
	a.mu.Lock()
	queues := append([]string(nil), a.queues...)
	a.mu.Unlock()
	out = append(out, sidebarEntry{text: "Queues", kind: "", icon: folder})
	for _, q := range queues {
		out = append(out, sidebarEntry{text: q, kind: "queue", value: q, indent: 1, icon: folder})
	}
	return out
}

func (a *application) applyFilter(kind, value string) {
	a.mu.Lock()
	a.filter.kind, a.filter.value = kind, value
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
	c, ok := o.(*tableCell)
	if !ok {
		return
	}
	icon, lbl := c.icon, c.label

	a.mu.Lock()
	if id.Row < 0 || id.Row >= len(a.items) {
		a.mu.Unlock()
		lbl.SetText("")
		icon.Hide()
		return
	}
	d := a.items[id.Row]
	a.mu.Unlock()

	if id.Col == colName {
		icon.Resource = iconForCategory(d.Category)
		icon.Refresh()
		icon.Show()
	} else {
		icon.Hide()
	}

	switch id.Col {
	case colName:
		lbl.SetText(d.FileName)
	case colQueue:
		lbl.SetText(d.Queue)
	case colSize:
		lbl.SetText(humanBytes(d.Total))
	case colStatus:
		switch {
		case d.Error != "":
			lbl.SetText(d.Error)
		case d.Total > 0 && (d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued):
			lbl.SetText(fmt.Sprintf("%.2f%%", d.Progress))
		default:
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

// iconForCategory picks a small file-type icon for a category folder.
func iconForCategory(cat string) fyne.Resource {
	switch cat {
	case "Video":
		return theme.MediaVideoIcon()
	case "Audio":
		return theme.MediaMusicIcon()
	case "Documents":
		return theme.DocumentIcon()
	case "Programs":
		return theme.ComputerIcon()
	case "Compressed", "Disk Images":
		return theme.StorageIcon()
	case "Images":
		return theme.MediaPhotoIcon()
	default:
		return theme.FileIcon()
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

	entries := a.buildEntries(all)

	a.mu.Lock()
	a.entries = entries
	a.mu.Unlock()

	if a.table != nil {
		a.table.Refresh()
	}
	if a.cats != nil {
		a.cats.Refresh()
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
