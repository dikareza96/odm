//go:build fyne && !windows

package ui

import "context"

// watchDPI is a Windows workaround (see dpi_windows.go); elsewhere Fyne
// follows the monitor's scale itself.
func (a *application) watchDPI(context.Context) {}
