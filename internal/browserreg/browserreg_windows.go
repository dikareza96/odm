package browserreg

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// Per-user keys; Edge also falls back to the Chrome key, but its own key is
// written so it does not depend on Chrome's entry existing.
var windowsKeys = []struct{ browser, key, manifest string }{
	{"Chrome", `Software\Google\Chrome\NativeMessagingHosts\` + HostName, "chromium"},
	{"Edge", `Software\Microsoft\Edge\NativeMessagingHosts\` + HostName, "chromium"},
	{"Chromium", `Software\Chromium\NativeMessagingHosts\` + HostName, "chromium"},
	{"Firefox", `Software\Mozilla\NativeMessagingHosts\` + HostName, "firefox"},
}

// register writes every key regardless of whether the browser is installed:
// HKCU keys are harmless, and a browser installed later then works at once.
func register(chromium, firefox string) []Result {
	var out []Result
	for _, w := range windowsKeys {
		path := chromium
		if w.manifest == "firefox" {
			path = firefox
		}
		k, _, err := registry.CreateKey(registry.CURRENT_USER, w.key, registry.SET_VALUE)
		if err == nil {
			err = k.SetStringValue("", path)
			k.Close()
		}
		out = append(out, Result{Browser: w.browser, Location: `HKCU\` + w.key, Err: err})
	}
	return out
}

func unregister() []Result {
	var out []Result
	for _, w := range windowsKeys {
		err := registry.DeleteKey(registry.CURRENT_USER, w.key)
		if errors.Is(err, registry.ErrNotExist) {
			err = nil
		}
		out = append(out, Result{Browser: w.browser, Location: `HKCU\` + w.key, Err: err})
	}
	return out
}
