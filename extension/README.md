# ODM browser integration

The extension hands downloads to ODM through Chrome/Firefox **native messaging**.
The native host is `ODM.exe -native-messaging`, which relays each request to a
running ODM instance over `http://127.0.0.1:8721` (override with `ODM_ADDR`).

## 1. Load the extension (developer / unpacked)

- **Chrome/Edge:** `chrome://extensions` → enable *Developer mode* → *Load
  unpacked* → pick this `extension/` folder. Copy the generated **extension ID**.
- **Firefox:** `about:debugging#/runtime/this-firefox` → *Load Temporary Add-on*
  → pick `extension/manifest.json`.

## 2. Register the native host

Edit `native/com.ochi.odm.json` (Chrome) and set:

- `path` → the absolute path to `ODM.exe` (backslashes escaped: `C:\\Tools\\ODM.exe`).
- `allowed_origins` → `chrome-extension://<the extension ID you copied>/`.

For Firefox use `native/com.ochi.odm.firefox.json` and set the extension id in
`manifest.json` (`browser_specific_settings.gecko.id`).

### Windows — Chrome/Edge

```cmd
reg add "HKCU\Software\Google\Chrome\NativeMessagingHosts\com.ochi.odm" /ve /t REG_SZ ^
  /d "C:\path\to\com.ochi.odm.json" /f

reg add "HKCU\Software\Microsoft\Edge\NativeMessagingHosts\com.ochi.odm" /ve /t REG_SZ ^
  /d "C:\path\to\com.ochi.odm.json" /f
```

### Windows — Firefox

```cmd
reg add "HKCU\Software\Mozilla\NativeMessagingHosts\com.ochi.odm" /ve /t REG_SZ ^
  /d "C:\path\to\com.ochi.odm.firefox.json" /f
```

### macOS / Linux (for reference)

- Chrome: `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.ochi.odm.json`
  and `~/.config/google-chrome/NativeMessagingHosts/com.ochi.odm.json`.
- Firefox: `~/Library/Application Support/Mozilla/NativeMessagingHosts/…`
  and `~/.mozilla/native-messaging-hosts/…`.
- The host `path` must point at a runnable ODM binary.

## 3. Use it

Start ODM (`ODM.exe`), then in the browser either click a file link (ODM takes
over matching download types) or right-click a link/video → **Download with ODM**.
The popup toggles takeover and tests the connection.
