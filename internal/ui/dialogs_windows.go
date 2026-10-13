//go:build windows

package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"odm/internal/engine"
)

/* ---------- connection model (progress window) ---------- */

type connModel struct {
	walk.TableModelBase
	mu   sync.Mutex
	rows []engine.ChunkInfo
}

func (m *connModel) RowCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rows)
}

func (m *connModel) Value(row, col int) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	c := m.rows[row]
	switch col {
	case 0:
		return row + 1
	case 1:
		return humanBytes(c.Done)
	case 2:
		return c.State
	}
	return ""
}

func (m *connModel) set(rows []engine.ChunkInfo) {
	m.mu.Lock()
	m.rows = rows
	m.mu.Unlock()
	m.PublishRowsReset()
}

/* ---------- Add: "Download File Info" ---------- */

func (a *application) openAddDialog() {
	settings := a.mgr.Settings()

	var dlg *walk.Dialog
	var urlEdit, folderEdit, nameEdit *walk.LineEdit
	var connEdit *walk.NumberEdit
	var catBox *walk.ComboBox
	var sizeLabel, typeLabel *walk.Label
	var startBtn, laterBtn, cancelBtn *walk.PushButton

	start := true

	d := Dialog{
		AssignTo:      &dlg,
		Title:         "Download File Info",
		Size:          Size{Width: 660, Height: 460},
		Layout:        VBox{},
		DefaultButton: &startBtn,
		CancelButton:  &cancelBtn,
		Children: []Widget{
			Label{Text: "URL"},
			LineEdit{AssignTo: &urlEdit},
			Label{Text: "Category"},
			ComboBox{AssignTo: &catBox, Model: engine.Categories()},
			Label{Text: "Save folder"},
			LineEdit{AssignTo: &folderEdit, Text: settings.Dir},
			Label{Text: "File name"},
			LineEdit{AssignTo: &nameEdit},
			Label{Text: "Connections"},
			NumberEdit{AssignTo: &connEdit, Decimals: 0, MinValue: 1, MaxValue: 32, Value: float64(settings.Connections)},
			Label{AssignTo: &sizeLabel, Text: "Size: —"},
			Label{AssignTo: &typeLabel, Text: ""},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &laterBtn, Text: "Download Later", OnClicked: func() { start = false; dlg.Accept() }},
					PushButton{AssignTo: &cancelBtn, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
					PushButton{AssignTo: &startBtn, Text: "Start Download", OnClicked: func() { start = true; dlg.Accept() }},
				},
			},
		},
	}
	if err := d.Create(a.mw); err != nil {
		return
	}

	urlEdit.EditingFinished().Attach(func() {
		raw := urlEdit.Text()
		if len(raw) < 8 {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			p, err := a.mgr.Probe(ctx, raw, nil)
			dlg.Synchronize(func() {
				if dlg.IsDisposed() {
					return
				}
				if err != nil {
					sizeLabel.SetText("Size: —")
					typeLabel.SetText(err.Error())
					return
				}
				sizeLabel.SetText("Size: " + humanBytes(p.Total))
				typeLabel.SetText(p.ContentType)
				if nameEdit.Text() == "" {
					nameEdit.SetText(p.FileName)
				}
				for i, c := range engine.Categories() {
					if c == engine.Category(p.FileName) {
						_ = catBox.SetCurrentIndex(i)
						break
					}
				}
			})
		}()
	})

	if dlg.Run() != walk.DlgCmdOK {
		return
	}
	url := urlEdit.Text()
	if len(url) == 0 {
		return
	}
	cat := ""
	if i := catBox.CurrentIndex(); i >= 0 && i < len(engine.Categories()) {
		cat = engine.Categories()[i]
	}
	a.mgr.Create(engine.AddOptions{
		URL:         url,
		Dir:         folderEdit.Text(),
		FileName:    nameEdit.Text(),
		Connections: int(connEdit.Value()),
		Category:    cat,
		Start:       start,
	})
	a.refresh()
}

/* ---------- Options ---------- */

func (a *application) openOptionsDialog() {
	settings := a.mgr.Settings()

	var dlg *walk.Dialog
	var dirEdit *walk.LineEdit
	var connEdit, concEdit, limitEdit *walk.NumberEdit
	var catsCheck, clipCheck *walk.CheckBox
	var saveBtn, closeBtn *walk.PushButton

	d := Dialog{
		AssignTo:      &dlg,
		Title:         "Options",
		Size:          Size{Width: 560, Height: 420},
		Layout:        VBox{},
		DefaultButton: &saveBtn,
		CancelButton:  &closeBtn,
		Children: []Widget{
			Label{Text: "Default folder"},
			LineEdit{AssignTo: &dirEdit, Text: settings.Dir},
			Label{Text: "Connections per file"},
			NumberEdit{AssignTo: &connEdit, Decimals: 0, MinValue: 1, MaxValue: 32, Value: float64(settings.Connections)},
			Label{Text: "Simultaneous downloads"},
			NumberEdit{AssignTo: &concEdit, Decimals: 0, MinValue: 1, MaxValue: 16, Value: float64(settings.Concurrent)},
			Label{Text: "Global speed limit (KB/s, 0 = off)"},
			NumberEdit{AssignTo: &limitEdit, Decimals: 0, MinValue: 0, MaxValue: 10000000, Value: float64(settings.SpeedLimit / 1024)},
			CheckBox{AssignTo: &catsCheck, Text: "Put files in category subfolders", Checked: settings.Categories},
			CheckBox{AssignTo: &clipCheck, Text: "Watch clipboard for downloadable URLs (Windows)", Checked: settings.Clipboard},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &closeBtn, Text: "Close", OnClicked: func() { dlg.Cancel() }},
					PushButton{AssignTo: &saveBtn, Text: "Save", OnClicked: func() { dlg.Accept() }},
				},
			},
		},
	}
	if err := d.Create(a.mw); err != nil {
		return
	}
	if dlg.Run() != walk.DlgCmdOK {
		return
	}
	a.mgr.UpdateSettings(engine.Settings{
		Dir:         dirEdit.Text(),
		Connections: int(connEdit.Value()),
		Concurrent:  int(concEdit.Value()),
		SpeedLimit:  int64(limitEdit.Value()) * 1024,
		Categories:  catsCheck.Checked(),
		Clipboard:   clipCheck.Checked(),
	})
	a.refresh()
}

/* ---------- Progress window ---------- */

func (a *application) openProgressDialog() {
	info, ok := a.currentDownload()
	if !ok {
		return
	}
	id := info.ID

	var dlg *walk.Dialog
	var urlLabel, statusLabel, sizeLabel, doneLabel, rateLabel, etaLabel, resumeLabel *walk.Label
	var bar *walk.ProgressBar
	var conns *walk.TableView
	var limitEdit *walk.NumberEdit
	var openCheck *walk.CheckBox
	var pauseBtn, applyBtn, closeBtn *walk.PushButton

	model := &connModel{}

	d := Dialog{
		AssignTo:     &dlg,
		Title:        "Download",
		Size:         Size{Width: 700, Height: 560},
		Layout:       VBox{},
		CancelButton: &closeBtn,
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					{
						Title:  "Download status",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "URL"}, Label{AssignTo: &urlLabel, Text: ""},
							Label{Text: "Status"}, Label{AssignTo: &statusLabel, Text: ""},
							Label{Text: "File size"}, Label{AssignTo: &sizeLabel, Text: ""},
							Label{Text: "Downloaded"}, Label{AssignTo: &doneLabel, Text: ""},
							Label{Text: "Transfer rate"}, Label{AssignTo: &rateLabel, Text: ""},
							Label{Text: "Time left"}, Label{AssignTo: &etaLabel, Text: ""},
							Label{Text: "Resume capability"}, Label{AssignTo: &resumeLabel, Text: ""},
							ProgressBar{AssignTo: &bar, MaxValue: 1000},
							TableView{
								AssignTo:         &conns,
								AlternatingRowBG: true,
								Columns: []TableViewColumn{
									{Title: "N", Width: 40},
									{Title: "Downloaded", Width: 140, Alignment: AlignFar},
									{Title: "Info", Width: 320},
								},
								Model:   model,
								MinSize: Size{Height: 120},
							},
						},
					},
					{
						Title:  "Speed Limiter",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "Limit (KB/s, 0 = unlimited)"},
							NumberEdit{AssignTo: &limitEdit, Decimals: 0, MinValue: 0, MaxValue: 10000000},
							PushButton{AssignTo: &applyBtn, Text: "Apply"},
						},
					},
					{
						Title:  "Options on completion",
						Layout: VBox{},
						Children: []Widget{
							CheckBox{AssignTo: &openCheck, Text: "Open file when done"},
						},
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &pauseBtn, Text: "Pause"},
					PushButton{AssignTo: &closeBtn, Text: "Close", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}
	if err := d.Create(a.mw); err != nil {
		return
	}

	pauseBtn.Clicked().Attach(func() {
		if cur, ok := a.mgr.Get(id); ok {
			if cur.Status == engine.StatusDownloading || cur.Status == engine.StatusQueued {
				a.mgr.Pause(id)
			} else {
				a.mgr.Resume(id)
			}
		}
	})
	applyBtn.Clicked().Attach(func() {
		a.mgr.SetSpeedLimit(id, int64(limitEdit.Value())*1024)
	})
	openCheck.CheckedChanged().Attach(func() {
		a.mgr.SetOpenWhenDone(id, openCheck.Checked())
	})

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				cur, ok := a.mgr.Get(id)
				if !ok {
					continue
				}
				dlg.Synchronize(func() {
					if dlg.IsDisposed() {
						return
					}
					renderProgress(cur, dlg, model, urlLabel, statusLabel, sizeLabel, doneLabel, rateLabel, etaLabel, resumeLabel, bar, pauseBtn, limitEdit, openCheck)
				})
			}
		}
	}()

	dlg.Run()
	close(stop)
	a.refresh()
}

func renderProgress(
	d engine.Info,
	dlg *walk.Dialog,
	model *connModel,
	urlLabel, statusLabel, sizeLabel, doneLabel, rateLabel, etaLabel, resumeLabel *walk.Label,
	bar *walk.ProgressBar,
	pauseBtn *walk.PushButton,
	limitEdit *walk.NumberEdit,
	openCheck *walk.CheckBox,
) {
	_ = dlg.SetTitle(fmt.Sprintf("%.0f%% %s", d.Progress, d.FileName))
	urlLabel.SetText(d.URL)
	status := string(d.Status)
	if d.Status == engine.StatusDownloading {
		status = "Receiving data…"
	}
	if d.Error != "" {
		status += " — " + d.Error
	}
	statusLabel.SetText(status)
	sizeLabel.SetText(textOrDash(humanBytes(d.Total)))
	doneLabel.SetText(fmt.Sprintf("%s (%.2f %%)", humanBytes(d.Done), d.Progress))
	rateLabel.SetText(textOrDash(rateText(d)))
	etaLabel.SetText(textOrDash(etaText(d)))
	if d.RangeOK {
		resumeLabel.SetText("Yes")
	} else {
		resumeLabel.SetText("No")
	}
	bar.SetValue(int(d.Progress * 10))

	model.set(d.Chunks)

	if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
		pauseBtn.SetText("Pause")
	} else {
		pauseBtn.SetText("Resume")
	}
	if limitEdit.Value() == 0 && d.SpeedLimit > 0 {
		limitEdit.SetValue(float64(d.SpeedLimit / 1024))
	}
	openCheck.SetChecked(d.OpenWhenDone)
}

func textOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
