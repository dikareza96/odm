# Changelog

All notable changes to ODM are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-10-13

### Added

- Segmented, resumable download engine: multiple sockets per file using
  `Range: bytes=`, dynamic chunk splitting, per-chunk resume state, retry with
  backoff, shared global and per-download speed limits.
- Graceful single-stream fallback for servers without `Accept-Ranges`.
- Category folders by file type, automatic subfolder placement.
- Queues: named queues, `Start queue` / `Stop queue`, move a download between
  queues, `Q` column. A started queue runs one download at a time in queue
  order; items can be moved up and down.
- Scheduler: start/stop queues at set times on chosen weekdays; optionally
  exit ODM or shut the computer down when started queues finish, after a
  cancellable 30-second countdown.
- Extra file types for the clipboard monitor and site grabber; per-category
  destination folders.
- Site logins: per-host credentials applied as HTTP Basic auth.
- HTTP proxy support.
- Download list export / import as JSON.
- Site grabber: breadth-first crawl with depth/page/file limits that queues
  every recognised file link.
- Browser takeover: MV3 extension plus a native-messaging relay host.
- Windows clipboard monitor for downloadable URLs.
- Desktop UI (Fyne, Windows/macOS/Linux with the `fyne` build tag): menu bar,
  icon toolbar, category/queue sidebar, sortable report table, Add/Options/
  progress/grabber dialogs.
- Headless mode: JSON + SSE API and an embedded web UI (`-no-ui`).
- Progress window opens when a download is started or resumed (status,
  speed limiter, completion options, connection map); completion window
  with Open / Open folder when a download finishes.
- System tray: close to tray, tray menu, launch on startup (hidden), and a
  second launch brings the running window forward.
- Multi-select in the list (Ctrl/Shift-click, Ctrl+A) with Delete, Enter,
  arrow keys and Ctrl+N shortcuts.
- Scheduler window with per-queue schedules (start with ODM, at a time once
  or on weekdays, stop at a time, retries, open file / exit / shut down when
  done, files at the same time) and download limits.
- Options window with File types, Save to, Downloads, Connection (per-server
  connection exceptions, download limits), Proxy / Socks (system, manual
  HTTP or SOCKS5) and Site logins tabs; configurable User-Agent.
- `-register-browser` and an Options button register the native host for
  Chrome, Edge, Chromium and Firefox; the extension's ID is pinned.
- Update check against GitHub Releases at start-up and from the Help menu
  (notify only, can be disabled in Options).
- Local control channel over a Unix domain socket (browser relay, single
  instance); no TCP port is opened unless `-serve` is given, and that
  listener requires a per-run token and rejects foreign origins.

[Unreleased]: https://github.com/troke12/odm/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/troke12/odm/releases/tag/v0.1.0
