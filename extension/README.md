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

## 2. Load the extension

- **Chrome / Edge:** `chrome://extensions` (or `edge://extensions`) → enable
  *Developer mode* → *Load unpacked* → pick this `extension/` folder. The
  `key` in `manifest.json` pins the ID to `fnondfibgeiffjplpokfgiacjelmbcbk`,
  which is the origin the host manifest allows; check that the page shows it.
- **Firefox:** `about:debugging#/runtime/this-firefox` → *Load Temporary
  Add-on* → pick `manifest.json`. Its add-on ID is `odm@ochi.local`.

Restart the browser once after registering so it re-reads the host list.

## 3. Use it

Keep ODM running, then click a file link (ODM takes over matching download
types) or right-click a link/video → **Download with ODM**. The popup toggles
takeover and tests the connection.
