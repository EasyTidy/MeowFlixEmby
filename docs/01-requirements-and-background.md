# MeowFlixEmby — Requirements & Background

> This is Part 1 of the design documents. Related:
> - [02-Architecture Decision & Language Rationale](02-architecture-and-language-choice.md)
> - [03-Architecture Design](03-architecture-design.md)
> - [04-Go Project Layering & Conventions](04-go-project-structure.md)
> - [05-Implementation Plan](05-implementation-plan.md)

## 1. Project Goal

When a user triggers playback from the **browser** on a media server powered by Emby (compatible with Jellyfin; Plex support desired eventually), stream or directly connect the video to a **local media player** (mpv / PotPlayer / VLC / MPC, etc.) and **report playback progress** back to the server.

Key constraints:

1. **Emby and the player need not be on the same machine**: the local program runs on the client machine with the player; Emby may be on a NAS, VPS, or another PC.
2. **Best playback path per resource type**:
   - **Cloud-drive resources** (strm / direct-access HTTP URLs) → player plays the **direct cloud-drive URL**, bypassing the Emby relay.
   - **NAS disk resources with local mount** (SMB/NFS mapped drive, rclone mount) → play via the **local disk path** with zero network relay.
   - **Everything else** → stream via Emby's **HTTP Direct Stream** URL.
3. **Avoid browser userscripts like Tampermonkey** unless absolutely necessary.
4. **Pluggable**: usable as a library/plugin embedded in other projects, or run standalone.
5. Use Rust, Go, or .NET, following the language's best practices for project structure.

## 2. Reference Project: `embyToLocalPlayer` (Python + Tampermonkey)

The existing project [`embyToLocalPlayer`](https://github.com/kjtsune/embyToLocalPlayer) solves the same problem. Source code study reveals the following mechanism:

### 2.1 Trigger Mechanism (the pain point we avoid)

- The browser side relies on a **Tampermonkey script** `embyToLocalPlayer.user.js`, which hooks `window.fetch` and `XMLHttpRequest` at `document-start`.
- Intercepts Emby/Jellyfin `/Items/{id}/PlaybackInfo` and Plex `playQueues` requests.
- When the user clicks the native "Play" button, the script POSTs playback data (ApiClient info, playbackData, playbackUrl, extraData) to `http://127.0.0.1:58000`.

> This is exactly what we **avoid** — depending on browser extensions + userscripts, which are cumbersome to install and break easily when Emby/Jellyfin frontend is updated (the script contains extensive CSS selectors and DOM compatibility code, see `removeErrorWindows`, `addFileNameElement`).

### 2.2 Local Service & Playback Routing (the core value we replicate)

The Python side (`utils/data_parser.py` + `utils/tools.py`) determines the playback method. Core logic (`parse_received_data_emby`):

| Condition | Result `media_path` |
|------|------|
| `force_disk_mode_path` prefix matches server file path | Force **disk mode** |
| strm with HTTP source and `strm_direct_host` match | **Direct cloud-drive URL** (`source_path`) |
| User enabled mount-disk mode, non-strm | `translate_path_by_ini(file_path)` → **local disk path** |
| Otherwise | Emby **HTTP Direct Stream URL** |

- **Path translation** (`translate_path_by_ini`): replace server-side prefix (`[src]`, e.g. `/mnt/disk1`) with local prefix (`[dst]`, e.g. `E:`); optional `path_check` for file existence and NFC/NFD normalization.
- **Multi-version selection** (`version_prefer_emby`): pick stream by filename keyword priority.
- **Subtitle selection** (`subtitle_checker`): pick external/embedded subtitles by `subtitle_priority` keyword order.

### 2.3 Player Integration & Progress Reporting

- `utils/players.py`: implements launch + progress polling for mpv (named pipe / unix socket JSON IPC), VLC (HTTP interface), PotPlayer, MPC, IINA, etc.
- `utils/player_manager.py`: organizes playlist, real-time progress, pre-fetch next episode, next-episode redirect.
- `utils/net_tools.py`: reports progress to `Sessions/Playing`, `Sessions/Playing/Progress`, `Sessions/Playing/Stopped` (Emby/Jellyfin) and `/:/timeline` (Plex). Tick unit is 100ns (`sec * 10^7`).
- Optional trakt.tv / bgm.tv watch sync.

## 3. What We Keep vs. Replace

Keep the battle-tested domain logic: **playback routing decisions, path translation, multi-version/subtitle selection, player IPC progress reading, progress reporting**. Replace the "Tampermonkey script intercepting browser requests" trigger.

After confirming with stakeholders, the replacement trigger is the **Cast Target** approach (see [02](02-architecture-and-language-choice.md), [03](03-architecture-design.md)): the local Go program registers itself as a "remote-controllable / castable session" with Emby via WebSocket. When the user clicks "Play On / Cast" in the web UI and selects this machine, the server pushes the playback command to the local program. **Zero browser changes, zero server changes, cross-device by design.**

## 4. Implementation Language

**Go** was chosen. See [02-Architecture Decision & Language Rationale](02-architecture-and-language-choice.md) for the full rationale.
