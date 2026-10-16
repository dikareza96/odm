//go:build fyne

package ui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestMenuBarSwitchesOnHover(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(newTheme())
	w := app.NewWindow("t")
	ran := ""
	menus := func() *fyne.MainMenu {
		return fyne.NewMainMenu(
			fyne.NewMenu("Tasks", fyne.NewMenuItem("Add", func() { ran = "add" })),
			fyne.NewMenu("Downloads", fyne.NewMenuItem("Pause all", func() { ran = "pause" })),
			fyne.NewMenu("Help", fyne.NewMenuItem("About", func() { ran = "about" })),
		)
	}
	w.SetContent(newMenuBar(w, menus))
	w.Resize(fyne.NewSize(600, 300))
	var bar *menuBar
	var find func(o fyne.CanvasObject)
	find = func(o fyne.CanvasObject) {
		if mt, ok := o.(*menuTitle); ok && bar == nil {
			bar = mt.bar
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, k := range c.Objects {
				find(k)
			}
		}
	}
	find(w.Content())
	if bar == nil || len(bar.titles) != 3 {
		t.Fatal("menu bar not built")
	}
	overlays := func() int { return len(w.Canvas().Overlays().List()) }
	centre := func(mt *menuTitle) fyne.Position {
		p := app.Driver().AbsolutePositionForObject(mt)
		return p.Add(fyne.NewPos(mt.Size().Width/2, mt.Size().Height/2))
	}

	bar.titles[0].Tapped(nil)
	if bar.open != bar.titles[0] || overlays() != 1 {
		t.Fatalf("click did not open Tasks (open=%v overlays=%d)", bar.open, overlays())
	}
	// Pointing at another title while one is open switches to it.
	bar.sess.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{AbsolutePosition: centre(bar.titles[1])}})
	if bar.open != bar.titles[1] || overlays() != 1 {
		t.Fatalf("hover did not switch to Downloads (open=%v overlays=%d)", bar.open, overlays())
	}
	if out := os.Getenv("ODM_SNAPSHOT"); out != "" {
		f, _ := os.Create(out)
		png.Encode(f, w.Canvas().Capture())
		f.Close()
	}
	// Right moves to the next menu, wrapping at the end.
	bar.sess.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	bar.sess.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if bar.open != bar.titles[0] {
		t.Fatalf("right arrow: open=%v", bar.open)
	}
	bar.sess.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if bar.open != nil || overlays() != 0 {
		t.Fatalf("escape left a menu open (overlays=%d)", overlays())
	}
	// Choosing an item closes the menu and runs it.
	bar.titles[1].Tapped(nil)
	bar.sess.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	bar.sess.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	if ran != "pause" || bar.open != nil || overlays() != 0 {
		t.Fatalf("item: ran=%q open=%v overlays=%d", ran, bar.open, overlays())
	}
	// A click on the open title, or elsewhere, closes.
	bar.titles[2].Tapped(nil)
	bar.sess.Tapped(&fyne.PointEvent{AbsolutePosition: centre(bar.titles[2])})
	if bar.open != nil {
		t.Fatal("click on the open title did not close")
	}
	bar.titles[2].Tapped(nil)
	bar.sess.Tapped(&fyne.PointEvent{AbsolutePosition: fyne.NewPos(400, 250)})
	if bar.open != nil || overlays() != 0 {
		t.Fatal("click elsewhere did not close")
	}
}
