# MeowFlixEmby — 实施计划

> 第 5 部分。上一篇：[04-Go 工程分层与规范](04-go-project-structure.md)

## 1. 里程碑

| 阶段 | 目标 | 交付物 | 验证方式 |
|------|------|------|------|
| **M0 脚手架** | Go module、目录分层、配置模型、日志、lint/CI | 可编译空跑的 `cmd/meowflix`、`configs/*.example.yaml`、接口定义 | `go build ./...`、`golangci-lint run` |
| **M1 决策引擎** | resolver + pathmap + version + subtitle（纯逻辑，无 I/O） | `pkg/resolver/*` + 表驱动单测 | `go test -race ./pkg/resolver/...` 全绿 |
| **M2 服务器适配** | Emby 认证 / 能力声明 / PlaybackInfo / 进度回传（REST） | `pkg/mediaserver/emby`、`pkg/progress` | 对真实/测试 Emby 实例：认证成功、能拿 MediaSources、能回传进度 |
| **M3 遥控会话** | WebSocket 连接 / 保活 / 收 Play·Playstate·GeneralCommand / 断线重连 | `pkg/remote` | 本机出现在网页端"Play On"；投放能收到 Play 指令 |
| **M4 播放器驱动** | mpv（IPC）优先，其次 vlc、potplayer、generic | `pkg/player/*` | 投放→本地 mpv 起播、遥控暂停/快进/停止生效 |
| **M5 编排闭环** | playback controller 串联全链路 + 进度回传 + 连播 | `pkg/playback`、`internal/app` | 端到端：网页投放→本地播放→进度回网页；下一集连播 |
| **M6 跨平台与集成** | Win/mac/Linux 构建、自启脚本、`c-shared` FFI 导出、README | `scripts/`、`api/ffi`、release 产物 | 三平台单文件运行；FFI 示例调用成功 |

## 2. M0 落地清单（本次先做）

1. `go.mod`（module 名、Go 版本 1.22+）。
2. 目录骨架与占位（见 [04](04-go-project-structure.md) §2）。
3. `internal/config`：YAML 加载 + 校验 + 环境变量覆盖。
4. `pkg/*` 核心 interface 定义（mediaserver / resolver / player / progress / remote）。
5. `cmd/meowflix/main.go`：装配骨架、`context` 生命周期、信号优雅退出、`log/slog` 初始化。
6. `configs/meowflix.example.yaml`、`.golangci.yml`、`README.md`。

## 3. 依赖选型

| 用途 | 选择 | 理由 |
|------|------|------|
| WebSocket | `github.com/coder/websocket`（原 nhooyr）或 `gorilla/websocket` | 维护活跃、context 友好 |
| YAML | `gopkg.in/yaml.v3` | 事实标准 |
| 日志 | 标准库 `log/slog` | 免依赖、结构化 |
| HTTP | 标准库 `net/http` | 够用 |
| mpv IPC | 自实现 JSON over 管道/socket（薄封装） | 无成熟通用库、需精细控制 |
| 测试断言 | 标准库 `testing`（表驱动）+ 可选 `testify` | 规范优先标准库 |

> 依赖遵循：优先标准库；第三方须活跃维护、许可兼容；`go.mod` 锁定版本。

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 遥控协议字段随服务器版本漂移 | 收指令/回传失败 | 适配层隔离；`SupportedCommands` 运行时枚举；抓包联调核实 **[待联调核实]** 项 |
| 会话空闲超时从投放列表消失 | 用户找不到目标 | 心跳保活 + 断线重连 + 重发 Capabilities |
| Jellyfin WebSocket 路径 `/socket` 未最终核实 | Jellyfin 连不上 | M3 对照 jellyfin-apiclient-python 验证 |
| 非 mpv 播放器进度回传弱 | 进度不准 | mpv 为一等公民；弱回传播放器降级策略并明确告知用户 |
| 挂载盘路径大小写/编码差异（跨 OS） | 找不到文件 | `path_check` + NFC/NFD 规范化；失败回退 HTTP 串流 |
| 投放 UX 与"点原生键即播"有差异 | 习惯差异 | 文档说明；Emby 会记住上次投放目标 |

## 5. 测试策略

- **单元**：resolver 全路径、pathmap、version、subtitle 表驱动；`-race`。
- **契约**：mediaserver / player 用接口 + mock，验证编排逻辑不依赖真实后端。
- **集成**：对接真实或 docker 化 Emby 实例，跑 M2/M3/M5 端到端脚本。
- **手动验收**：三平台各跑一次"网页投放→本地播放→进度回传→连播→遥控"。

## 6. 交付与分发

- `scripts/build.sh`：`GOOS/GOARCH` 交叉编译出 win/mac/linux 单文件。
- 自启：Windows 计划任务/启动项、macOS launchd plist、Linux systemd user unit（`scripts/` 提供模板）。
- FFI：`scripts/build-shared.sh` 产出 `.dll/.so/.dylib` + 头文件。
- 版本：SemVer；`CHANGELOG.md`；Conventional Commits。
