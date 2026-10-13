# ODM browser integration

The extension hands downloads to ODM through Chrome/Firefox **native messaging**.
The browser starts `ODM.exe` as the native host; ODM recognises that launch and
relays each request to the running ODM instance over its control socket,
`<config>/odm/odm.sock` (override with `ODM_SOCKET`). `<config>` is
`%AppData%` on Windows, `~/Library/Application Support` on macOS and
`~/.config` on Linux.

## 1. Register the native host (once)

In ODM: **Options → General → Register browser integration**, or from a shell:

```
ODM.exe -register-browser      # Windows
./odm -register-browser        # macOS / Linux
```

This writes the host manifests to `<config>/odm/native/` and registers them for
the current user:

- **Windows** — `HKCU\Software\{Google\Chrome, Microsoft\Edge, Chromium, Mozilla}\NativeMessagingHosts\com.ochi.odm`.
- **macOS / Linux** — copies the manifest into each installed browser's
  `NativeMessagingHosts` folder (Chrome, Chromium, Edge, Firefox).

Run it again after moving `ODM.exe`; `-unregister-browser` removes it.

## 2. Install the extension

Each release ships both packages:

- **Chrome, Edge, Brave, Opera** — `odm-extension-chromium.zip`. Unzip it to
  a folder you keep, open `chrome://extensions` (or `edge://extensions`),
  enable *Developer mode*, choose *Load unpacked* and pick that folder. The
  `key` in the manifest pins the ID to `fnondfibgeiffjplpokfgiacjelmbcbk`,
  the origin the host manifest allows; check that the page shows it. To
  update, replace the folder's contents and click *Reload*. Chrome and Edge
  do not install `.crx` files from outside their stores on Windows and macOS,
  hence the unpacked route until the extension is in the stores.
- **Firefox** — `odm-extension-firefox.xpi`, signed by Mozilla: open it in
  Firefox (drag it onto a window, or *File → Open File*) and confirm. Its
  add-on ID is `odm@troke.id`. Pre-releases ship
  `odm-extension-firefox-unsigned.xpi`, which only Firefox Developer Edition
  or Nightly accept (with `xpinstall.signatures.required` set to `false`).

Restart the browser once after registering so it re-reads the host list.

Developers can load the source folder directly: *Load unpacked* on this
`extension/` folder in Chromium browsers, or `about:debugging` → *Load
Temporary Add-on* in Firefox (removed when Firefox closes).

### How the packages differ

`go run ./cmd/extpack` builds both from this folder. Firefox does not support
`background.service_worker`, so its manifest uses `background.scripts`, drops
`key`, and keeps `browser_specific_settings` (add-on ID and the data
collection declaration: the download URL and the page it came from are sent
to ODM on this computer). The Chromium manifest drops the Firefox-only keys.

## 3. Use it

Keep ODM running, then click a file link (ODM takes over matching download
types) or right-click a link/video → **Download with ODM**. The popup toggles
takeover and tests the connection.
