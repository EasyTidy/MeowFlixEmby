# 新手入门指南（Windows）

第一次用 MeowFlixEmby？照着这篇从头做一遍就能用。**全程不需要敲命令**，双击即可。

> 只想快速了解它是做什么的：在 Emby / Jellyfin 网页上点视频 → **Play On / 投放** → 选你的电脑，
> 视频就在你本机的播放器（mpv / PotPlayer / VLC / MPC-HC）里打开，进度还会回传给服务器。

---

## 你需要准备的东西

| 需要 | 说明 |
|---|---|
| 一台 Windows 电脑 | 就是你平时看片的这台 |
| Emby 或 Jellyfin 服务器地址 | 形如 `http://192.168.1.10:8096`，就是你平时打开网页用的那个 |
| 服务器的**用户名 + 密码** | ⚠️ 必须是用户名密码，**只有 API Key 是不行的**（拿不到投放资格） |
| 一个本地播放器 | 推荐 [mpv](https://mpv.io/installation/)，功能最全。也支持 PotPlayer / VLC / MPC-HC |

---

## 第 1 步：下载并解压

1. 打开项目的 **Releases** 页面，下载 `meowflix_版本号_windows_amd64.zip`。
   - 如果你的电脑是 ARM 架构（比如骁龙的 Surface），下 `windows_arm64` 那个。绝大多数人选 **amd64**。
2. **右键 → 全部解压缩**，解压到一个固定位置，比如 `C:\meowflix`。

> ⚠️ **不要直接在压缩包里双击运行。** Windows 会把它解到临时目录，配置改了会丢，还可能报错。
> 一定要先解压出来。

解压后你会看到这些文件：

```
meowflix.exe            主程序
meowflix.example.yaml   配置模板
1-首次设置.bat          ← 从这里开始
2-启动.bat
3-开机自启-安装.bat
3-开机自启-卸载.bat
deploy\                 高级用法脚本
```

---

## 第 2 步：双击「1-首次设置.bat」

它会做两件事：

1. 帮你把 `meowflix.example.yaml` 复制成 `meowflix.yaml`（你的正式配置文件）。
2. 自动用记事本打开这个配置文件，等你改完。

### 配置文件至少要改这 3 处

打开的记事本里，找到最上面的 `server:` 段落：

```yaml
server:
  type: emby                              # 用 Jellyfin 就把 emby 改成 jellyfin
  address: https://emby.example.com       # ← 改成你的服务器地址
  username: alice                         # ← 改成你的用户名
  password: ""                            # ← 引号中间填你的密码
```

改完长这样（举例）：

```yaml
server:
  type: emby
  address: http://192.168.1.10:8096
  username: rick
  password: "我的密码"
```

**填写注意事项：**

- 地址**必须带 `http://` 或 `https://` 开头**，末尾不要加 `/`。
- 密码写在两个英文引号 `"` 中间。密码里如果有特殊符号，引号不能省。
- YAML 格式对**缩进敏感**：每行前面的空格数量不要动，冒号后面要留一个空格。
- 如果你的服务器用的是自签名 HTTPS 证书（浏览器会报"不安全"），把 `skip_tls_verify: false` 改成 `true`。

### 再改一处：播放器路径

往下找到 `players:` 段落：

```yaml
players:
  default: mpv                            # 默认用哪个播放器
  exe:
    mpv: C:\Green\mpv\mpv.exe             # ← 改成你电脑上 mpv.exe 的真实路径
    potplayer: C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
    vlc: C:\Green\vlc\vlc.exe
    mpc-hc: C:\Program Files\MPC-HC\mpc-hc64.exe
```

- **只需要填你实际装了的那个**，用不到的行可以留着不管。
- 不知道路径怎么找：在开始菜单找到播放器 → 右键 → 更多 → 打开文件位置 → 右键快捷方式 → 属性 →
  「目标」那一栏就是完整路径。
- 如果 mpv 已经加进了系统 PATH，`mpv:` 那行可以直接留空。

**改完记事本记得按 Ctrl+S 保存，再关掉。**

---

## 第 3 步：双击「2-启动.bat」

会弹出一个黑色窗口，打印类似这样的日志：

```
level=INFO msg="MeowFlixEmby starting" server_type=emby device_name="MeowFlix (MY-PC)"
level=INFO msg=authenticated device_id=xxxxxxxx
```

看到 **`authenticated`** 就代表连上服务器了，成功。

> **这个黑窗口不能关**，关了程序就停了。想让它后台常驻、开机自动跑，看下面第 5 步。

---

## 第 4 步：在网页上投放

1. 用浏览器打开你的 Emby / Jellyfin 网页端（**用第 2 步配置里同一个账号登录**）。
2. 随便点开一个视频。
3. 找到播放器界面右上角（或播放控制条上）的 **投放图标**（一个屏幕带信号的图标，Emby 里叫 "Play On"）。
4. 列表里应该能看到 **`MeowFlix (你的电脑名)`**，点它。
5. 本机播放器就会自动弹出来开始播放。

之后你在**网页上**点暂停、拖进度条、切下一集，本地播放器都会跟着动；反过来你在播放器里看的进度，
也会同步回服务器（继续观看、已看标记都正常）。

---

## 第 5 步（可选）：开机自动启动

不想每次都手动双击启动，就双击 **「3-开机自启-安装.bat」**。

装好之后：

- 下次开机登录 Windows 时会自动在后台运行，**不会弹黑窗口**。
- 不需要管理员权限。
- 不想要了就双击「3-开机自启-卸载.bat」。

> 因为窗口被隐藏了，出问题时看不到日志。配置文件里 `log:` 段的 `file: ./meowflix.log`
> 默认已经开着，日志会写到程序目录下的 `meowflix.log`，用记事本打开就能看。

---

## 遇到问题怎么办

### 双击 .bat 窗口一闪就没了

正常情况下脚本结束前会停住等你按键。如果真的一闪而过，说明脚本没跑起来：

1. 确认你已经**解压**了压缩包（不是在压缩包里双击）。
2. 按住 <kbd>Shift</kbd> 右键点空白处 → 「在此处打开 PowerShell 窗口」，
   然后输入 `.\2-启动.bat` 回车，这样报错信息就不会消失。

### 报错：`load config: read config "...meowflix.yaml": The system cannot find the file specified`

你还没跑「1-首次设置.bat」，或者配置文件名字不对。文件必须叫 **`meowflix.yaml`**，
和 `meowflix.exe` 放在**同一个文件夹**里。

> 小提示：Windows 默认隐藏扩展名，你看到的 `meowflix.yaml` 有可能实际叫 `meowflix.yaml.txt`。
> 在资源管理器「查看」里勾上「文件扩展名」确认一下。

### 报错：`invalid config ...: server.address must start with http:// or https://`

地址忘了写 `http://` 前缀。改成 `http://192.168.1.10:8096` 这种形式。

### 报错：`invalid config ...: provide server.api_key, or both server.username and server.password`

用户名或密码是空的。两个都要填，密码要写在英文引号中间。

### 报错：`parse config ...: yaml: ...`

配置文件格式被改坏了，通常是**缩进被动过**或者**用了中文的冒号／引号**。
最省事的办法：删掉 `meowflix.yaml`，重新双击「1-首次设置.bat」从模板来一遍，这次只改冒号**后面**的值。

### 报错：`authenticate: ...` / 401

用户名密码不对，或者地址指向的不是那台服务器。用浏览器访问一下配置里填的那个地址，
确认能打开登录页、并且这套用户名密码能登进去。

### 网页上的投放列表里找不到我的电脑

按顺序排查：

1. 黑窗口里有没有打印 `authenticated`？没有的话是登录没成功，见上一条。
2. **网页登录的账号，和配置文件里的用户名是不是同一个？** 不同账号之间互相看不到。
3. 配置里是不是只填了 `api_key` 没填用户名密码？纯 API Key 不能作为投放目标。
4. 刷新一下网页（F5）。

### 能投放，但播放器没弹出来

播放器路径填错了。检查 `players.exe` 下面那行路径是否真实存在——
直接把路径复制到资源管理器地址栏回车，能打开播放器才算对。

### 网页上点暂停/拖进度，播放器不响应

看你用的是哪个播放器，遥控能力不一样：

| 播放器 | 遥控支持 |
|---|---|
| **mpv** | 全支持（暂停 / 快进 / 拖动 / 音量 / 字幕音轨切换 / 连播）**推荐** |
| VLC | 大部分支持 |
| MPC-HC | 大部分支持，需要先在 MPC 的「选项 → Web 界面」里勾选启用 |
| PotPlayer | **只能启动播放和停止**，不支持网页遥控 |

想要完整体验就用 mpv。

### 视频卡顿 / 走了服务器中转

MeowFlixEmby 会按这个优先级挑播放方式：**本地已挂载磁盘 → 网盘直连 → 服务器 HTTP 串流**。
如果你的片源在 NAS 上、本机也映射了网络驱动器，可以配置 `playback.path_maps`
把服务器路径映射成本地盘符，播放器就直接读盘不走网络中转了：

```yaml
playback:
  path_maps:
    - src: /mnt/disk1          # 服务器上的路径前缀
      dst: E:\                 # 你本机对应的盘符
```

用 openlist / AList 挂网盘的，见 `meowflix.yaml` 里 `openlist:` 段的注释。

---

## 想更进一步

- 配置项的完整说明：看 `meowflix.example.yaml`，每一行都有中文注释。
- 想装成 Windows 服务（不登录也运行、需要管理员权限）、Linux / macOS 自启、
  或作为共享库嵌入自己的程序：见 [README](../../README.zh-CN.md)。
- 各种命令行参数：`meowflix.exe -version`、`meowflix.exe -config 别的路径.yaml`。
