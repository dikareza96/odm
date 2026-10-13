package native

import "strings"

// LaunchedByBrowser reports whether the process was started by a browser as
// a native-messaging host. Browsers cannot pass flags (the host manifest's
// "path" takes no arguments), so the invocation is recognised by what they
// do pass:
//
//   - Chrome, Edge, Chromium: the caller's origin, "chrome-extension://<id>/",
//     plus "--parent-window=<handle>" on Windows;
//   - Firefox: the path to the host manifest, then the add-on ID.
func LaunchedByBrowser(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if strings.HasPrefix(args[0], "chrome-extension://") {
		return true
	}
	return len(args) >= 2 && strings.HasSuffix(strings.ToLower(args[0]), ".json") && strings.Contains(args[1], "@")
}
