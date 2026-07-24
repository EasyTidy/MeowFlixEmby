# MeowFlixEmby

从 Emby（兼容 Jellyfin，尽量兼容 Plex）网页端把播放**投放到本地播放器**（mpv / PotPlayer / VLC / MPC…），按资源类型选择最佳播放方式，并把进度回传给服务器。**无需油猴脚本、无需改动服务器。**

## 它如何工作

MeowFlixEmby 作为常驻守护进程，通过 WebSocket 把自己注册成 Emby 的一个"可遥控/可投放会话"。在网页端点视频 → **Play On / 投放** → 选中本机，服务器即把播放指令推给本进程，由它拉起本地播放器播放。

按资源类型自动选择播放方式：

- **网盘挂载资源**（strm / 可直连 http 源）→ 播放器**直连网盘 URL**，绕过服务器中转。
- **NAS 硬盘且本地已挂载为磁盘** → 用**本地磁盘路径**直接播放。
- **其余** → 走服务器 **HTTP Direct Stream**。

## 设计文档

完整方案见 [docs/](docs/)：

1. [需求与背景分析](docs/01-需求与背景分析.md)
2. [方案选型与语言论证](docs/02-方案选型与语言论证.md)
3. [架构设计](docs/03-架构设计.md)
4. [Go 工程分层与规范](docs/04-Go%20工程分层与规范.md)
5. [实施计划](docs/05-实施计划.md)

## 快速开始（开发中）

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
scripts/build.sh all      # windows/linux/darwin，amd64+arm64
```

版本信息通过 `-ldflags` 注入，运行 `meowflix -version` 查看。

## 运行方式

三种方式任选其一：

1. **前台运行**（默认，无需安装）：直接 `meowflix -config meowflix.yaml`，Ctrl+C 退出。
2. **开机登录自启**（推荐日常使用，无需管理员）：见下方"Windows 开机启动项"。
3. **Windows 服务**（后台常驻，需管理员）：见下方"Windows 服务"。

方式 2 与 3 二选一，不要同时启用。

### Windows 开机启动项（免管理员）

注册到当前用户的登录启动项，运行在你的交互桌面会话中（能拉起播放器），默认隐藏控制台窗口：

```powershell
deploy\windows\setup-autostart.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
deploy\windows\setup-autostart.ps1 -Action status
deploy\windows\setup-autostart.ps1 -Action uninstall
# 需要看到控制台窗口时加 -ShowWindow
```

窗口隐藏后请在配置中设置 `log.file` 以保留日志。

### Windows 服务

**Windows 服务**（需管理员权限）：

```powershell
# 直接用内置命令
meowflix.exe -service install -config C:\meowflix\meowflix.yaml
meowflix.exe -service start
meowflix.exe -service status
meowflix.exe -service stop
meowflix.exe -service uninstall

# 或用封装脚本
deploy\windows\install-service.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
```

服务以自动启动方式注册，并配置失败自动重启。日志建议在配置中设置 `log.file`，因为服务无控制台。

**Linux（systemd 用户级）**：见 [deploy/systemd/meowflix.service](deploy/systemd/meowflix.service)。

**macOS（launchd）**：见 [deploy/launchd/com.easytidy.meowflix.plist](deploy/launchd/com.easytidy.meowflix.plist)。

## 作为共享库嵌入（FFI）

可编译为 C 共享库供原生宿主（Electron/Tauri sidecar、Qt/WinUI 等）嵌入：

```bash
scripts/build-shared.sh   # 产出 dist/{meowflix.dll|libmeowflix.so|libmeowflix.dylib} + meowflix.h
```

导出的生命周期 API 与事件回调 JSON 结构见 [api/ffi/EVENTS.md](api/ffi/EVENTS.md)。

## 状态

按 [实施计划](docs/05-实施计划.md) 分 M0–M6 推进。当前：**M0–M6 全部完成**。

- **M0–M2**：配置、Emby 客户端、鉴权与能力上报。
- **M3**：WebSocket 遥控会话——网页端 "Play On" 投放本机。
- **M4**：mpv JSON IPC 驱动（完整遥控）。
- **M5**：端到端闭环——resolver 决策 → 拉起播放器 → 周期回传进度 → 遥控（暂停/快进/Seek/停止）与连播下一集；openlist 直连网盘策略。
- **M6**：跨平台构建脚本、Windows 服务 / systemd / launchd 自启、FFI c-shared 导出；播放器驱动扩展到 mpv / VLC / PotPlayer / MPC-HC / generic。

详见 [CHANGELOG.md](CHANGELOG.md)。

## 许可

MIT
