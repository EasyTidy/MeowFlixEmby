<h1 align="center">
  MeowFlixEmby 🎬
</h1>

<p align="center">
  <strong>MeowFlixEmby 是一个轻量级 Emby / Jellyfin 本地播放桥接工具。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey" alt="Platform">
  <img src="https://img.shields.io/badge/server-Emby%20%7C%20Jellyfin-52B54B" alt="Media Server">
</p>

<p align="center">
  <sub>将 Emby / Jellyfin 网页端的播放投放到本地播放器。<br>
  智能路由自动选择最佳播放路径：本地磁盘 → 网盘直连 → 服务器串流。<br>
  无需油猴脚本 · 无需改动服务器 · 支持 mpv / VLC / PotPlayer / MPC-HC</sub>
</p>

<p align="center">
  <a href="README.md">🇬🇧 English</a> &nbsp;·&nbsp;
  <a href="docs/zh-CN/Getting-Started-Windows.md">📖 新手入门指南（Windows）</a> &nbsp;·&nbsp;
  <a href="docs/zh-CN/Configuration.md">⚙️ 配置指南</a>
</p>

---

## 目录

- [它如何工作](#它如何工作)
- [核心特性](#核心特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [配置](#配置)
- [从源码构建](#从源码构建)
- [运行方式](#运行方式)
- [高级用法](#高级用法)
- [设计文档](#设计文档)
- [许可](#许可)

## 效果演示

> <!-- TODO: 替换为实际截图 / GIF -->
> ![Play On 示例](docs/images/demo.gif)
>
> **1.** 浏览器打开 Emby/Jellyfin → 点视频 → **Play On / 投放** → 选 **MeowFlix**
>
> **2.** 本地播放器自动弹出来开始播放
>
> **3.** 网页上暂停、拖进度、切下一集全同步，播放进度自动回传服务器
>
> ```
> ┌─────────────────┐     WebSocket      ┌──────────────┐     IPC / CLI      ┌───────────┐
> │  Emby / Jellyfin │ ◄────────────────► │  MeowFlixEmby │ ─────────────────► │   mpv /   │
> │    (网页端)       │   PlayOn + 遥控    │   (守护进程)   │   启动 + 遥控       │   VLC …   │
> └─────────────────┘                    └──────────────┘                    └───────────┘
> ```

## 它如何工作

MeowFlixEmby 是一个常驻守护进程，通过 WebSocket 将自己注册为 Emby/Jellyfin 的可投放会话。网页端点视频 → **Play On / 投放** → 选中本机，服务器即把播放指令推送给本进程，由它拉起本地播放器。

### 智能路由

根据资源类型自动选择最佳播放路径：

| 优先级 | 来源 | 播放方式 |
|:---:|:---|:---|
| 1 | 网盘挂载资源（strm / 可直连 HTTP 源） | **直连网盘 URL** — 绕过服务器中转 |
| 2 | NAS 硬盘且本地已挂载为磁盘 | **本地磁盘路径** — 直接文件访问 |
| 3 | 其余资源 | **HTTP Direct Stream** — 服务器串流 |

## 核心特性

- **远程投放** — 网页端点"Play On"，本地播放器自动拉起
- **智能路由** — DirectDisk → Openlist 直连 → DirectURL → HTTP Stream 四级策略
- **多播放器** — mpv（全遥控）、VLC、MPC-HC、PotPlayer、通用播放器
- **播放遥控** — 暂停 / 快进 / Seek / 音量 / 静音 / 字幕音轨切换
- **连播 + 进度回传** — ≥90% 自动下一集，进度同步回服务器
- **跨平台自启** — Windows 服务 / 开机自启（免管理员）、Linux systemd、macOS launchd
- **零侵入** — 不修改服务器任何文件，无需油猴脚本
- **FFI 嵌入** — 可编译为 C 共享库，供 Electron / Tauri / Qt 集成

## 安装

### 下载预编译包（推荐）

从 [Releases](https://github.com/EasyTidy/MeowFlixEmby/releases) 页面下载对应平台的压缩包：

| 平台 | 文件 |
|:---|:---|
| Windows x64 | `meowflix_<版本>_windows_amd64.zip` |
| Windows ARM64 | `meowflix_<版本>_windows_arm64.zip` |
| Linux x64 | `meowflix_<版本>_linux_amd64.tar.gz` |
| macOS x64 | `meowflix_<版本>_darwin_amd64.tar.gz` |

解压到固定目录（如 Windows 上 `C:\meowflix`，Linux/macOS 上 `~/meowflix`）。

> **Windows 用户：** 看 👉 [新手入门指南（Windows）](docs/zh-CN/Getting-Started-Windows.md)，全程双击即可，无需敲命令。

### 从源码安装

需要 [Go 1.25+](https://go.dev/dl/)。

```bash
git clone https://github.com/EasyTidy/MeowFlixEmby.git
cd MeowFlixEmby
go build ./cmd/meowflix
```

## 快速开始

```bash
# 1. 从模板创建配置
cp configs/meowflix.example.yaml meowflix.yaml

# 2. 编辑 meowflix.yaml — 最少只改 3 处：
#    server:
#      type: emby             # emby 或 jellyfin
#      address: http://192.168.1.10:8096
#      username: 你的用户名
#      password: "你的密码"

# 3. 运行
./meowflix -config meowflix.yaml
```

看到 `authenticated` 即连接成功。打开 Emby/Jellyfin 网页端，点播放 → **Play On / 投放** → 选你的设备即可。

## 配置

完整配置项见 **[配置指南](docs/zh-CN/Configuration.md)**。

最小配置只需填 `server` 段：

```yaml
server:
  type: emby
  address: http://你的服务器:8096
  username: 你的用户名
  password: "你的密码"
```

> **注意：** 必须填**用户名 + 密码**才能作为投放目标，纯 API Key 不行。

## 从源码构建

```bash
go build ./...
go test -race ./...

# 跨平台构建 → dist/
scripts/build.sh          # 当前平台
scripts/build.sh all      # windows/linux/darwin, amd64 + arm64
```

版本信息通过 `-ldflags` 注入，运行 `meowflix -version` 查看。

### GitHub Actions 自动发版

推送 `v*` 标签触发 [release.yml](.github/workflows/release.yml)：

| 产物 | 说明 |
|:---|:---|
| **可执行文件** | windows/linux/darwin × amd64/arm64。Windows `.zip`，其余 `.tar.gz` |
| **FFI 共享库** | CGO 构建 `.dll` / `.so` / `.dylib` + 头文件 |
| **校验和** | `SHA256SUMS.txt` |

```bash
git tag v1.0.0
git push origin v1.0.0   # 带 - 的标签（如 v1.0.0-rc1）标记为 prerelease
```

## 运行方式

三种方式任选，开机自启与 Windows 服务**二选一**。

### 1. 前台运行（默认）

```bash
meowflix -config meowflix.yaml   # Ctrl+C 退出
```

Windows 用户可双击 `2-启动.bat`，无需敲命令。

### 2. Windows 开机自启（推荐，免管理员）

```powershell
deploy\windows\setup-autostart.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
deploy\windows\setup-autostart.ps1 -Action status
deploy\windows\setup-autostart.ps1 -Action uninstall
```

默认隐藏控制台窗口，双击 `3-开机自启-安装.bat` 即可。建议设置 `log.file` 保留日志。

### 3. Windows 服务（需管理员）

```powershell
meowflix.exe -service install -config C:\meowflix\meowflix.yaml
meowflix.exe -service start
meowflix.exe -service stop
meowflix.exe -service uninstall
```

自动启动注册，失败自动重启。建议配置 `log.file`（服务无控制台）。

### Windows 双击入口（无需命令行）

| 文件 | 作用 |
|:---|:---|
| `1-首次设置.bat` | 从模板生成配置并用记事本打开 |
| `2-启动.bat` | 前台运行，出错时窗口不关闭 |
| `3-开机自启-安装.bat` | 注册登录自启（免管理员） |
| `3-开机自启-卸载.bat` | 移除登录自启 |

### Linux（systemd 用户级）

见 [deploy/systemd/meowflix.service](deploy/systemd/meowflix.service)。

### macOS（launchd）

见 [deploy/launchd/com.easytidy.meowflix.plist](deploy/launchd/com.easytidy.meowflix.plist)。

## 高级用法

### FFI 嵌入

编译为 C 共享库供原生宿主嵌入：

```bash
scripts/build-shared.sh   # 产出 dist/{meowflix.dll,libmeowflix.so,libmeowflix.dylib} + meowflix.h
```

适用：Electron/Tauri sidecar、Qt/WinUI 等。事件回调 JSON 结构见 [api/ffi/EVENTS.md](api/ffi/EVENTS.md)。

## 设计文档

| 序号 | 中文文档 | English |
|:---:|:---|:---|
| 0 | [新手入门指南（Windows）](docs/zh-CN/Getting-Started-Windows.md) | [Getting Started](docs/Getting-Started-Windows.md) |
| 1 | [需求与背景分析](docs/zh-CN/01-requirements-and-background.md) | [Requirements & Background](docs/01-requirements-and-background.md) |
| 2 | [方案选型与语言论证](docs/zh-CN/02-architecture-and-language-choice.md) | [Architecture Decision & Language Rationale](docs/02-architecture-and-language-choice.md) |
| 3 | [架构设计](docs/zh-CN/03-architecture-design.md) | [Architecture Design](docs/03-architecture-design.md) |
| 4 | [Go 工程分层与规范](docs/zh-CN/04-go-project-structure.md) | [Go Project Layering & Conventions](docs/04-go-project-structure.md) |
| 5 | [实施计划](docs/zh-CN/05-implementation-plan.md) | [Implementation Plan](docs/05-implementation-plan.md) |
|     | [配置指南](docs/zh-CN/Configuration.md) | [Configuration Guide](docs/Configuration.md) |

## 许可

[MIT](LICENSE)
