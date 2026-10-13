# Ochi Download Manager (ODM)

A segmented, resumable download manager written in pure Go, with a desktop UI
and a headless HTTP/SSE API. Files are fetched over several connections, resumed
exactly where they stopped, and organised into category folders and queues.

## Highlights

- **Segmented transfers** — up to N sockets per file using `Range: bytes=start-end`
  (default 8).
- **Dynamic splitting** — an idle connection takes over the tail half of the
  busiest chunk instead of sitting still.
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

Requires Go 1.23+. The engine is standard-library only. The desktop UI uses
[Fyne](https://fyne.io) behind the `fyne` build tag and therefore needs cgo.

| Target | Command | Extra requirements |
| ------ | ------- | ------------------ |
| Windows | `./scripts/build.sh windows` | mingw-w64 (`CC_WINDOWS`, default `x86_64-w64-mingw32-gcc`) |
| Linux | `./scripts/build.sh linux` | `libgl1-mesa-dev xorg-dev libxkbcommon-dev` |
| macOS | `./scripts/build.sh darwin` | Xcode command line tools |
| Headless | `./scripts/build.sh cli` | none |

```bash
go test ./...                              # engine tests, no UI needed
go build -tags fyne -o odm ./cmd/odm        # desktop build for the current OS
sudo apt install gcc-mingw-w64-x86-64      # for a Windows cross-build from Linux
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

Resume state records `{id,start,end,done}` per chunk as JSON, so a restart
continues from the exact byte offset.

## Design notes

- **Connections** — 8 per file by default, configurable per download or in settings.
- **Chunking** — `[0,total)` is split into chunks no smaller than `MinChunkSize`
  (23363 B); a tiny tail is folded into the previous chunk.
- **Dynamic splitting** — a free worker halves the largest remaining chunk.
- **Resume** — per-chunk state is persisted, so restarts continue precisely.
- **Fallback** — servers without `Accept-Ranges` are fetched on a single stream.

These are well-known techniques used by download managers generally, implemented
here from scratch.

## Roadmap

- Scheduler (start/stop times, days, shut down or exit when done) and sequential
  queue processing with item reordering.
- File types list, per-category default folders, per-download properties dialog.
- 401 challenge handling (Basic + Digest) with a credentials prompt.
- Zip preview, sounds, virus-scan hook, grabber projects, localisation.
- HLS/DASH grabber with ffmpeg finalise.

## License

[MIT](LICENSE) © 2026 Ochi Download Manager contributors.

ODM is an independent, from-scratch implementation. It ships no code, binaries,
assets or data from any other product and does not bypass copy protection or
activation. Copyright protects expression, not ideas: this project claims rights
only over its own source and asserts none over any idea, algorithm or behaviour.
See [DISCLAIMER.md](DISCLAIMER.md) for the full statement.
