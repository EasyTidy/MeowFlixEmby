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
```

## 状态

按 [实施计划](docs/05-实施计划.md) 分 M0–M6 推进。当前：**M0 脚手架**。

## 许可

MIT
