<h1 align="center">
  MeowFlixEmby 🎬
</h1>

<p align="center">
  <strong>MeowFlixEmby is a lightweight Emby / Jellyfin local playback bridge.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey" alt="Platform">
  <img src="https://img.shields.io/badge/server-Emby%20%7C%20Jellyfin-52B54B" alt="Media Server">
</p>

<p align="center">
  <sub>Cast playback from the Emby / Jellyfin web UI to your local media player.<br>
  Smart routing picks the best path — direct disk, cloud drive, or server stream.<br>
  No userscripts · No server modifications · mpv / VLC / PotPlayer / MPC-HC</sub>
</p>

<p align="center">
  <a href="README.zh-CN.md">🇨🇳 中文文档</a> &nbsp;·&nbsp;
  <a href="docs/Getting-Started-Windows.md">📖 Getting Started (Windows)</a> &nbsp;·&nbsp;
  <a href="docs/Configuration.md">⚙️ Configuration</a>
</p>

---

## Table of Contents

- [How It Works](#how-it-works)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Build (from source)](#build-from-source)
- [Running Modes](#running-modes)
- [Advanced Usage](#advanced-usage)
- [Design Docs](#design-docs)
- [License](#license)

## Demo

> <!-- TODO: replace with actual screenshots / GIF -->
> ![Play On 示例](docs/images/demo.gif)
>
> **1.** Open Emby/Jellyfin in your browser → click a video → **Play On / Cast** → select **MeowFlix**
>
> **2.** Your local media player launches and starts playing
>
> **3.** Pause, seek, next episode from the web UI — all synced. Progress reported back to server.
>
> ```
> ┌─────────────────┐     WebSocket      ┌──────────────┐     IPC / CLI      ┌───────────┐
> │  Emby / Jellyfin │ ◄────────────────► │  MeowFlixEmby │ ─────────────────► │   mpv /   │
> │    (Web UI)      │   PlayOn + Remote  │   (daemon)    │   launch & control │   VLC …   │
> └─────────────────┘                    └──────────────┘                    └───────────┘
> ```

## How It Works

MeowFlixEmby runs as a background daemon. It registers itself as a castable session with Emby/Jellyfin via WebSocket. When you click **Play On / Cast** in the web UI and select your device, the server sends the playback command to this process, which launches your local media player.

### Smart Routing

Automatically picks the best playback path:

| Priority | Source | Method |
|:---:|:---|:---|
| 1 | Cloud drive (strm / direct HTTP URL) | **Direct cloud-drive URL** — bypass server relay |
| 2 | NAS drive mounted locally | **Local disk path** — direct file access |
| 3 | Everything else | **HTTP Direct Stream** via server |

## Key Features

- **Remote Cast** — Click "Play On" in the web UI, local player launches instantly
- **Smart Routing** — 4-tier strategy: DirectDisk → Openlist Direct → DirectURL → HTTP Stream
- **Multi-Player** — mpv (full remote), VLC, MPC-HC, PotPlayer, generic
- **Playback Control** — Pause / Seek / Volume / Mute / Subtitle & Audio Track switching
- **Auto-Next + Progress Sync** — Next episode at ≥90% completion; progress synced to server
- **Cross-Platform Auto-Start** — Windows service / user-level startup, Linux systemd, macOS launchd
- **Zero Server Changes** — No scripts, no plugins, no server modifications
- **FFI Embedding** — Compiles to a C shared library for Electron / Tauri / Qt hosts

## Installation

### Download Prebuilt Binary (Recommended)

Go to the [Releases](https://github.com/EasyTidy/MeowFlixEmby/releases) page and download the `.zip` for your platform:

| Platform | File |
|:---|:---|
| Windows x64 | `meowflix_<version>_windows_amd64.zip` |
| Windows ARM64 | `meowflix_<version>_windows_arm64.zip` |
| Linux x64 | `meowflix_<version>_linux_amd64.tar.gz` |
| macOS x64 | `meowflix_<version>_darwin_amd64.tar.gz` |

Extract to a permanent folder (e.g. `C:\meowflix` on Windows, `~/meowflix` on Linux/macOS).

> **Windows users:** see the [Getting Started Guide (Windows)](docs/Getting-Started-Windows.md) for a step-by-step walkthrough — no CLI needed.

### Install from Source

Requires [Go 1.25+](https://go.dev/dl/).

```bash
git clone https://github.com/EasyTidy/MeowFlixEmby.git
cd MeowFlixEmby
go build ./cmd/meowflix
```

## Quick Start

```bash
# 1. Create your config from the template
cp configs/meowflix.example.yaml meowflix.yaml

# 2. Edit meowflix.yaml — at minimum, set your server address and credentials:
#    server:
#      type: emby             # emby or jellyfin
#      address: http://192.168.1.10:8096
#      username: your-name
#      password: "your-password"

# 3. Run
./meowflix -config meowflix.yaml
```

You should see `authenticated` in the console. Open your Emby/Jellyfin web UI, play a video,
click **Play On / Cast**, and select your device.

## Configuration

See the full **[Configuration Guide](docs/Configuration.md)** for every option.

The minimal config only needs the `server` block:

```yaml
server:
  type: emby
  address: http://your-server:8096
  username: your-name
  password: "your-password"
```

> **Important:** Only **username + password** works as a cast target. API Key alone cannot be discovered.

## Build (from Source)

```bash
go build ./...
go test -race ./...

# Cross-platform build → dist/
scripts/build.sh          # current platform
scripts/build.sh all      # windows/linux/darwin, amd64 + arm64
```

Version info is injected via `-ldflags`. Run `meowflix -version` to view.

### GitHub Actions Releases

Push a `v*` tag to trigger [release.yml](.github/workflows/release.yml):

| Artifact | Description |
|:---|:---|
| **Binaries** | windows/linux/darwin × amd64/arm64. Windows `.zip`, others `.tar.gz` |
| **FFI Libraries** | CGO-built `.dll` / `.so` / `.dylib` + header files |
| **Checksums** | `SHA256SUMS.txt` |

```bash
git tag v1.0.0
git push origin v1.0.0   # Tags with `-` (e.g. v1.0.0-rc1) are marked as prerelease
```

## Running Modes

Choose one. Auto-start and Windows Service are **mutually exclusive**.

### 1. Foreground (default)

```bash
meowflix -config meowflix.yaml   # Ctrl+C to exit
```

### 2. Windows User-Level Auto-Start (recommended, no admin)

```powershell
deploy\windows\setup-autostart.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
deploy\windows\setup-autostart.ps1 -Action status
deploy\windows\setup-autostart.ps1 -Action uninstall
```

Console hidden by default. Set `log.file` in your config to persist logs.

### 3. Windows Service (requires admin)

```powershell
meowflix.exe -service install -config C:\meowflix\meowflix.yaml
meowflix.exe -service start
meowflix.exe -service stop
meowflix.exe -service uninstall
```

Auto-start with restart on failure. Set `log.file` — no console.

### Linux (systemd user-level)

See [deploy/systemd/meowflix.service](deploy/systemd/meowflix.service).

### macOS (launchd)

See [deploy/launchd/com.easytidy.meowflix.plist](deploy/launchd/com.easytidy.meowflix.plist).

## Advanced Usage

### FFI Embedding

Build as a C shared library for embedding in native hosts:

```bash
scripts/build-shared.sh   # produces dist/{meowflix.dll,libmeowflix.so,libmeowflix.dylib} + meowflix.h
```

Ideal for Electron/Tauri sidecars, Qt/WinUI, etc. See [api/ffi/EVENTS.md](api/ffi/EVENTS.md) for the event callback JSON schema.

## Design Docs

| No. | Document (English) | 中文 |
|:---:|:---|:---|
| 0 | [Getting Started Guide (Windows)](docs/Getting-Started-Windows.md) | [新手入门指南](docs/zh-CN/Getting-Started-Windows.md) |
| 1 | [Requirements & Background](docs/01-requirements-and-background.md) | [需求与背景分析](docs/zh-CN/01-requirements-and-background.md) |
| 2 | [Architecture Decision & Language Rationale](docs/02-architecture-and-language-choice.md) | [方案选型与语言论证](docs/zh-CN/02-architecture-and-language-choice.md) |
| 3 | [Architecture Design](docs/03-architecture-design.md) | [架构设计](docs/zh-CN/03-architecture-design.md) |
| 4 | [Go Project Layering & Conventions](docs/04-go-project-structure.md) | [Go 工程分层与规范](docs/zh-CN/04-go-project-structure.md) |
| 5 | [Implementation Plan](docs/05-implementation-plan.md) | [实施计划](docs/zh-CN/05-implementation-plan.md) |
|     | [Configuration Guide](docs/Configuration.md) | [配置指南](docs/zh-CN/Configuration.md) |

## License

[MIT](LICENSE)
