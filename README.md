<!--
  语言导航 / Language Switcher
  中文用户 → 往下看中文区 | English users → scroll down for English section
-->

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
  <strong>将 Emby / Jellyfin 网页端的播放投放到本地播放器，自动选择最佳播放路径，并回传播放进度。</strong><br>
  <sub>无需油猴脚本 · 无需改动服务器 · 支持 mpv / VLC / PotPlayer / MPC-HC</sub>
</p>

---

## 📑 目录 / Table of Contents

- [中文文档](#中文文档)
  - [它如何工作](#它如何工作)
  - [核心特性](#核心特性)
  - [快速开始](#快速开始)
  - [构建](#构建)
  - [运行方式](#运行方式)
  - [FFI 嵌入](#ffi-嵌入)
  - [开发状态](#开发状态)
  - [设计文档](#设计文档)
  - [许可](#许可)
- [English Documentation](#english-documentation)
  - [How It Works](#how-it-works)
  - [Key Features](#key-features)
  - [Quick Start](#quick-start)
  - [Build](#build)
  - [Running Modes](#running-modes)
  - [FFI Embedding](#ffi-embedding)
  - [Development Status](#development-status)
  - [Design Docs](#design-docs)
  - [License](#license)

---

# 中文文档

> 🐣 **第一次使用 / 不熟悉命令行？** 看 👉 **[新手入门指南](docs/00-新手入门指南.md)**  
> 下载解压后依次双击 `1-首次设置.bat` → `2-启动.bat` 即可，全程不用敲命令。

## 它如何工作

MeowFlixEmby 是一个常驻守护进程，通过 WebSocket 将自己注册为 Emby/Jellyfin 的一个"可遥控/可投放会话"。在网页端点视频 → **Play On / 投放** → 选中本机，服务器即把播放指令推送给本进程，由它拉起本地播放器播放。

### 智能播放策略

根据资源类型自动选择最佳播放方式：

| 优先级 | 资源类型 | 播放方式 |
|:---:|:---|:---|
| 1 | 网盘挂载资源（strm / 可直连 HTTP 源） | 播放器**直连网盘 URL**，绕过服务器中转 |
| 2 | NAS 硬盘且本地已挂载为磁盘 | 用**本地磁盘路径**直接播放 |
| 3 | 其余资源 | 走服务器 **HTTP Direct Stream** |

## 核心特性

- **远程投放** — 网页端点"Play On"，自动拉起本地播放器
- **智能路由** — DirectDisk > Openlist 直连 > DirectURL > HTTP Stream 四级策略
- **播放器驱动** — 完整支持 mpv（全遥控）、VLC、MPC-HC、PotPlayer、通用播放器
- **播放遥控** — 暂停/快进/Seek/音量/静音/字幕切换/音轨切换
- **连播 + 进度回传** — 当前集 ≥90% 自动播放下一集，周期性回传播放进度给服务器
- **跨平台自启** — Windows 服务/开机自启（免管理员）、Linux systemd、macOS launchd
- **无侵入** — 不修改服务器任何文件，无需油猴脚本
- **FFI 嵌入** — 可编译为 C 共享库，供 Electron/Tauri/Qt 等原生宿主集成

## 快速开始

```bash
cp configs/meowflix.example.yaml meowflix.yaml
# 编辑 meowflix.yaml 填入服务器地址与账号
go run ./cmd/meowflix -config meowflix.yaml
```

## 构建

```bash
go build ./...
go test -race ./...

# 跨平台构建（产物在 dist/）
scripts/build.sh          # 当前平台
scripts/build.sh all      # windows/linux/darwin, amd64 + arm64
```

版本信息通过 `-ldflags` 注入，运行 `meowflix -version` 查看。

### GitHub Actions 自动发版

推送 `v*` 标签即触发 [release.yml](.github/workflows/release.yml)，自动构建并发布 GitHub Release：

| 产物 | 说明 |
|:---|:---|
| **可执行文件** | windows/linux/darwin × amd64/arm64，含示例配置与自启脚本。Windows `.zip`，其余 `.tar.gz` |
| **FFI 共享库** | 各平台 CGO 构建 `.dll` / `.so` / `.dylib` + 头文件 |
| **校验和** | `SHA256SUMS.txt` |

```bash
git tag v1.0.0
git push origin v1.0.0   # 触发发行流程；带 - 的预发布标签标记为 prerelease
```

## 运行方式

三种方式任选，开机自启与 Windows 服务**二选一**，不要同时启用。

### 1. 前台运行（默认）

```bash
meowflix -config meowflix.yaml   # Ctrl+C 退出
# Windows 上不想敲命令就双击 2-启动.bat
```

### 2. Windows 开机自启（推荐，免管理员）

注册到当前用户的登录启动项，默认隐藏控制台窗口：

```powershell
deploy\windows\setup-autostart.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
deploy\windows\setup-autostart.ps1 -Action status
deploy\windows\setup-autostart.ps1 -Action uninstall
```

Windows 用户可双击 `3-开机自启-安装.bat`，无需敲命令。窗口隐藏后请在配置中设置 `log.file` 以保留日志。

### 3. Windows 服务（需管理员）

```powershell
# 内置命令
meowflix.exe -service install -config C:\meowflix\meowflix.yaml
meowflix.exe -service start
meowflix.exe -service stop
meowflix.exe -service uninstall

# 或封装脚本
deploy\windows\install-service.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
```

服务以自动启动方式注册，失败自动重启。建议配置 `log.file`（服务无控制台）。

### Linux（systemd 用户级）

见 [deploy/systemd/meowflix.service](deploy/systemd/meowflix.service)。

### macOS（launchd）

见 [deploy/launchd/com.easytidy.meowflix.plist](deploy/launchd/com.easytidy.meowflix.plist)。

### Windows 双击入口（无命令行）

发行包根目录提供四个 `.bat`：

| 文件 | 作用 |
|:---|:---|
| `1-首次设置.bat` | 从模板生成 `meowflix.yaml` 并用记事本打开 |
| `2-启动.bat` | 前台运行，出错时窗口不关闭 |
| `3-开机自启-安装.bat` | 注册登录自启（免管理员） |
| `3-开机自启-卸载.bat` | 移除登录自启 |

## FFI 嵌入

可编译为 C 共享库供原生宿主嵌入：

```bash
scripts/build-shared.sh   # 产出 dist/{meowflix.dll,libmeowflix.so,libmeowflix.dylib} + meowflix.h
```

适用场景：Electron/Tauri sidecar、Qt/WinUI 等。导出的生命周期 API 与事件回调 JSON 结构见 [api/ffi/EVENTS.md](api/ffi/EVENTS.md)。

## 开发状态

按 [实施计划](docs/05-实施计划.md) 分 M0–M6 推进，**全部完成**：

| 里程碑 | 内容 |
|:---|:---|
| M0–M2 | 配置体系、Emby 客户端、鉴权与能力上报 |
| M3 | WebSocket 遥控会话——网页端 "Play On" 投放本机 |
| M4 | mpv JSON IPC 驱动（完整遥控） |
| M5 | 端到端闭环——resolver 决策 → 拉起播放器 → 进度回传 → 遥控与连播；openlist 直连网盘 |
| M6 | 跨平台构建、自启服务（Windows/systemd/launchd）、FFI 导出、多播放器驱动 |

详见 [CHANGELOG.md](CHANGELOG.md)。

## 设计文档

| 序号 | 文档 |
|:---:|:---|
| 0 | [新手入门指南](docs/00-新手入门指南.md)（面向使用者） |
| 1 | [需求与背景分析](docs/01-需求与背景分析.md) |
| 2 | [方案选型与语言论证](docs/02-方案选型与语言论证.md) |
| 3 | [架构设计](docs/03-架构设计.md) |
| 4 | [Go 工程分层与规范](docs/04-Go%20工程分层与规范.md) |
| 5 | [实施计划](docs/05-实施计划.md) |

## 许可

[MIT](LICENSE)

---

# English Documentation

> 🐣 **First time / not comfortable with CLI?** Check out the [Getting Started Guide (Chinese)](docs/00-新手入门指南.md).

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
- **Cross-Platform Auto-Start** — Windows service / user-level startup, Linux systemd, macOS launchd
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

## Development Status

All milestones M0–M6 are **complete** per the [Implementation Plan](docs/05-实施计划.md):

| Milestone | Content |
|:---|:---|
| M0–M2 | Configuration, Emby client, authentication & capability reporting |
| M3 | WebSocket remote session — "Play On" casting from web UI |
| M4 | mpv JSON IPC driver (full remote control) |
| M5 | End-to-end loop — resolver decisions → launch player → progress sync → remote control & auto-next; openlist direct cloud drive |
| M6 | Cross-platform build, startup services (Windows/systemd/launchd), FFI export, multi-player drivers |

See [CHANGELOG.md](CHANGELOG.md) for details.

## Design Docs

| No. | Document |
|:---:|:---|
| 0 | [Getting Started Guide](docs/00-新手入门指南.md) (Chinese, for end users) |
| 1 | [Requirements & Background](docs/01-需求与背景分析.md) (Chinese) |
| 2 | [Architecture Decision & Language Rationale](docs/02-方案选型与语言论证.md) (Chinese) |
| 3 | [Architecture Design](docs/03-架构设计.md) (Chinese) |
| 4 | [Go Project Layering & Conventions](docs/04-Go%20工程分层与规范.md) (Chinese) |
| 5 | [Implementation Plan](docs/05-实施计划.md) (Chinese) |

## License

[MIT](LICENSE)
