// Package browserreg registers ODM as the native-messaging host for its
// browser extension, the step that otherwise means hand-editing a JSON
// manifest and the registry.
//
// Locations follow the browsers' documentation: on Windows a per-user
// registry key whose default value is the manifest path; on macOS and Linux
// the manifest file itself in each browser's NativeMessagingHosts folder.
package browserreg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HostName is the native host name the extension connects to.
const HostName = "com.ochi.odm"

// ChromeExtensionID is fixed by the "key" in extension/manifest.json (see
// TestChromeIDMatchesManifestKey). Chrome's allowed_origins takes no
// wildcards, so the unpacked extension must keep this ID.
const ChromeExtensionID = "fnondfibgeiffjplpokfgiacjelmbcbk"

// ChromiumExtensionIDs are every ID the Chromium-family extension can have:
// the pinned unpacked ID, plus each store's ID once it is published there
// (stores assign their own). The host manifest allows all of them, as a
// browser reads only the first registered host manifest it finds.
var ChromiumExtensionIDs = []string{ChromeExtensionID}

// FirefoxExtensionID is browser_specific_settings.gecko.id in the manifest.
const FirefoxExtensionID = "odm@troke.id"

// Result describes one browser registration.
type Result struct {
	Browser  string
	Location string // registry key or manifest file
	Err      error
	Skipped  bool // browser not installed for this user
}

func (r Result) String() string {
	switch {
	case r.Skipped:
		return fmt.Sprintf("%s: not found, skipped", r.Browser)
	case r.Err != nil:
		return fmt.Sprintf("%s: %v", r.Browser, r.Err)
	default:
		return fmt.Sprintf("%s: %s", r.Browser, r.Location)
	}
}

type manifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedOrigins    []string `json:"allowed_origins,omitempty"`
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
}

// writeManifests writes the Chromium-family and Firefox host manifests for
// exe into dir and returns their paths. Firefox rejects allowed_origins and
// Chrome rejects allowed_extensions, hence two files.
func writeManifests(dir, exe string) (chromium, firefox string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	base := manifest{Name: HostName, Description: "Ochi Download Manager", Path: exe, Type: "stdio"}
	c := base
	for _, id := range ChromiumExtensionIDs {
		c.AllowedOrigins = append(c.AllowedOrigins, "chrome-extension://"+id+"/")
	}
	f := base
	f.AllowedExtensions = []string{FirefoxExtensionID}
	chromium = filepath.Join(dir, HostName+".json")
	firefox = filepath.Join(dir, HostName+".firefox.json")
	for path, m := range map[string]manifest{chromium: c, firefox: f} {
		data, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", "", err
		}
	}
	return chromium, firefox, nil
}

// Register writes the manifests under configDir/native and points every
// supported browser found for this user at them. exe must be absolute.
func Register(exe, configDir string) []Result {
	chromium, firefox, err := writeManifests(filepath.Join(configDir, "native"), exe)
	if err != nil {
		return []Result{{Browser: "manifest", Err: err}}
	}
	return register(chromium, firefox)
}

// Unregister removes what Register added (the manifests under configDir are
// left; they are inert without the registrations).
func Unregister() []Result { return unregister() }
