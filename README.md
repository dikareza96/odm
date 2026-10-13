# Ochi Download Manager (ODM)

A segmented, resumable download manager for Windows (with a cross-platform core),
built in pure Go. The engine design follows what we learned by reverse engineering
Internet Download Manager: multiple sockets per file, byte-range requests, dynamic
chunk splitting, per-chunk resume state and a ~23 KiB minimum chunk.

## Highlights

- **Segmented transfers** — up to N sockets per file using `Range: bytes=start-end`
  (default 8, like the reference product).
- **Dynamic splitting** — an idle connection steals the tail half of the busiest
  chunk (the reference product's "alternative connection").
- **Resume** — per-chunk progress persisted as JSON plus a `.odmpart` file.
- **Graceful fallback** — servers without `Accept-Ranges` are fetched on one stream.
- **Queue, settings, categories** — concurrent-download cap, global speed limit,
  and automatic subfolders (`Video/`, `Audio/`, `Documents/`, …).
- **Everyday actions** — pause / resume, pause-all / resume-all, clear-completed,
  open file, reveal in folder, remove, or remove + delete the file.
- **Browser takeover** — an MV3 extension plus a native-messaging host queues
  downloads from Chrome / Edge / Firefox, carrying referrer and cookies.
- **Clipboard monitor** — on Windows, copying a downloadable URL queues it.
- **Desktop UI** — a themed window built with Fyne: icon toolbar, category
  sidebar, report table with progress bars, and dialogs (Download File Info,
  Options, per-download progress with tabs and a connection table). No browser
  and no web server; on headless/non-UI runs the same engine is driven by a
  JSON + SSE API.

## Build

Requires Go 1.23+. The core engine uses only the standard library. The desktop
UI uses [Fyne](https://fyne.io), which needs cgo, so a Windows cross-build needs
mingw-w64 (`CC_WINDOWS`, default `x86_64-w64-mingw32-gcc`):

```bash
sudo apt install gcc-mingw-w64-x86-64      # mingw toolchain for cross-building
./scripts/build.sh windows                 # -> dist/ODM.exe
go build -o odm ./cmd/odm                  # local build (no UI on non-Windows)
```

## Run

```bash
./ODM.exe                        # engine + UI window
./ODM.exe -dir "D:\\Downloads"    # override the saved folder
./ODM.exe -add https://host/big.iso -no-ui   # headless: fetch one URL and exit
```

Flags: `-addr` (default `127.0.0.1:8721`), `-dir`, `-conns`, `-files`, `-add`,
`-native-messaging`, `-open`, `-no-ui`. Settings (folder, connections,
concurrency, speed limit, categories, clipboard) persist in
`<config>/odm/settings.json`.

`-native-messaging` runs the browser relay: it does not start a server, it just
forwards requests to an already-running ODM (`ODM_ADDR`, default `127.0.0.1:8721`).
See [extension/README.md](extension/README.md) for registering it with a browser.

## HTTP API

| Method | Path | Purpose |
| ------ | ---- | ------- |
| GET | `/api/settings` (`/api/config`) | current settings |
| PUT | `/api/settings` | update settings (applied live) |
| GET | `/api/downloads` | list all downloads |
| POST | `/api/downloads` | `{"url","dir","connections","headers","speed_limit"}` |
| GET | `/api/downloads/{id}` | one download |
| POST | `/api/downloads/{id}/pause` \| `/resume` \| `/retry` | control |
| POST | `/api/downloads/{id}/open` \| `/reveal` | open file / show in folder |
| DELETE | `/api/downloads/{id}[?file=true]` | remove entry, optionally delete file |
| POST | `/api/actions/pause-all` \| `/resume-all` \| `/clear-completed` | batch |
| GET | `/api/events` | Server-Sent Events stream of the list |

## Architecture

```
cmd/odm            CLI entry point; native UI, native-messaging relay, headless add
internal/engine    probe, chunk planner, segmented transfer, manager, settings, JSON store
internal/ui        desktop UI (Fyne) — toolbar, category list, report table, dialogs
internal/server    JSON + SSE API and the embedded web UI (headless)
internal/native    Chrome/Firefox native-messaging framing
internal/clipmon   Windows clipboard watcher + URL detection
internal/osutil    open / reveal helpers
extension/         MV3 browser extension + native-host manifests
```

The Windows build is a GUI subsystem executable (`-H=windowsgui`), so no console
window appears. The loopback API still runs as an internal control channel so the
browser native-messaging relay can queue downloads; it is not used to draw the UI.

`internal/engine` uses only the standard library and has no OS bindings, so the
same engine backs the planned Linux/macOS builds.

### Transfer algorithm

1. **Probe** — a ranged `GET bytes=0-` learns size, `Accept-Ranges`, file name,
   cookies and validators.
2. **Plan** — `[0,total)` is split into up to N chunks, never smaller than
   `MinChunkSize` (23363 B); a tiny tail is folded into the previous chunk.
3. **Work** — N workers claim chunks and stream them into the `.odmpart` file at
   their absolute offset.
4. **Split** — a free worker halves the largest remaining chunk and takes the tail.
5. **Finish** — the `.odmpart` file is renamed onto the final name.

Resume state records `{id,start,end,done}` per chunk, mirroring the per-chunk
records the reference product keeps in the registry.

## Design notes from the reverse engineering

| Observed in IDM | ODM |
| --------------- | --- |
| default 8 connections, `MaxConnectionsNumber` registry value | `-conns` / settings |
| min chunk 23363 B, 2 MiB query-range step | `MinChunkSize`, `DefaultChunkStep` |
| `Range: bytes=` builder | `Download.fetch` |
| chunk reassigned between connections | `Split` + worker steal |
| per-chunk state for resume | `State` / `Store` |
| category folders by file type | `Category` + settings |
| `&bytes=` fallback for non-range servers | single-stream fallback (query-range planned) |
| MP4/MKV/FLV muxer in `idmvconv.dll` | planned: hand off to ffmpeg |

## Roadmap

- Query-range (`?start=..&end=..`) fallback for servers without `Accept-Ranges`.
- HLS/DASH grabber (sniff + segment download) with ffmpeg finalise.
- System tray, per-download speed limits, scheduler, site grabber.

## License note

ODM is an independent reimplementation of publicly observable download-manager
behaviour. It contains no code or data from any commercial product.
