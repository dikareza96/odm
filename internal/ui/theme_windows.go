//go:build windows

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// odmPalette is a compact, modern dark palette.
var odmPalette = struct {
	background  color.NRGBA
	surface     color.NRGBA
	surfaceAlt  color.NRGBA
	border      color.NRGBA
	foreground  color.NRGBA
	muted       color.NRGBA
	accent      color.NRGBA
	accentSoft  color.NRGBA
	hover       color.NRGBA
	pressed     color.NRGBA
	success     color.NRGBA
	warning     color.NRGBA
	danger      color.NRGBA
	scrollTrack color.NRGBA
	scrollThumb color.NRGBA
}{
	background:  color.NRGBA{R: 0x0E, G: 0x10, B: 0x14, A: 0xFF},
	surface:     color.NRGBA{R: 0x15, G: 0x18, B: 0x1F, A: 0xFF},
	surfaceAlt:  color.NRGBA{R: 0x1B, G: 0x1F, B: 0x27, A: 0xFF},
	border:      color.NRGBA{R: 0x26, G: 0x2B, B: 0x36, A: 0xFF},
	foreground:  color.NRGBA{R: 0xE7, G: 0xEA, B: 0xF1, A: 0xFF},
	muted:       color.NRGBA{R: 0x8B, G: 0x93, B: 0xA5, A: 0xFF},
	accent:      color.NRGBA{R: 0x4F, G: 0x7C, B: 0xFF, A: 0xFF},
	accentSoft:  color.NRGBA{R: 0x2A, G: 0x3A, B: 0x63, A: 0xFF},
	hover:       color.NRGBA{R: 0x22, G: 0x27, B: 0x31, A: 0xFF},
	pressed:     color.NRGBA{R: 0x2A, G: 0x30, B: 0x3D, A: 0xFF},
	success:     color.NRGBA{R: 0x3F, G: 0xBF, B: 0x8F, A: 0xFF},
	warning:     color.NRGBA{R: 0xE0, G: 0xA9, B: 0x3B, A: 0xFF},
	danger:      color.NRGBA{R: 0xF0, G: 0x55, B: 0x5C, A: 0xFF},
	scrollTrack: color.NRGBA{R: 0x1B, G: 0x1F, B: 0x27, A: 0xFF},
	scrollThumb: color.NRGBA{R: 0x39, G: 0x41, B: 0x52, A: 0xFF},
}

// odmTheme overrides the stock dark theme with a flatter, rounder look.
type odmTheme struct {
	fyne.Theme
}

func newTheme() fyne.Theme {
	return odmTheme{Theme: theme.DarkTheme()}
}

func (t odmTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	p := odmPalette
	switch name {
	case theme.ColorNameBackground:
		return p.background
	case theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return p.surface
	case theme.ColorNameInputBackground, theme.ColorNameButton:
		return p.surfaceAlt
	case theme.ColorNamePrimary:
		return p.accent
	case theme.ColorNameSelection:
		return p.accentSoft
	case theme.ColorNameHover:
		return p.hover
	case theme.ColorNamePressed:
		return p.pressed
	case theme.ColorNameFocus:
		return color.NRGBA{R: p.accent.R, G: p.accent.G, B: p.accent.B, A: 0x88}
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return p.border
	case theme.ColorNameScrollBar:
		return p.scrollThumb
	case theme.ColorNameScrollBarBackground:
		return p.scrollTrack
	case theme.ColorNameForeground:
		return p.foreground
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return p.muted
	case theme.ColorNameSuccess:
		return p.success
	case theme.ColorNameWarning:
		return p.warning
	case theme.ColorNameError:
		return p.danger
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}
	return t.Theme.Color(name, variant)
}

func (t odmTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 4
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 5
	case theme.SizeNameSelectionRadius:
		return 4
	case theme.SizeNameCardRadius:
		return 8
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return 10
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameText:
		return 12
	case theme.SizeNameSubHeadingText:
		return 13
	case theme.SizeNameHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 10
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameLineSpacing:
		return 2
	}
	return t.Theme.Size(name)
}
