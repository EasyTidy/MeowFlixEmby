<h1 align="center">
  MeowFlixEmby 🎬
</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey" alt="Platform">
  <img src="https://img.shields.io/badge/server-Emby%20%7C%20Jellyfin-52B54B" alt="Media Server">
</p>

<p align="center">
  <strong>Cast Emby / Jellyfin playback from the web UI to your local media player.<br>
  Automatic best-path routing with progress sync back to the server.</strong><br>
  <sub>No userscripts · No server modifications · mpv / VLC / PotPlayer / MPC-HC</sub>
</p>

<p align="center">
  <a href="README.zh-CN.md">🇨🇳 中文文档</a> &nbsp;·&nbsp;
  <a href="docs/Getting-Started-Windows.md">📖 Getting Started (Windows)</a>
</p>

---

## Table of Contents

- [How It Works](#how-it-works)
- [Key Features](#key-features)
- [Quick Start](#quick-start)
- [Build](#build)
- [Running Modes](#running-modes)
- [FFI Embedding](#ffi-embedding)
- [Design Docs](#design-docs)
- [License](#license)

## How It Works

MeowFlixEmby runs as a persistent daemon. It registers itself as a controllable/castable session with Emby/Jellyfin via WebSocket. Click play on the web UI → **Play On / Cast** → select this device, and the server pushes the playback command to this process, which then launches your local media player.

### Smart Playback Routing

Automatically selects the best playback path based on resource type:

| Priority | Resource Type | Playback Method |
|:---:|:---|:---|
| 1 | Cloud drive resources (strm / direct HTTP source) | **Direct cloud drive URL**, bypassing server relay |
| 2 | NAS drive mounted locally | **Local disk path** playback |
| 3 | Everything else | **HTTP Direct Stream** via server |

## Key Features

- **Remote Cast** — Click "Play On" in the web UI, local player launches automatically
- **Smart Routing** — 4-tier strategy: DirectDisk > Openlist Direct > DirectURL > HTTP Stream
- **Multi-Player Support** — Full support for mpv (full remote control), VLC, MPC-HC, PotPlayer, generic players
- **Playback Control** — Pause / Fast-Forward / Seek / Volume / Mute / Subtitle & Audio Track switching
- **Auto-Play Next + Progress Sync** — Auto-plays next episode at ≥90% completion; periodically reports progress back to server
- **Cross-Platform Auto-Start** — Windows service / user-level startup (no admin), Linux systemd, macOS launchd
- **Non-Intrusive** — No server-side modifications, no userscripts required
- **FFI Embedding** — Compiles to a C shared library for integration with Electron/Tauri/Qt hosts

## Quick Start

```bash
cp configs/meowflix.example.yaml meowflix.yaml
# Edit meowflix.yaml with your server address and credentials
go run ./cmd/meowflix -config meowflix.yaml
```

## Build

```bash
go build ./...
go test -race ./...

# Cross-platform build (output in dist/)
scripts/build.sh          # current platform
scripts/build.sh all      # windows/linux/darwin, amd64 + arm64
```

Version info is injected via `-ldflags`. Run `meowflix -version` to view.

### GitHub Actions Releases

Push a `v*` tag to trigger [release.yml](.github/workflows/release.yml), which automatically builds and publishes a GitHub Release:

| Artifact | Description |
|:---|:---|
| **Binaries** | windows/linux/darwin × amd64/arm64 with sample config & startup scripts. Windows `.zip`, others `.tar.gz` |
| **FFI Libraries** | CGO-built `.dll` / `.so` / `.dylib` + header files for each platform |
| **Checksums** | `SHA256SUMS.txt` |

```bash
git tag v1.0.0
git push origin v1.0.0   # triggers release workflow; tags with `-` are marked as prerelease
```

## Running Modes

Choose one mode. Auto-start and Windows Service are **mutually exclusive** — do not enable both.

### 1. Foreground (default)

```bash
meowflix -config meowflix.yaml   # Ctrl+C to exit
```

### 2. Windows User-Level Auto-Start (recommended, no admin)

Registers as a login startup item for the current user. Console window is hidden by default:

```powershell
deploy\windows\setup-autostart.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
deploy\windows\setup-autostart.ps1 -Action status
deploy\windows\setup-autostart.ps1 -Action uninstall
```

Since the console is hidden, set `log.file` in your config to persist logs.

### 3. Windows Service (requires admin)

```powershell
# Built-in commands
meowflix.exe -service install -config C:\meowflix\meowflix.yaml
meowflix.exe -service start
meowflix.exe -service stop
meowflix.exe -service uninstall

# Or via wrapper script
deploy\windows\install-service.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
```

Registered as auto-start with automatic restart on failure. Set `log.file` in your config since the service has no console.

### Linux (systemd user-level)

See [deploy/systemd/meowflix.service](deploy/systemd/meowflix.service).

### macOS (launchd)

See [deploy/launchd/com.easytidy.meowflix.plist](deploy/launchd/com.easytidy.meowflix.plist).

## FFI Embedding

Build as a C shared library for embedding in native hosts:

```bash
scripts/build-shared.sh   # produces dist/{meowflix.dll,libmeowflix.so,libmeowflix.dylib} + meowflix.h
```

Ideal for Electron/Tauri sidecars, Qt/WinUI, etc. Exported lifecycle APIs and event callback JSON schemas are documented in [api/ffi/EVENTS.md](api/ffi/EVENTS.md).

## Design Docs

| No. | Document |
|:---:|:---|
| 0 | [Getting Started Guide (Windows)](docs/Getting-Started-Windows.md) |
| 1 | [Requirements & Background](docs/01-需求与背景分析.md) |
| 2 | [Architecture Decision & Language Rationale](docs/02-方案选型与语言论证.md) |
| 3 | [Architecture Design](docs/03-架构设计.md) |
| 4 | [Go Project Layering & Conventions](docs/04-Go%20工程分层与规范.md) |
| 5 | [Implementation Plan](docs/05-实施计划.md) |

## License

[MIT](LICENSE)
