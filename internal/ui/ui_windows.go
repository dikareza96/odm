//go:build windows

// Package ui implements the ODM desktop interface with Fyne: a themed window
// with an icon toolbar, a category sidebar and a report table, driving the
// engine in-process (no web server, no embedded browser).
package ui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
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
	}
	a.fyneApp = app.NewWithID("com.ochi.odm")
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("Ochi Download Manager")
	a.win.Resize(fyne.NewSize(1040, 640))
	a.win.SetIcon(theme.DownloadIcon())

	a.build()
	a.refresh()
	go a.loop()
	a.win.ShowAndRun()
	return nil
}

type catEntry struct {
	text  string
	kind  string
	value string
}

type application struct {
	mgr     *engine.Manager
	fyneApp fyne.App
	win     fyne.Window

	table  *downloadTable
	cats   *widget.List
	status *widget.Label

	mu          sync.Mutex
	items       []engine.Info
	catEntries  []catEntry
	selectedRow int
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

var columnTitles = []string{"File Name", "Size", "Progress", "Status", "Time left", "Transfer rate", "Path"}

const (
	colName = iota
	colSize
	colProgress
	colStatus
	colETA
	colRate
	colPath
)

func (a *application) build() {
	top := container.NewVBox(
		container.NewPadded(container.NewHBox(
			iconButton("Add URL", theme.ContentAddIcon(), a.openAddDialog),
			widget.NewSeparator(),
			iconButton("Resume", theme.MediaPlayIcon(), a.resumeSelected),
			iconButton("Stop", theme.MediaPauseIcon(), a.stopSelected),
			iconButton("Stop All", theme.MediaStopIcon(), func() { a.mgr.PauseAll(); a.refresh() }),
			widget.NewSeparator(),
			iconButton("Delete", theme.ContentRemoveIcon(), a.deleteSelected),
			iconButton("Del Completed", theme.ConfirmIcon(), func() { a.mgr.ClearCompleted(); a.refresh() }),
			iconButton("Options", theme.SettingsIcon(), a.openOptionsDialog),
			widget.NewSeparator(),
			iconButton("Progress", theme.InfoIcon(), func() {
				if d, ok := a.current(); ok {
					a.openProgressWindow(d.ID)
				}
			}),
			iconButton("Open folder", theme.FolderOpenIcon(), func() {
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
			return container.NewMax(widget.NewLabel(""), widget.NewProgressBar())
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
		l := widget.NewLabel("")
		l.TextStyle = fyne.TextStyle{Bold: true}
		return l
	}
	base.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col >= 0 && id.Col < len(columnTitles) {
			o.(*widget.Label).SetText(columnTitles[id.Col])
		}
	}
	for i, w := range []float32{260, 90, 150, 100, 100, 120, 240} {
		base.SetColumnWidth(i, w)
	}
	a.table = &downloadTable{Table: base, app: a}

	a.cats = widget.NewList(
		func() int {
			a.mu.Lock()
			defer a.mu.Unlock()
			return len(a.catEntries)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			a.mu.Lock()
			defer a.mu.Unlock()
			if id >= 0 && id < len(a.catEntries) {
				o.(*widget.Label).SetText(a.catEntries[id].text)
			}
		},
	)
	a.cats.OnSelected = func(id widget.ListItemID) {
		a.mu.Lock()
		if id >= 0 && id < len(a.catEntries) {
			a.filter.kind = a.catEntries[id].kind
			a.filter.value = a.catEntries[id].value
		}
		a.mu.Unlock()
		a.refresh()
	}

	a.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{})

	catHeader := widget.NewLabelWithStyle("Categories", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sidebar := container.NewBorder(container.NewPadded(catHeader), nil, nil, nil, a.cats)
	split := container.NewHSplit(sidebar, a.table)
	split.Offset = 0.22

	footer := container.NewPadded(a.status)
	a.win.SetContent(container.NewBorder(top, footer, nil, nil, split))
}

func (a *application) updateCell(id widget.TableCellID, o fyne.CanvasObject) {
	box := o.(*fyne.Container)
	lbl := box.Objects[0].(*widget.Label)
	bar := box.Objects[1].(*widget.ProgressBar)
	bar.TextFormatter = func() string { return fmt.Sprintf("%.0f%%", bar.Value*100) }

	a.mu.Lock()
	if id.Row < 0 || id.Row >= len(a.items) {
		a.mu.Unlock()
		lbl.SetText("")
		lbl.Show()
		bar.Hide()
		return
	}
	d := a.items[id.Row]
	a.mu.Unlock()

	if id.Col == colProgress {
		lbl.Hide()
		bar.Show()
		val := 0.0
		if d.Total > 0 {
			val = d.Progress / 100
		}
		bar.SetValue(val)
		return
	}
	bar.Hide()
	lbl.Show()
	switch id.Col {
	case colName:
		lbl.SetText(d.FileName)
	case colSize:
		lbl.SetText(humanBytes(d.Total))
	case colStatus:
		if d.Error != "" {
			lbl.SetText(d.Error)
		} else {
			lbl.SetText(string(d.Status))
		}
	case colETA:
		lbl.SetText(etaText(d))
	case colRate:
		lbl.SetText(rateText(d))
	case colPath:
		lbl.SetText(d.Dir)
	}
}

func (a *application) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fyne.Do(a.refresh)
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

	a.mu.Lock()
	a.items = filtered
	a.catEntries = buildCategoryEntries(all)
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

func (a *application) matches(d engine.Info) bool {
	a.mu.Lock()
	kind, value := a.filter.kind, a.filter.value
	a.mu.Unlock()
	switch kind {
	case "unfinished":
		return d.Status != engine.StatusComplete
	case "finished":
		return d.Status == engine.StatusComplete
	case "category":
		return d.Category == value
	default:
		return true
	}
}

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
		fyne.NewMenuItem("Remove from list", func() { a.removeSelected(false) }),
		fyne.NewMenuItem("Remove + delete file", func() { a.removeSelected(true) }),
	)
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), a.win.Canvas())
	pop.ShowAtPosition(pos)
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

func iconButton(label string, icon fyne.Resource, fn func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, icon, fn)
	b.Importance = widget.MediumImportance
	return b
}

func buildCategoryEntries(items []engine.Info) []catEntry {
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range items {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
	}
	out := []catEntry{{text: fmt.Sprintf("All Downloads (%d)", len(items)), kind: "all"}}
	out = append(out, catEntry{text: "— Categories —"})
	for _, name := range engine.Categories() {
		out = append(out, catEntry{text: fmt.Sprintf("  %s (%d)", name, counts[name]), kind: "category", value: name})
	}
	out = append(out, catEntry{text: "— Status —"})
	out = append(out,
		catEntry{text: fmt.Sprintf("  Unfinished (%d)", unfinished), kind: "unfinished"},
		catEntry{text: fmt.Sprintf("  Finished (%d)", finished), kind: "finished"},
	)
	return out
}

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

func etaText(d engine.Info) string {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 || d.Total <= 0 {
		return ""
	}
	secs := float64(d.Total-d.Done) / float64(d.Speed)
	m, s := int(secs)/60, int(secs)%60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
