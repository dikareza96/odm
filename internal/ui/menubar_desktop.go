//go:build fyne

package ui

import (
	"image/color"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The window's menu bar is drawn by ODM rather than Fyne on Windows and
// Linux. Fyne's own bar takes its spacing from the theme's padding, which
// ODM keeps tight for the list and toolbar, so its titles sat cramped in a
// thin strip; it cannot be styled on its own. macOS keeps the system menu
// bar at the top of the screen (useNativeMenu).
//
// A menu opens on a click; while one is open, pointing at another title
// switches to it, as in a native bar. That needs ODM's own overlay
// (menuSession): Fyne's pop-up menus cover the window with an invisible
// layer that would keep the pointer from ever reaching the bar. Keys:
// up/down/enter/escape inside the menu, left/right to the menu beside.

// useNativeMenu reports whether the platform shows the menu bar itself.
func useNativeMenu() bool { return runtime.GOOS == "darwin" }

// Menu bar geometry, in Fyne units.
const (
	menuBarHeight = 26
	menuTitlePadX = 10
	menuMinWidth  = 210
)

// newMenuBar builds the bar from menus, which is called on every opening
// so labels can follow state ("Hide categories" / "Show categories").
func newMenuBar(win fyne.Window, menus func() *fyne.MainMenu) fyne.CanvasObject {
	bar := &menuBar{win: win, menus: menus}
	titles := container.NewHBox()
	for i, m := range menus().Items {
		t := newMenuTitle(bar, i, m.Label)
		bar.titles = append(bar.titles, t)
		titles.Add(t)
	}
	bg := canvas.NewRectangle(odmPalette.surface)
	line := canvas.NewRectangle(odmPalette.border)
	line.SetMinSize(fyne.NewSize(0, 1))
	row := container.New(layout.NewCustomPaddedLayout(0, 0, 4, 4), titles)
	return container.NewStack(bg, container.NewBorder(nil, line, nil, nil, row))
}

type menuBar struct {
	win    fyne.Window
	menus  func() *fyne.MainMenu
	titles []*menuTitle
	open   *menuTitle   // the title whose menu is showing, nil for none
	sess   *menuSession // its overlay
}

// show opens t's menu, replacing one that is open.
func (b *menuBar) show(t *menuTitle) {
	items := b.menus().Items
	if t.index >= len(items) {
		return
	}
	b.close()
	c := b.win.Canvas()
	m := widget.NewMenu(items[t.index])
	s := &menuSession{bar: b, title: t, menu: m}
	s.ExtendBaseWidget(s)
	// Roomier items than the window's tight theme; the override is kept
	// per object, so it applies though the menu lives in an overlay.
	container.NewThemeOverride(s, menuTheme{Theme: newTheme()})
	// An item that is chosen dismisses its menu, then runs its action.
	m.OnDismiss = func() {
		if b.sess == s {
			b.close()
		}
	}
	b.open, b.sess = t, s
	t.Refresh()
	c.Overlays().Add(s)
	s.Resize(c.Size())
	c.Focus(s)
}

// close hides the open menu, if any.
func (b *menuBar) close() {
	if b.sess == nil {
		return
	}
	s, t := b.sess, b.open
	b.sess, b.open = nil, nil
	b.win.Canvas().Overlays().Remove(s)
	t.Refresh()
}

// titleAt returns the title under an absolute position, or nil.
func (b *menuBar) titleAt(p fyne.Position) *menuTitle {
	d := fyne.CurrentApp().Driver()
	for _, t := range b.titles {
		o := d.AbsolutePositionForObject(t)
		sz := t.Size()
		if p.X >= o.X && p.X < o.X+sz.Width && p.Y >= o.Y && p.Y < o.Y+sz.Height {
			return t
		}
	}
	return nil
}

// step opens the menu dir titles away (left -1, right +1), wrapping.
func (b *menuBar) step(dir int) {
	if b.open == nil || len(b.titles) == 0 {
		return
	}
	n := len(b.titles)
	b.show(b.titles[(b.open.index+dir+n)%n])
}

// menuSession is the overlay of one open menu: it covers the window, holds
// the menu under its title, and takes the pointer everywhere else: over
// another title it switches menus, a click elsewhere closes.
type menuSession struct {
	widget.BaseWidget
	bar   *menuBar
	title *menuTitle
	menu  *widget.Menu
}

func (s *menuSession) Tapped(e *fyne.PointEvent) {
	if t := s.bar.titleAt(e.AbsolutePosition); t != nil && t != s.title {
		s.bar.show(t)
		return
	}
	s.bar.close() // elsewhere, or the open title itself
}

func (s *menuSession) TappedSecondary(*fyne.PointEvent) { s.bar.close() }

func (s *menuSession) MouseIn(*desktop.MouseEvent) {}
func (s *menuSession) MouseOut()                   {}
func (s *menuSession) MouseMoved(e *desktop.MouseEvent) {
	if t := s.bar.titleAt(e.AbsolutePosition); t != nil && t != s.title {
		s.bar.show(t)
	}
}

func (s *menuSession) FocusGained()   {}
func (s *menuSession) FocusLost()     {}
func (s *menuSession) TypedRune(rune) {}
func (s *menuSession) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyDown:
		s.menu.ActivateNext()
	case fyne.KeyUp:
		s.menu.ActivatePrevious()
	case fyne.KeyEnter, fyne.KeyReturn, fyne.KeySpace:
		s.menu.TriggerLast()
	case fyne.KeyEscape:
		s.bar.close()
	case fyne.KeyLeft:
		s.bar.step(-1)
	case fyne.KeyRight:
		s.bar.step(1)
	}
}

func (s *menuSession) CreateRenderer() fyne.WidgetRenderer {
	return &menuSessionRenderer{s: s}
}

type menuSessionRenderer struct{ s *menuSession }

// Layout puts the menu under its title, at least menuMinWidth wide (so a
// short menu does not look like a tooltip), kept inside the window.
func (r *menuSessionRenderer) Layout(size fyne.Size) {
	m := r.s.menu
	ms := m.MinSize()
	ms.Width = max(ms.Width, menuMinWidth)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(r.s.title)
	pos.Y += r.s.title.Size().Height
	if pos.X+ms.Width > size.Width {
		pos.X = max(0, size.Width-ms.Width)
	}
	m.Move(pos)
	m.Resize(ms)
}

func (r *menuSessionRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *menuSessionRenderer) Refresh()                     { r.Layout(r.s.Size()); r.s.menu.Refresh() }
func (r *menuSessionRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.s.menu} }
func (r *menuSessionRenderer) Destroy()                     {}

// menuTheme styles the drop-down menus: more space around each item, a
// lifted background, and the accent for the item under the pointer.
type menuTheme struct {
	fyne.Theme
}

func (t menuTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 12.5
	case theme.SizeNameMenuRadius:
		return 8
	}
	return t.Theme.Size(name)
}

func (t menuTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return odmPalette.surfaceAlt
	case theme.ColorNameHover, theme.ColorNameFocus:
		return odmPalette.accentSoft
	case theme.ColorNameSeparator:
		return odmPalette.border
	case theme.ColorNameShadow:
		// Darker than the window's, so the menu stands off the toolbar.
		return color.NRGBA{A: 0xB0}
	}
	return t.Theme.Color(name, v)
}

// menuTitle is one clickable title in the bar.
type menuTitle struct {
	widget.BaseWidget
	bar     *menuBar
	index   int
	label   string
	hovered bool
}

func newMenuTitle(bar *menuBar, index int, label string) *menuTitle {
	t := &menuTitle{bar: bar, index: index, label: label}
	t.ExtendBaseWidget(t)
	return t
}

func (t *menuTitle) Tapped(*fyne.PointEvent) { t.bar.show(t) }

func (t *menuTitle) MouseIn(*desktop.MouseEvent)    { t.hovered = true; t.Refresh() }
func (t *menuTitle) MouseMoved(*desktop.MouseEvent) {}
func (t *menuTitle) MouseOut()                      { t.hovered = false; t.Refresh() }

func (t *menuTitle) Cursor() desktop.Cursor { return desktop.DefaultCursor }

func (t *menuTitle) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(odmPalette.hover)
	bg.CornerRadius = 4
	text := canvas.NewText(t.label, odmPalette.foreground)
	text.TextSize = theme.TextSize() + 0.5
	r := &menuTitleRenderer{t: t, bg: bg, text: text}
	r.Refresh()
	return r
}

type menuTitleRenderer struct {
	t    *menuTitle
	bg   *canvas.Rectangle
	text *canvas.Text
}

func (r *menuTitleRenderer) MinSize() fyne.Size {
	ts := r.text.MinSize()
	return fyne.NewSize(ts.Width+2*menuTitlePadX, menuBarHeight)
}

func (r *menuTitleRenderer) Layout(size fyne.Size) {
	// The highlight sits inside the bar, clear of its bottom line.
	r.bg.Move(fyne.NewPos(0, 3))
	r.bg.Resize(fyne.NewSize(size.Width, size.Height-6))
	ts := r.text.MinSize()
	r.text.Move(fyne.NewPos((size.Width-ts.Width)/2, (size.Height-ts.Height)/2))
	r.text.Resize(ts)
}

func (r *menuTitleRenderer) Refresh() {
	switch {
	case r.t.bar.open == r.t:
		r.bg.FillColor = odmPalette.accentSoft
		r.bg.Show()
	case r.t.hovered:
		r.bg.FillColor = odmPalette.hover
		r.bg.Show()
	default:
		r.bg.Hide()
	}
	r.text.Text = r.t.label
	r.bg.Refresh()
	r.text.Refresh()
	r.Layout(r.t.Size())
}

func (r *menuTitleRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.text} }

func (r *menuTitleRenderer) Destroy() {}
