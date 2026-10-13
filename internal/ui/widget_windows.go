//go:build windows

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	r.bg.CornerRadius = 8
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
	const pad = float32(6)
	iconSize := size.Height * 0.5
	r.icon.Move(fyne.NewPos(size.Width/2-iconSize/2, pad))
	r.icon.Resize(fyne.NewSize(iconSize, iconSize))
	top := pad + iconSize + 3
	r.label.Move(fyne.NewPos(0, top))
	r.label.Resize(fyne.NewSize(size.Width, size.Height-top-pad))
}

func (r *toolButtonRenderer) MinSize() fyne.Size {
	w := fyne.MeasureText(r.btn.text, theme.CaptionTextSize(), fyne.TextStyle{}).Width + 18
	if w < 62 {
		w = 62
	}
	return fyne.NewSize(w, 54)
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
