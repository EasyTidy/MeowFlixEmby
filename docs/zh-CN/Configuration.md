# 配置指南

MeowFlixEmby 通过单个 YAML 文件（`meowflix.yaml`）进行配置。复制模板开始：

```bash
cp configs/meowflix.example.yaml meowflix.yaml
```

敏感字段可以通过环境变量覆盖，避免在磁盘上存储凭据：

| 变量 | 字段 |
|:---|:---|
| `MEOWFLIX_SERVER_ADDRESS` | `server.address` |
| `MEOWFLIX_USERNAME` | `server.username` |
| `MEOWFLIX_PASSWORD` | `server.password` |
| `MEOWFLIX_API_KEY` | `server.api_key` |
| `MEOWFLIX_OPENLIST_TOKEN` | `openlist.token` |

---

## 快速开始（最小配置）

最少只需要改 3 处：

```yaml
server:
  type: emby                  # emby | jellyfin
  address: http://192.168.1.10:8096   # ← 你的服务器地址
  username: alice             # ← 你的用户名
  password: "your-password"   # ← 你的密码
```

> **注意：** 只有 **用户名 + 密码** 才能作为 Play On / 投放目标。
> 纯 API Key 无法注册为可投放设备。

---

## 完整配置参考

### `server` — 媒体服务器连接

```yaml
server:
  type: emby                              # emby | jellyfin
  address: https://emby.example.com       # 必填 — 必须以 http:// 或 https:// 开头
  username: alice
  password: ""
  api_key: ""                             # 可选备用（不能作为投放目标）
  device_name: "MeowFlix (客厅电脑)"       # 在 Play On / 投放列表中显示的名称
  skip_tls_verify: false                  # 自签名证书时设为 true
```

### `playback` — 路径映射与路由

```yaml
playback:
  # 服务器路径 → 本地挂载盘映射（NAS 场景）
  path_maps:
    - src: /mnt/disk1          # 服务器上的路径前缀
      dst: E:\                 # 本地盘符或挂载点
    - src: /mnt/disk2/media
      dst: F:\media

  # 命中这些前缀时强制走本地磁盘播放
  force_disk_prefixes: []

  # strm / 网盘源的直连域名
  direct_url_hosts: []

  path_check: false            # 路径转换后校验文件是否真实存在（支持 NFC/NFD）
  one_instance: false          # 限制播放器单实例运行
```

**路由优先级：** 本地已挂载磁盘（经 `path_check` 验证）→ Openlist 直连 → 服务器 HTTP 串流。

### `players` — 播放器选择与偏好

```yaml
players:
  default: mpv                  # 默认播放器
  fullscreen: false             # 启动时全屏播放

  # 按路径关键字路由到不同播放器
  by_path:
    - player: vlc
      match: [".iso", "__bdmv"]

  # 播放器路径 — 只填实际安装了的
  exe:
    mpv: C:\Green\mpv\mpv.exe
    vlc: C:\Green\vlc\vlc.exe
    mpc-hc: C:\Program Files\MPC-HC\mpc-hc64.exe
    potplayer: C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
    # generic: C:\Path\To\AnyPlayer.exe   # 可选：任意播放器（只启动不遥控）
```

| 播放器 | 遥控支持 | 备注 |
|:---|:---|:---|
| **mpv** | 全支持（暂停、快进、音量、静音、字幕/音轨、消息、连播） | 推荐 |
| VLC | 大部分（HTTP 接口：暂停、快进、音量、音轨、停止） | DisplayMessage 无声忽略 |
| MPC-HC | 大部分（Web 接口：暂停、快进、音量、静音、停止、上一集/下一集） | 需在 MPC 选项里启用 Web 界面 |
| PotPlayer | 仅启动/停止 | 无运行时控制与进度同步 |
| Generic | 仅启动/停止 | 任意可执行文件 |

### `openlist` — 网盘直连（兼容 AList）

让播放器直接串流网盘，绕过服务器中转。

```yaml
openlist:
  host: ""                     # 例：http://192.168.31.10:5255 — 留空禁用
  token: ""                    # AList API 密钥

  # 服务器路径前缀 → openlist 路径前缀
  # dst 可以为空（根路径映射）
  path_maps:
    - src: /volume1/video
      dst: ""
```

### `subtitle` — 字幕选择

```yaml
subtitle:
  # 优先级顺序 — 首个关键字匹配即选中
  priority: ["中英", "双语", "简", "chi", "ass", "srt"]
```

### `version` — 多版本选择

```yaml
version:
  # 多版本媒体的优先级（如不同分辨率）
  prefer: ["remux", "2160", "1080"]
```

### `log` — 日志

```yaml
log:
  level: info                  # debug | info | warn | error
  file: ./meowflix.log
  mask_sensitive: true         # 日志中脱敏 token/域名
```

---

## Windows：双击设置（免命令行）

如果你下载的是 release `.zip`，解压后双击 `1-首次设置.bat`。
它会从模板创建 `meowflix.yaml` 并用记事本打开。

完整步骤见 [新手入门指南（Windows）](Getting-Started-Windows.md)。

---

## 运行

```bash
# 前台运行（Ctrl+C 停止）
meowflix -config meowflix.yaml

# 指定其他配置路径
meowflix -config /path/to/custom.yaml

# 查看版本
meowflix -version
```

关于自启动，见主 README 中的[运行方式](../../README.zh-CN.md#运行方式)。
