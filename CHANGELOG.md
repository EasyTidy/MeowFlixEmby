# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范。

## [Unreleased]

### M6 — 打包、服务化与 FFI

#### Added
- **Windows 服务**：`meowflix -service install|uninstall|start|stop|status|run`，
  基于 `golang.org/x/sys/windows/svc`。自动启动 + 失败自动重启（5s/5s/30s），
  由 SCM 启动时自动检测并进入服务模式。封装脚本 `deploy/windows/install-service.ps1`。
- **Windows 开机启动项**（免管理员）`deploy/windows/setup-autostart.ps1`：
  注册到 HKCU Run，运行在交互桌面会话，默认经 .vbs 隐藏控制台窗口；
  与服务方式二选一。前台运行始终可用（无需安装任何一种）。
- **自启模板**：systemd 用户级 unit（`deploy/systemd/meowflix.service`）、
  macOS launchd LaunchAgent（`deploy/launchd/com.easytidy.meowflix.plist`）。
- **`.vscode/settings.json`**：为 gopls 启用 CGO_ENABLED=1，消除 cgo 文件
  （`api/ffi`）中 `C.*` 符号的"undefined"误报。
- **GitHub Actions 发行流程** `.github/workflows/release.yml`：推送 `v*` 标签触发，
  跨平台构建可执行文件（6 组 OS×arch）+ 原生构建 FFI 共享库（win/linux/mac）+
  SHA256 校验和，自动发布 GitHub Release（`-` 标签标记为 prerelease）。
- **跨平台构建脚本** `scripts/build.sh`：支持 `host` / `all`（windows/linux/darwin ×
  amd64/arm64），产物在 `dist/`，通过 `-ldflags` 注入版本/commit/构建时间。
- **`-version` 标志**：打印版本、commit 与构建时间。
- **FFI c-shared 库**（`api/ffi`）：导出 `MeowflixStart` / `MeowflixStop` /
  `MeowflixIsRunning` / `MeowflixVersion` / `MeowflixLastError` /
  `MeowflixSetEventCallback` / `MeowflixFreeString`，生命周期事件以 JSON 回调交付。
  构建脚本 `scripts/build-shared.sh`，事件 schema 见 `api/ffi/EVENTS.md`。
- **文件日志**：`log.file` 配置项，控制台/服务两种启动路径共用；服务无控制台时写文件。

### M5 — 端到端闭环与直连网盘

#### Added
- 播放编排（`pkg/playback`）：resolver 决策 → 拉起播放器 → 周期回传进度 →
  遥控（暂停/快进/Seek/停止/音量/字幕/音轨）与 ≥90% 自动连播下一集。
- openlist（AList 兼容）直连网盘策略：`/api/fs/get` 取 `raw_url`；
  优先级 本地挂载磁盘（校验文件存在）→ openlist → HTTP Direct Stream 兜底。
- 播放器驱动扩展：VLC（HTTP 接口）、PotPlayer / generic（仅拉起）、
  MPC-HC（WebUI 命令 + 进度）。

### M3–M4 — 遥控会话与 mpv

#### Added
- Emby/Jellyfin WebSocket 遥控会话（`pkg/mediaserver/emby/websocket.go`），
  网页端 "Play On" 投放本机；GeneralCommand / Playstate / Play 指令解码。
- mpv JSON IPC 驱动（Windows 命名管道 / Unix socket），
  Emby 绝对流索引 → mpv track-list ff-index 映射。

### M0–M2 — 基础设施

#### Added
- 配置加载与校验（`internal/config`）、结构化日志（log/slog）。
- Emby 客户端：鉴权、能力上报、进度回传接口。
- 项目分层与接口隔离（URLBuilder / Reporter / Server），纯函数 resolver。
