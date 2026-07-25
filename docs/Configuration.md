# Configuration Guide

MeowFlixEmby is configured via a single YAML file (`meowflix.yaml`). Copy the example template
to get started:

```bash
cp configs/meowflix.example.yaml meowflix.yaml
```

Sensitive fields can be overridden via environment variables so you don't have to store
credentials on disk:

| Variable | Field |
|:---|:---|
| `MEOWFLIX_SERVER_ADDRESS` | `server.address` |
| `MEOWFLIX_USERNAME` | `server.username` |
| `MEOWFLIX_PASSWORD` | `server.password` |
| `MEOWFLIX_API_KEY` | `server.api_key` |
| `MEOWFLIX_OPENLIST_TOKEN` | `openlist.token` |

---

## Quick Start (Minimal Config)

At minimum you only need to change 3 things:

```yaml
server:
  type: emby                  # emby | jellyfin
  address: http://192.168.1.10:8096   # ← Your server URL
  username: alice             # ← Your username
  password: "your-password"   # ← Your password
```

> **Note:** Only **username + password** works as a Play On / Cast target.
> API Key alone cannot register as a castable device.

---

## Full Configuration Reference

### `server` — Media Server Connection

```yaml
server:
  type: emby                              # emby | jellyfin
  address: https://emby.example.com       # Required — must start with http:// or https://
  username: alice
  password: ""
  api_key: ""                             # Optional fallback (cannot act as cast target)
  device_name: "MeowFlix (Living Room)"   # Name shown in the Play On / Cast list
  skip_tls_verify: false                  # Set to true for self-signed certs
```

### `playback` — Path Mapping & Routing

```yaml
playback:
  # Map server-side paths to locally mounted drives (NAS scenario)
  path_maps:
    - src: /mnt/disk1          # Server path prefix
      dst: E:\                 # Local drive letter or mount point
    - src: /mnt/disk2/media
      dst: F:\media

  # Force direct-disk playback when these prefixes match
  force_disk_prefixes: []

  # Direct-URL hosts for strm / cloud-drive sources
  direct_url_hosts: []

  path_check: false            # Verify file exists after path translation (NFC/NFD aware)
  one_instance: false          # Limit player to a single instance
```

**Routing priority:** locally mounted disk (with `path_check` verification) → Openlist direct link → HTTP stream from server.

### `players` — Player Selection & Preferences

```yaml
players:
  default: mpv                  # Default player key
  fullscreen: false             # Launch players in fullscreen mode

  # Route specific media to a different player based on path keywords
  by_path:
    - player: vlc
      match: [".iso", "__bdmv"]

  # Executable paths — only fill in players you actually have
  exe:
    mpv: C:\Green\mpv\mpv.exe
    vlc: C:\Green\vlc\vlc.exe
    mpc-hc: C:\Program Files\MPC-HC\mpc-hc64.exe
    potplayer: C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
    # generic: C:\Path\To\AnyPlayer.exe   # Optional: any launch-only player
```

| Player | Remote Control | Notes |
|:---|:---|:---|
| **mpv** | Full (pause, seek, volume, mute, subtitle/audio track, message, auto-next) | Recommended |
| VLC | Mostly (HTTP interface: pause, seek, volume, tracks, stop) | DisplayMessage silently ignored |
| MPC-HC | Mostly (Web interface: pause, seek, volume, mute, stop, prev/next) | Must enable Web UI in MPC options |
| PotPlayer | Start/stop only | No runtime control or progress sync |
| Generic | Start/stop only | Any executable |

### `openlist` — Cloud Drive Direct Link (AList Compatible)

Allows players to stream directly from cloud drives, bypassing server relay.

```yaml
openlist:
  host: ""                     # e.g. http://192.168.31.10:5255 — leave empty to disable
  token: ""                    # AList API key

  # Map server-side path prefix → openlist path prefix
  # dst can be empty for root mapping
  path_maps:
    - src: /volume1/video
      dst: ""
```

### `subtitle` — Subtitle Selection

```yaml
subtitle:
  # Priority order — first keyword match wins
  priority: ["中英", "双语", "简", "chi", "ass", "srt"]
```

### `version` — Multi-Version Selection

```yaml
version:
  # Priority order for multi-version media (e.g. different resolutions)
  prefer: ["remux", "2160", "1080"]
```

### `log` — Logging

```yaml
log:
  level: info                  # debug | info | warn | error
  file: ./meowflix.log
  mask_sensitive: true         # Mask tokens/domains in log output
```

---

## Windows: Double-Click Setup (No CLI)

If you downloaded the release `.zip`, extract it and double-click `1-首次设置.bat`.
It creates `meowflix.yaml` from the template and opens it in Notepad.

See the full [Getting Started Guide (Windows)](Getting-Started-Windows.md) for step-by-step instructions.

---

## Running

```bash
# Foreground (Ctrl+C to stop)
meowflix -config meowflix.yaml

# Different config path
meowflix -config /path/to/custom.yaml

# Show version
meowflix -version
```

For auto-start, see [Running Modes](../README.md#running-modes) in the main README.
