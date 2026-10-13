package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// entryPath follows the XDG autostart spec: $XDG_CONFIG_HOME/autostart,
// defaulting to ~/.config/autostart.
func entryPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "odm.desktop")
}

// quoteExec quotes an argument for a desktop entry's Exec line.
func quoteExec(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(s) + `"`
}

func enable_(exe string) error {
	p := entryPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Ochi Download Manager\nExec=%s %s\nTerminal=false\n",
		quoteExec(exe), HiddenFlag)
	return os.WriteFile(p, []byte(body), 0o644)
}

func disable() error {
	if err := os.Remove(entryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func enabled() bool {
	_, err := os.Stat(entryPath())
	return err == nil
}
