//go:build fyne && windows

package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

// GetDpiForWindow is Windows 10 1607 and later.
var procGetDpiForWindow = windows.NewLazySystemDLL("user32.dll").NewProc("GetDpiForWindow")

// watchDPI rescales ODM's windows when one is moved to a monitor with a
// different Windows scale (100% and 200%, say).
//
// Fyne 2.8.2 does not do this reliably on Windows. It recomputes a window's
// scale on a move only when the monitor's physical DPI changes, and reads
// the scale from GLFW at that moment, which is often still the old one:
// Windows reports the new DPI a little later (WM_DPICHANGED), and Fyne
// ignores GLFW's content-scale callback outside Wayland. The window frame
// follows the new monitor while the content keeps the old scale: tiny on a
// 200% screen.
//
// The workaround polls each window's DPI and, when one changed, sets the
// theme again: Fyne then reloads every window's scale (and resizes it to
// keep its size at the new scale). Polling is cheap, one call per window
// twice a second; on Windows before 10 1607 the call is missing and the
// watch does nothing.
func (a *application) watchDPI(ctx context.Context) {
	if procGetDpiForWindow.Find() != nil {
		return
	}
	last := map[uintptr]uintptr{}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		// HWNDs come from the UI thread, and the theme is set there.
		fyne.DoAndWait(func() {
			changed := false
			seen := map[uintptr]bool{}
			for _, w := range a.fyneApp.Driver().AllWindows() {
				nw, ok := w.(driver.NativeWindow)
				if !ok {
					continue
				}
				var hwnd uintptr
				nw.RunNative(func(c any) {
					if wc, ok := c.(driver.WindowsWindowContext); ok {
						hwnd = wc.HWND
					}
				})
				if hwnd == 0 { // not shown yet
					continue
				}
				seen[hwnd] = true
				dpi, _, _ := procGetDpiForWindow.Call(hwnd)
				if dpi == 0 {
					continue
				}
				if prev, ok := last[hwnd]; ok && prev != dpi {
					changed = true
				}
				last[hwnd] = dpi
			}
			for h := range last {
				if !seen[h] {
					delete(last, h)
				}
			}
			if changed {
				s := a.fyneApp.Settings()
				s.SetTheme(s.Theme())
			}
		})
	}
}
