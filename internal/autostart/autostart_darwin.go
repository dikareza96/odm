package autostart

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

const label = "com.ochi.odm"

func agentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// enable_ writes a LaunchAgent with RunAtLoad, which launchd runs when the
// user's session loads its agents at login.
func enable_(exe string) error {
	p := agentPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + label + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + xmlEscape(exe) + `</string>
		<string>` + HiddenFlag + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
	return os.WriteFile(p, []byte(body), 0o644)
}

func disable() error {
	if err := os.Remove(agentPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func enabled() bool {
	_, err := os.Stat(agentPath())
	return err == nil
}
