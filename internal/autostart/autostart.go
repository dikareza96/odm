// Package autostart starts ODM when the user logs in, hidden in the tray,
// using each OS's per-user mechanism: the HKCU Run key on Windows, an XDG
// autostart .desktop file on Linux, and a LaunchAgent on macOS. All three are
// per-user, so no elevation is needed and nothing changes for other accounts.
package autostart

// HiddenFlag is passed so a login launch starts in the tray, not on screen.
const HiddenFlag = "-hidden"

// Set enables or disables starting exe at login.
func Set(enable bool, exe string) error {
	if enable {
		return enable_(exe)
	}
	return disable()
}

// Enabled reports whether ODM is registered to start at login.
func Enabled() bool { return enabled() }
