//go:build windows

package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
	"odm/internal/osutil"
)

func openPath(p string)   { _ = osutil.Open(p) }
func revealPath(p string) { _ = osutil.Reveal(p) }

func dialogConfirm(parent fyne.Window, title, msg string, fn func(bool)) {
	dialog.NewConfirm(title, msg, fn, parent).Show()
}

/* ---------- Add: "Download File Info" ---------- */

func (a *application) openAddDialog() {
	settings := a.mgr.Settings()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://host/file.zip")
	catSelect := widget.NewSelect(engine.Categories(), nil)
	catSelect.SetSelected("Other")
	folderEntry := widget.NewEntry()
	folderEntry.SetText(settings.Dir)
	nameEntry := widget.NewEntry()
	connEntry := widget.NewEntry()
	connEntry.SetText(fmt.Sprint(settings.Connections))
	sizeLabel := widget.NewLabel("Size: —")
	typeLabel := widget.NewLabel("")

	var probeTimer *time.Timer
	urlEntry.OnChanged = func(s string) {
		if probeTimer != nil {
			probeTimer.Stop()
		}
		if !strings.HasPrefix(s, "http") {
			return
		}
		probeTimer = time.AfterFunc(600*time.Millisecond, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			p, err := a.mgr.Probe(ctx, s, nil)
			fyne.Do(func() {
				if err != nil {
					sizeLabel.SetText("Size: —")
					typeLabel.SetText(err.Error())
					return
				}
				sizeLabel.SetText("Size: " + humanBytes(p.Total))
				typeLabel.SetText(p.ContentType)
				if nameEntry.Text == "" {
					nameEntry.SetText(p.FileName)
				}
				catSelect.SetSelected(engine.Category(p.FileName))
			})
		})
	}

	form := widget.NewForm(
		widget.NewFormItem("URL", urlEntry),
		widget.NewFormItem("Category", catSelect),
		widget.NewFormItem("Save folder", folderEntry),
		widget.NewFormItem("File name", nameEntry),
		widget.NewFormItem("Connections", connEntry),
	)
	content := container.NewVBox(form, sizeLabel, typeLabel)

	dlg := dialog.NewCustomWithoutButtons("Download File Info", content, a.win)

	create := func(start bool) {
		conns := settings.Connections
		fmt.Sscanf(connEntry.Text, "%d", &conns)
		url := strings.TrimSpace(urlEntry.Text)
		if url == "" {
			return
		}
		a.mgr.Create(engine.AddOptions{
			URL:         url,
			Dir:         strings.TrimSpace(folderEntry.Text),
			FileName:    strings.TrimSpace(nameEntry.Text),
			Connections: conns,
			Category:    catSelect.Selected,
			Start:       start,
		})
		dlg.Hide()
		a.refresh()
	}

	startBtn := widget.NewButtonWithIcon("Start Download", theme.ConfirmIcon(), func() { create(true) })
	startBtn.Importance = widget.HighImportance
	laterBtn := widget.NewButtonWithIcon("Download Later", theme.ContentAddIcon(), func() { create(false) })
	cancelBtn := widget.NewButton("Cancel", func() { dlg.Hide() })
	dlg.SetButtons([]fyne.CanvasObject{laterBtn, cancelBtn, startBtn})
	dlg.Resize(fyne.NewSize(620, 420))
	dlg.Show()
}

/* ---------- Options ---------- */

func (a *application) openOptionsDialog() {
	settings := a.mgr.Settings()

	dirEntry := widget.NewEntry()
	dirEntry.SetText(settings.Dir)
	connEntry := widget.NewEntry()
	connEntry.SetText(fmt.Sprint(settings.Connections))
	concEntry := widget.NewEntry()
	concEntry.SetText(fmt.Sprint(settings.Concurrent))
	limitEntry := widget.NewEntry()
	limitEntry.SetText(fmt.Sprint(settings.SpeedLimit / 1024))
	catsCheck := widget.NewCheck("Put files in category subfolders", nil)
	catsCheck.SetChecked(settings.Categories)
	clipCheck := widget.NewCheck("Watch clipboard for downloadable URLs", nil)
	clipCheck.SetChecked(settings.Clipboard)

	form := widget.NewForm(
		widget.NewFormItem("Default folder", dirEntry),
		widget.NewFormItem("Connections per file", connEntry),
		widget.NewFormItem("Simultaneous downloads", concEntry),
		widget.NewFormItem("Global speed limit (KB/s, 0 = off)", limitEntry),
	)
	content := container.NewVBox(form, catsCheck, clipCheck)

	dlg := dialog.NewCustomWithoutButtons("Options", content, a.win)
	save := func() {
		conns, conc, limit := settings.Connections, settings.Concurrent, settings.SpeedLimit/1024
		fmt.Sscanf(connEntry.Text, "%d", &conns)
		fmt.Sscanf(concEntry.Text, "%d", &conc)
		fmt.Sscanf(limitEntry.Text, "%d", &limit)
		a.mgr.UpdateSettings(engine.Settings{
			Dir:         strings.TrimSpace(dirEntry.Text),
			Connections: conns,
			Concurrent:  conc,
			SpeedLimit:  limit * 1024,
			Categories:  catsCheck.Checked,
			Clipboard:   clipCheck.Checked,
		})
		dlg.Hide()
		a.refresh()
	}
	closeBtn := widget.NewButton("Close", func() { dlg.Hide() })
	saveBtn := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), save)
	saveBtn.Importance = widget.HighImportance
	dlg.SetButtons([]fyne.CanvasObject{closeBtn, saveBtn})
	dlg.Resize(fyne.NewSize(560, 380))
	dlg.Show()
}

/* ---------- Progress window ---------- */

type progressView struct {
	win        fyne.Window
	url        *widget.Label
	status     *widget.Label
	size       *widget.Label
	done       *widget.Label
	rate       *widget.Label
	eta        *widget.Label
	resume     *widget.Label
	bar        *widget.ProgressBar
	conns      *widget.Table
	connRows   []engine.ChunkInfo
	connMu     sync.Mutex
	pauseBtn   *widget.Button
	limitEntry *widget.Entry
	openCheck  *widget.Check
}

func (a *application) openProgressWindow(id string) {
	if v, ok := a.progressWindows[id]; ok {
		v.win.Show()
		v.win.RequestFocus()
		a.updateProgressWindow(id)
		return
	}
	if _, ok := a.byID(id); !ok {
		return
	}

	v := &progressView{}
	v.win = a.fyneApp.NewWindow("Download")
	v.win.Resize(fyne.NewSize(720, 540))

	v.url = widget.NewLabel("")
	v.status = widget.NewLabel("")
	v.size = widget.NewLabel("")
	v.done = widget.NewLabel("")
	v.rate = widget.NewLabel("")
	v.eta = widget.NewLabel("")
	v.resume = widget.NewLabel("")
	v.bar = widget.NewProgressBar()
	v.bar.TextFormatter = func() string { return fmt.Sprintf("%.0f%%", v.bar.Value*100) }

	v.conns = widget.NewTable(
		func() (int, int) {
			v.connMu.Lock()
			defer v.connMu.Unlock()
			return len(v.connRows), 3
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(cell widget.TableCellID, o fyne.CanvasObject) {
			v.connMu.Lock()
			defer v.connMu.Unlock()
			lbl := o.(*widget.Label)
			if cell.Row < 0 || cell.Row >= len(v.connRows) {
				lbl.SetText("")
				return
			}
			c := v.connRows[cell.Row]
			switch cell.Col {
			case 0:
				lbl.SetText(fmt.Sprint(cell.Row + 1))
			case 1:
				lbl.SetText(humanBytes(c.Done))
			case 2:
				lbl.SetText(c.State)
			}
		},
	)
	v.conns.SetColumnWidth(0, 50)
	v.conns.SetColumnWidth(1, 200)
	v.conns.SetColumnWidth(2, 380)

	statusTab := container.NewBorder(
		container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("URL", v.url),
				widget.NewFormItem("Status", v.status),
				widget.NewFormItem("File size", v.size),
				widget.NewFormItem("Downloaded", v.done),
				widget.NewFormItem("Transfer rate", v.rate),
				widget.NewFormItem("Time left", v.eta),
				widget.NewFormItem("Resume capability", v.resume),
			),
			v.bar,
		),
		nil, nil, nil,
		v.conns,
	)

	v.limitEntry = widget.NewEntry()
	limitTab := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Limit (KB/s, 0 = unlimited)", v.limitEntry)),
		widget.NewButtonWithIcon("Apply", theme.ConfirmIcon(), func() {
			limit := 0
			fmt.Sscanf(v.limitEntry.Text, "%d", &limit)
			a.mgr.SetSpeedLimit(id, int64(limit)*1024)
		}),
	)

	v.openCheck = widget.NewCheck("Open file when done", func(b bool) { a.mgr.SetOpenWhenDone(id, b) })
	compTab := container.NewVBox(v.openCheck)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Download status", theme.InfoIcon(), statusTab),
		container.NewTabItemWithIcon("Speed Limiter", theme.MediaPauseIcon(), limitTab),
		container.NewTabItemWithIcon("Options on completion", theme.SettingsIcon(), compTab),
	)

	v.pauseBtn = widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if d, ok := a.mgr.Get(id); ok {
			if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
				a.mgr.Pause(id)
			} else {
				a.mgr.Resume(id)
			}
		}
	})
	closeBtn := widget.NewButton("Close", func() { v.win.Close() })

	w := v.win
	w.SetContent(container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), v.pauseBtn, closeBtn), nil, nil, tabs))
	w.SetOnClosed(func() { delete(a.progressWindows, id) })

	a.progressWindows[id] = v
	a.updateProgressWindow(id)
	w.Show()
}

func (a *application) updateProgressWindow(id string) {
	v, ok := a.progressWindows[id]
	if !ok {
		return
	}
	d, ok := a.mgr.Get(id)
	if !ok {
		return
	}
	v.win.SetTitle(fmt.Sprintf("%.0f%% %s", d.Progress, d.FileName))
	v.url.SetText(d.URL)
	status := string(d.Status)
	if d.Status == engine.StatusDownloading {
		status = "Receiving data…"
	}
	if d.Error != "" {
		status += " — " + d.Error
	}
	v.status.SetText(status)
	v.size.SetText(dash(humanBytes(d.Total)))
	v.done.SetText(fmt.Sprintf("%s (%.2f %%)", humanBytes(d.Done), d.Progress))
	v.rate.SetText(dash(rateText(d)))
	v.eta.SetText(dash(etaText(d)))
	if d.RangeOK {
		v.resume.SetText("Yes")
	} else {
		v.resume.SetText("No")
	}
	if d.Total > 0 {
		v.bar.SetValue(d.Progress / 100)
	}
	if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
		v.pauseBtn.SetText("Pause")
	} else {
		v.pauseBtn.SetText("Resume")
	}
	if v.limitEntry.Text == "" && d.SpeedLimit > 0 {
		v.limitEntry.SetText(fmt.Sprint(d.SpeedLimit / 1024))
	}
	v.openCheck.SetChecked(d.OpenWhenDone)

	v.connMu.Lock()
	v.connRows = d.Chunks
	v.connMu.Unlock()
	v.conns.Refresh()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
