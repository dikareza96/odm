//go:build fyne

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

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
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
	descEntry := widget.NewEntry()
	connEntry := widget.NewEntry()
	connEntry.SetText(fmt.Sprint(settings.Connections))
	queueSelect := widget.NewSelect(a.mgr.Queues(), nil)
	queueSelect.SetSelected(engine.DefaultQueue)
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

	browseButton := widget.NewButtonWithIcon("Browse…", theme.FolderOpenIcon(), func() {
		dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			folderEntry.SetText(dir.Path())
		}, a.win).Show()
	})
	folderRow := container.NewBorder(nil, nil, nil, browseButton, folderEntry)

	form := widget.NewForm(
		widget.NewFormItem("URL", urlEntry),
		widget.NewFormItem("Category", catSelect),
		widget.NewFormItem("Save folder", folderRow),
		widget.NewFormItem("File name", nameEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Connections", connEntry),
		widget.NewFormItem("Queue", queueSelect),
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
			Description: strings.TrimSpace(descEntry.Text),
			Connections: conns,
			Category:    catSelect.Selected,
			Queue:       queueSelect.Selected,
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
	dlg.Resize(fyne.NewSize(660, 520))
	dlg.Show()
}

/* ---------- Options (tabbed) ---------- */

func (a *application) openOptionsDialog() {
	settings := a.mgr.Settings()

	// General
	dirEntry := widget.NewEntry()
	dirEntry.SetText(settings.Dir)
	browse := widget.NewButtonWithIcon("Browse…", theme.FolderOpenIcon(), func() {
		dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			dirEntry.SetText(u.Path())
		}, a.win).Show()
	})
	dirRow := container.NewBorder(nil, nil, nil, browse, dirEntry)
	catsCheck := widget.NewCheck("Put files in category subfolders", nil)
	catsCheck.SetChecked(settings.Categories)
	clipCheck := widget.NewCheck("Watch clipboard for downloadable URLs", nil)
	clipCheck.SetChecked(settings.Clipboard)
	generalTab := container.NewVBox(widget.NewForm(widget.NewFormItem("Default folder", dirRow)), catsCheck, clipCheck)

	// Connection
	connEntry := widget.NewEntry()
	connEntry.SetText(fmt.Sprint(settings.Connections))
	concEntry := widget.NewEntry()
	concEntry.SetText(fmt.Sprint(settings.Concurrent))
	limitEntry := widget.NewEntry()
	limitEntry.SetText(fmt.Sprint(settings.SpeedLimit / 1024))
	connectionTab := container.NewVBox(widget.NewForm(
		widget.NewFormItem("Connections per file", connEntry),
		widget.NewFormItem("Simultaneous downloads", concEntry),
		widget.NewFormItem("Global speed limit (KB/s, 0 = off)", limitEntry),
	))

	// Proxy
	proxyCheck := widget.NewCheck("Use an HTTP proxy", nil)
	proxyCheck.SetChecked(settings.Proxy.Mode == "http")
	proxyHost := widget.NewEntry()
	proxyHost.SetText(settings.Proxy.Host)
	proxyPort := widget.NewEntry()
	proxyPort.SetText(fmt.Sprint(settings.Proxy.Port))
	proxyUser := widget.NewEntry()
	proxyUser.SetText(settings.Proxy.User)
	proxyPass := widget.NewEntry()
	proxyPass.SetText(settings.Proxy.Pass)
	proxyTab := container.NewVBox(proxyCheck, widget.NewForm(
		widget.NewFormItem("Host", proxyHost),
		widget.NewFormItem("Port", proxyPort),
		widget.NewFormItem("User", proxyUser),
		widget.NewFormItem("Password", proxyPass),
	))

	// Sites logins
	logins := append([]engine.Login(nil), settings.Logins...)
	loginsTable := widget.NewTable(
		func() (int, int) { return len(logins), 3 },
		func() fyne.CanvasObject { return widget.NewEntry() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			e := o.(*widget.Entry)
			if id.Row < 0 || id.Row >= len(logins) {
				e.SetText("")
				return
			}
			switch id.Col {
			case 0:
				e.SetText(logins[id.Row].Host)
			case 1:
				e.SetText(logins[id.Row].User)
			case 2:
				e.SetText(logins[id.Row].Pass)
			}
			row, col := id.Row, id.Col
			e.OnChanged = func(v string) {
				if row >= len(logins) {
					return
				}
				switch col {
				case 0:
					logins[row].Host = v
				case 1:
					logins[row].User = v
				case 2:
					logins[row].Pass = v
				}
			}
		},
	)
	loginsTable.SetColumnWidth(0, 200)
	loginsTable.SetColumnWidth(1, 140)
	loginsTable.SetColumnWidth(2, 140)
	selected := -1
	loginsTable.OnSelected = func(id widget.TableCellID) { selected = id.Row }
	addLogin := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		logins = append(logins, engine.Login{})
		loginsTable.Refresh()
	})
	delLogin := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), func() {
		if selected >= 0 && selected < len(logins) {
			logins = append(logins[:selected], logins[selected+1:]...)
			selected = -1
			loginsTable.Refresh()
		}
	})
	loginsTab := container.NewBorder(
		container.NewHBox(addLogin, delLogin),
		nil, nil, nil,
		loginsTable,
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("General", theme.SettingsIcon(), generalTab),
		container.NewTabItemWithIcon("Connection", theme.MediaPlayIcon(), connectionTab),
		container.NewTabItemWithIcon("Proxy", theme.ComputerIcon(), proxyTab),
		container.NewTabItemWithIcon("Sites Logins", theme.LoginIcon(), loginsTab),
	)

	dlg := dialog.NewCustomWithoutButtons("Options", tabs, a.win)
	save := func() {
		conns, conc, limit := settings.Connections, settings.Concurrent, settings.SpeedLimit/1024
		fmt.Sscanf(connEntry.Text, "%d", &conns)
		fmt.Sscanf(concEntry.Text, "%d", &conc)
		fmt.Sscanf(limitEntry.Text, "%d", &limit)
		port := 8080
		fmt.Sscanf(proxyPort.Text, "%d", &port)
		proxy := engine.ProxyConfig{Mode: "none"}
		if proxyCheck.Checked {
			proxy = engine.ProxyConfig{
				Mode: "http",
				Host: strings.TrimSpace(proxyHost.Text),
				Port: port,
				User: strings.TrimSpace(proxyUser.Text),
				Pass: proxyPass.Text,
			}
		}
		clean := make([]engine.Login, 0, len(logins))
		for _, l := range logins {
			if strings.TrimSpace(l.Host) != "" {
				clean = append(clean, l)
			}
		}
		a.mgr.UpdateSettings(engine.Settings{
			Dir:         strings.TrimSpace(dirEntry.Text),
			Connections: conns,
			Concurrent:  conc,
			SpeedLimit:  limit * 1024,
			Categories:  catsCheck.Checked,
			Clipboard:   clipCheck.Checked,
			Proxy:       proxy,
			Logins:      clean,
			Queues:      settings.Queues,
		})
		dlg.Hide()
		a.refresh()
	}
	closeBtn := widget.NewButton("Close", func() { dlg.Hide() })
	saveBtn := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), save)
	saveBtn.Importance = widget.HighImportance
	dlg.SetButtons([]fyne.CanvasObject{closeBtn, saveBtn})
	dlg.Resize(fyne.NewSize(620, 460))
	dlg.Show()
}

/* ---------- Site Grabber ---------- */

func (a *application) openGrabberDialog() {
	settings := a.mgr.Settings()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://site.example/downloads/")
	depthEntry := widget.NewEntry()
	depthEntry.SetText("1")
	pagesEntry := widget.NewEntry()
	pagesEntry.SetText("50")
	filesEntry := widget.NewEntry()
	filesEntry.SetText("100")
	sameHost := widget.NewCheck("Stay on the same host", nil)
	sameHost.SetChecked(true)
	startNow := widget.NewCheck("Start downloads immediately", nil)
	startNow.SetChecked(true)
	info := widget.NewLabel("")

	form := widget.NewForm(
		widget.NewFormItem("Start page", urlEntry),
		widget.NewFormItem("Depth", depthEntry),
		widget.NewFormItem("Max pages", pagesEntry),
		widget.NewFormItem("Max files", filesEntry),
	)
	content := container.NewVBox(form, sameHost, startNow, info)

	dlg := dialog.NewCustomWithoutButtons("ODM Site Grabber", content, a.win)
	start := func() {
		depth, pages, files := 1, 50, 100
		fmt.Sscanf(depthEntry.Text, "%d", &depth)
		fmt.Sscanf(pagesEntry.Text, "%d", &pages)
		fmt.Sscanf(filesEntry.Text, "%d", &files)
		opts := engine.GrabberOptions{
			URL:      strings.TrimSpace(urlEntry.Text),
			Depth:    depth,
			MaxPages: pages,
			MaxFiles: files,
			SameHost: sameHost.Checked,
			Dir:      settings.Dir,
			Start:    startNow.Checked,
		}
		if opts.URL == "" {
			info.SetText("Enter a start page.")
			return
		}
		info.SetText("Exploring…")
		go func() {
			res, err := a.mgr.Grab(context.Background(), opts, func(p, f int) {
				fyne.Do(func() { info.SetText(fmt.Sprintf("Exploring… %d pages, %d files", p, f)) })
			})
			fyne.Do(func() {
				if err != nil {
					info.SetText("Failed: " + err.Error())
					return
				}
				info.SetText(fmt.Sprintf("Done: %d pages explored, %d files queued.", res.Pages, len(res.Files)))
				a.refresh()
			})
		}()
	}
	startBtn := widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), start)
	startBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton("Close", func() { dlg.Hide() })
	dlg.SetButtons([]fyne.CanvasObject{closeBtn, startBtn})
	dlg.Resize(fyne.NewSize(560, 420))
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
	strip      *chunkStrip
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
	v.win.Resize(fyne.NewSize(740, 560))

	v.url = widget.NewLabel("")
	v.status = widget.NewLabel("")
	v.size = widget.NewLabel("")
	v.done = widget.NewLabel("")
	v.rate = widget.NewLabel("")
	v.eta = widget.NewLabel("")
	v.resume = widget.NewLabel("")
	v.bar = widget.NewProgressBar()
	v.bar.TextFormatter = func() string { return fmt.Sprintf("%.0f%%", v.bar.Value*100) }
	v.strip = newChunkStrip()

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
	v.conns.SetColumnWidth(2, 400)

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
			v.strip,
			widget.NewLabelWithStyle("Start positions and download progress by connections",
				fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
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
	v.strip.set(d.Chunks, d.Total)
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
