//go:build windows

package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// toolButton is an icon-above-label toolbar button with a hover highlight,
// matching the classic download-manager toolbar.
type toolButton struct {
	widget.BaseWidget
	icon  fyne.Resource
	text  string
	onTap func()
	hover bool
}

func newToolButton(icon fyne.Resource, text string, onTap func()) *toolButton {
	b := &toolButton{icon: icon, text: text, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *toolButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *toolButton) MouseIn(*fyne.PointEvent) { b.hover = true; b.Refresh() }

func (b *toolButton) MouseMoved(*fyne.PointEvent) {}

func (b *toolButton) MouseOut() { b.hover = false; b.Refresh() }

type toolButtonRenderer struct {
	btn     *toolButton
	bg      *canvas.Rectangle
	icon    *canvas.Image
	label   *canvas.Text
	objects []fyne.CanvasObject
}

func (b *toolButton) CreateRenderer() fyne.WidgetRenderer {
	r := &toolButtonRenderer{btn: b}
	r.bg = canvas.NewRectangle(color.Transparent)
	r.bg.CornerRadius = 6
	r.icon = canvas.NewImageFromResource(b.icon)
	r.icon.FillMode = canvas.ImageFillContain
	r.icon.ScaleMode = canvas.ImageScaleSmooth
	r.label = canvas.NewText(b.text, theme.ForegroundColor())
	r.label.Alignment = fyne.TextAlignCenter
	r.label.TextSize = theme.CaptionTextSize()
	r.objects = []fyne.CanvasObject{r.bg, r.icon, r.label}
	r.Refresh()
	return r
}

func (r *toolButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	const pad = float32(4)
	iconSize := size.Height * 0.48
	r.icon.Move(fyne.NewPos(size.Width/2-iconSize/2, pad))
	r.icon.Resize(fyne.NewSize(iconSize, iconSize))
	top := pad + iconSize + 2
	r.label.Move(fyne.NewPos(0, top))
	r.label.Resize(fyne.NewSize(size.Width, size.Height-top-pad))
}

func (r *toolButtonRenderer) MinSize() fyne.Size {
	w := fyne.MeasureText(r.btn.text, theme.CaptionTextSize(), fyne.TextStyle{}).Width + 14
	if w < 56 {
		w = 62
	}
	return fyne.NewSize(w, 44)
}

func (r *toolButtonRenderer) Refresh() {
	if r.btn.hover {
		r.bg.FillColor = theme.HoverColor()
		r.label.Color = theme.PrimaryColor()
	} else {
		r.bg.FillColor = color.Transparent
		r.label.Color = theme.ForegroundColor()
	}
	r.icon.Resource = r.btn.icon
	r.label.Text = r.btn.text
	r.bg.Refresh()
	r.icon.Refresh()
	r.label.Refresh()
}

func (r *toolButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *toolButtonRenderer) Destroy() {}

// chunkStrip draws one bar per connection, positioned and sized in proportion
// to its byte range ("start positions and download progress by connections").
type chunkStrip struct {
	widget.BaseWidget
	mu     sync.Mutex
	chunks []engine.ChunkInfo
	total  int64
}

func newChunkStrip() *chunkStrip {
	s := &chunkStrip{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *chunkStrip) set(chunks []engine.ChunkInfo, total int64) {
	s.mu.Lock()
	s.chunks = chunks
	s.total = total
	s.mu.Unlock()
	s.Refresh()
}

func (s *chunkStrip) CreateRenderer() fyne.WidgetRenderer {
	r := &stripRenderer{strip: s}
	r.rebuild()
	return r
}

type stripRenderer struct {
	strip   *chunkStrip
	objects []fyne.CanvasObject
}

func (r *stripRenderer) rebuild() {
	r.strip.mu.Lock()
	n := len(r.strip.chunks)
	r.strip.mu.Unlock()
	if n < 1 {
		n = 1
	}
	r.objects = make([]fyne.CanvasObject, 0, n*2)
	for i := 0; i < n; i++ {
		bg := canvas.NewRectangle(odmPalette.scrollTrack)
		bg.CornerRadius = 2
		fill := canvas.NewRectangle(odmPalette.accent)
		fill.CornerRadius = 2
		r.objects = append(r.objects, bg, fill)
	}
}

func (r *stripRenderer) Layout(size fyne.Size) {
	r.strip.mu.Lock()
	chunks := append([]engine.ChunkInfo(nil), r.strip.chunks...)
	total := r.strip.total
	r.strip.mu.Unlock()
	if total <= 0 {
		total = 1
	}
	for i, c := range chunks {
		if i*2+1 >= len(r.objects) {
			break
		}
		bg := r.objects[i*2].(*canvas.Rectangle)
		fill := r.objects[i*2+1].(*canvas.Rectangle)
		x := size.Width * float32(float64(c.Start)/float64(total))
		w := size.Width * float32(float64(c.End-c.Start+1)/float64(total))
		if w < 1 {
			w = 1
		}
		bg.Move(fyne.NewPos(x, 0))
		bg.Resize(fyne.NewSize(w, size.Height))

		frac := 0.0
		if span := float64(c.End - c.Start + 1); span > 0 {
			frac = float64(c.Done) / span
		}
		if frac >= 1 {
			fill.FillColor = odmPalette.success
		} else {
			fill.FillColor = odmPalette.accent
		}
		fill.Move(fyne.NewPos(x, 0))
		fill.Resize(fyne.NewSize(w*float32(frac), size.Height))
	}
}

func (r *stripRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 14) }

func (r *stripRenderer) Refresh() {
	r.strip.mu.Lock()
	need := len(r.strip.chunks)
	r.strip.mu.Unlock()
	if need*2 != len(r.objects) {
		r.rebuild()
	}
}

func (r *stripRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *stripRenderer) Destroy() {}

// miniBar is a slim progress bar used inside table rows so rows stay short.
type miniBar struct {
	widget.BaseWidget
	mu sync.Mutex
	v  float64
}

func newMiniBar() *miniBar {
	b := &miniBar{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *miniBar) set(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	b.mu.Lock()
	b.v = v
	b.mu.Unlock()
	b.Refresh()
}

func (b *miniBar) MinSize() fyne.Size { return fyne.NewSize(60, 5) }

func (b *miniBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(odmPalette.scrollTrack)
	bg.CornerRadius = 2
	fill := canvas.NewRectangle(odmPalette.accent)
	fill.CornerRadius = 2
	return &miniBarRenderer{bar: b, bg: bg, fill: fill, objects: []fyne.CanvasObject{bg, fill}}
}

type miniBarRenderer struct {
	bar     *miniBar
	bg      *canvas.Rectangle
	fill    *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *miniBarRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bar.mu.Lock()
	v := r.bar.v
	r.bar.mu.Unlock()
	if v >= 1 {
		r.fill.FillColor = odmPalette.success
	} else {
		r.fill.FillColor = odmPalette.accent
	}
	r.fill.Resize(fyne.NewSize(size.Width*float32(v), size.Height))
}

func (r *miniBarRenderer) MinSize() fyne.Size { return fyne.NewSize(60, 5) }

func (r *miniBarRenderer) Refresh() {
	canvas.Refresh(r.bg)
	canvas.Refresh(r.fill)
}

func (r *miniBarRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *miniBarRenderer) Destroy() {}

// headerLabel is a compact, clickable table column header.
type headerLabel struct {
	widget.BaseWidget
	app   *application
	col   int
	text  string
	hover bool
}

func newHeaderLabel(app *application) *headerLabel {
	h := &headerLabel{app: app}
	h.ExtendBaseWidget(h)
	return h
}

// SetText updates the header caption.
func (h *headerLabel) SetText(s string) {
	if h.text == s {
		return
	}
	h.text = s
	h.Refresh()
}

func (h *headerLabel) Tapped(*fyne.PointEvent) {
	if h.app != nil {
		h.app.sortBy(h.col)
	}
}

func (h *headerLabel) MouseIn(*fyne.PointEvent) { h.hover = true; h.Refresh() }

func (h *headerLabel) MouseMoved(*fyne.PointEvent) {}

func (h *headerLabel) MouseOut() { h.hover = false; h.Refresh() }

type headerRenderer struct {
	h       *headerLabel
	text    *canvas.Text
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (h *headerLabel) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(h.text, theme.ForegroundColor())
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.TextSize = theme.CaptionTextSize()
	bg := canvas.NewRectangle(color.Transparent)
	return &headerRenderer{h: h, text: text, bg: bg, objects: []fyne.CanvasObject{bg, text}}
}

func (r *headerRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	pad := float32(4)
	r.text.Move(fyne.NewPos(pad, size.Height/2-r.text.TextSize/2))
	r.text.Resize(fyne.NewSize(size.Width-pad*2, size.Height))
}

func (r *headerRenderer) MinSize() fyne.Size {
	w := fyne.MeasureText(r.h.text, theme.CaptionTextSize(), fyne.TextStyle{Bold: true}).Width + 10
	return fyne.NewSize(w, theme.CaptionTextSize()+6)
}

func (r *headerRenderer) Refresh() {
	r.text.Text = r.h.text
	if r.h.hover {
		r.bg.FillColor = theme.HoverColor()
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *headerRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *headerRenderer) Destroy() {}

// indentSpacer reserves horizontal space so nested sidebar rows line up.
type indentSpacer struct {
	widget.BaseWidget
	w float32
}

func newIndentSpacer() *indentSpacer {
	s := &indentSpacer{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *indentSpacer) set(w float32) {
	if s.w == w {
		return
	}
	s.w = w
	s.Refresh()
}

func (s *indentSpacer) MinSize() fyne.Size { return fyne.NewSize(s.w, 1) }

func (s *indentSpacer) CreateRenderer() fyne.WidgetRenderer { return &spacerRenderer{s: s} }

type spacerRenderer struct{ s *indentSpacer }

func (r *spacerRenderer) Layout(fyne.Size) {}

func (r *spacerRenderer) MinSize() fyne.Size { return r.s.MinSize() }

func (r *spacerRenderer) Refresh() {}

func (r *spacerRenderer) Objects() []fyne.CanvasObject { return nil }

func (r *spacerRenderer) Destroy() {}
