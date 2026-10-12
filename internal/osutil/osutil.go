// Package osutil opens files and folders with the host's default handler.
package osutil

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open launches the file with the default application.
func Open(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// Reveal shows the file in the OS file manager, selecting it when supported.
func Reveal(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}
