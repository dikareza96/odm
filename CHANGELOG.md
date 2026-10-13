# Changelog

All notable changes to ODM are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Segmented, resumable download engine: multiple sockets per file using
  `Range: bytes=`, dynamic chunk splitting, per-chunk resume state, retry with
  backoff, shared global and per-download speed limits.
- Graceful single-stream fallback for servers without `Accept-Ranges`.
- Category folders by file type, automatic subfolder placement.
- Queues: named queues, `Start queue` / `Stop queue`, move a download between
  queues, `Q` column.
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
