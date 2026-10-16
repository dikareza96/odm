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
// Gotcha: while a Fyne pop-up menu is open it covers the window with an
// invisible overlay, so pointing at another title does not switch menus as
// in a native bar: a click closes the open menu, a second click opens the
// other.

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
		titles.Add(newMenuTitle(bar, i, m.Label))
	}
	bg := canvas.NewRectangle(odmPalette.surface)
	line := canvas.NewRectangle(odmPalette.border)
	line.SetMinSize(fyne.NewSize(0, 1))
	row := container.New(layout.NewCustomPaddedLayout(0, 0, 4, 4), titles)
	return container.NewStack(bg, container.NewBorder(nil, line, nil, nil, row))
}

type menuBar struct {
	win   fyne.Window
	menus func() *fyne.MainMenu
	open  *menuTitle // the title whose menu is showing, nil for none
}

// show opens menu i under title t.
func (b *menuBar) show(t *menuTitle, i int) {
	items := b.menus().Items
	if i >= len(items) {
		return
	}
	c := b.win.Canvas()
	pop := widget.NewPopUpMenu(items[i], c)
	// Roomier items than the window's tight theme; the override is kept
	// per object, so it applies though the menu lives in an overlay.
	container.NewThemeOverride(pop, menuTheme{Theme: newTheme()})
	// The override changed the item sizes; a minimum width keeps short
	// menus (Help) from looking like a tooltip.
	pop.Resize(fyne.NewSize(max(pop.MinSize().Width, menuMinWidth), pop.MinSize().Height))
	dismiss := pop.OnDismiss
	pop.OnDismiss = func() {
		if b.open == t {
			b.open = nil
			t.Refresh()
		}
		if dismiss != nil {
			dismiss()
		}
	}
	b.open = t
	t.Refresh()
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(t)
	pop.ShowAtPosition(pos.Add(fyne.NewPos(0, t.Size().Height)))
}

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

func (t *menuTitle) Tapped(*fyne.PointEvent) { t.bar.show(t, t.index) }

func (t *menuTitle) MouseIn(*desktop.MouseEvent) { t.hovered = true; t.Refresh() }
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
