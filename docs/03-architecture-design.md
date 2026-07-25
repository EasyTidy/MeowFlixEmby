# MeowFlixEmby — Architecture Design

> Part 3. Previous: [02-Architecture Decision & Language Rationale](02-architecture-and-language-choice.md); Next: [04-Go Project Layering & Conventions](04-go-project-structure.md)
>
> Fields marked **[TBD: verify against server]** must be confirmed via packet capture or against the target server at runtime; do not hardcode.

## 1. High-Level Architecture

MeowFlixEmby is a Go resident daemon running on the **client machine where the player is installed**. It registers itself as a "remote-controllable / castable session" with Emby via WebSocket, receives playback and remote-control commands from the server, selects the best playback path per resource type, launches the local media player, and reports progress back to the server.

```
┌────────────┐         User clicks "Play On/Cast" in browser
│  Browser    │ ──────────────────────────────────────┐
│ (Emby web) │                                        │
└────────────┘                                        ▼
                                            ┌────────────────────┐
┌────────────────────┐   ① Auth REST       │    Emby / Jellyfin   │
│                     │◄───────────────────│      Server          │
│   MeowFlixEmby      │   ② WebSocket      │  (can be remote,     │
│   (Go daemon)       │◄══════════════════►│   NAS, cloud, etc.)  │
│                     │   ③ Recv Play/     └────────────────────┘
│  ┌───────────────┐  │      Playstate
│  │ remote session│  │   ④ REST: PlaybackInfo / progress report
│  ├───────────────┤  │
│  │ resolver      │──┼──► Direct URL / Local disk / HTTP stream
│  ├───────────────┤  │
│  │ player driver │──┼──► Local player (mpv/pot/vlc/mpc...)
│  ├───────────────┤  │         ▲ IPC: read progress, pause/seek/next
│  │ progress rpt  │  │         │
│  └───────────────┘  │◄────────┘
└────────────────────┘
```

## 2. Runtime Flow

### 2.1 Startup & Registration (become a cast target)

1. **Authenticate**: `POST {base}/Users/AuthenticateByName`, body `{"Username":..,"Pw":..}`, header
   `Authorization: Emby UserId="", Client="MeowFlix", Device="<hostname>", DeviceId="<persistent GUID>", Version="<ver>", Token=""`.
   Extract `AccessToken` and `User.Id` from the response. **DeviceId must be persisted** and must be identical across authentication, WebSocket, and capability declaration.
   > API Key alone does not work: admin API keys are not bound to a user and will not appear as castable targets. Real user authentication is required.
2. **Declare capabilities**: `POST {base}/Sessions/Capabilities/Full` (with `X-Emby-Token`), body:
   ```json
   {
     "PlayableMediaTypes": ["Video","Audio"],
     "SupportedCommands": ["PlayPause","Pause","Unpause","Stop","Seek","NextTrack","PreviousTrack","SetSubtitleStreamIndex","SetAudioStreamIndex","DisplayMessage","SetVolume","Mute","Unmute"],
     "SupportsMediaControl": true,
     "SupportsPersistentIdentifier": true
   }
   ```
   `SupportsMediaControl:true` is the key flag that makes this machine appear in the "Play On" menu. `SupportedCommands` only declares the subset actually implemented; the full available set varies by server version and should be enumerated at runtime, not hardcoded **[TBD: verify against server]**.
3. **Establish WebSocket**: Derive from the HTTP base URL by replacing the scheme (`https→wss`, preserving any path prefix):
   `ws(s)://host[/emby]/embywebsocket?api_key=<AccessToken>&deviceId=<DeviceId>`.
   > Jellyfin uses the path `/socket` (**[TBD: verify]**; reference: jellyfin-apiclient-python), and the auth header scheme keyword is `MediaBrowser`. The adapter layer switches based on `server.type`.
4. **Keep-alive**: The server periodically sends `KeepAlive`/`ForceKeepAlive`; reply with `{"MessageType":"KeepAlive"}`. **An idle session (≈5 minutes of inactivity or disconnection) disappears from the cast list**, so heartbeats must be maintained, and **capabilities must be re-POSTed** after reconnection.

### 2.2 Receiving a Cast → Playback

WebSocket messages use a uniform envelope: `{"MessageType":<name>,"Data":<obj|string>,"MessageId":<opt>}`.

On receiving `Play`:
```json
{ "MessageType":"Play",
  "Data":{ "ItemIds":["<id>"], "PlayCommand":"PlayNow",
           "StartPositionTicks":0, "StartIndex":0, "ControllingUserId":"<uid>" } }
```
- `PlayCommand` ∈ `PlayNow|PlayNext|PlayLast`. `ItemIds` is the queue; play the first, queue the rest for auto-next.
- `StartPositionTicks` is the resume position (100ns ticks) **[TBD: verify if 0 is omitted]**.

Processing flow:
1. For the first ItemId, call `POST {base}/Items/{id}/PlaybackInfo` (body may include `UserId`, `StartTimeTicks`, `DeviceProfile`, `MaxStreamingBitrate`), obtaining `MediaSources[]` and `PlaySessionId` (**reuse the same PlaySessionId throughout** to tie stream URLs and progress reports together).
2. Feed into the **resolver engine** (see §3) to select the playback method and final media path/URL.
3. When metadata is needed (title/episode/resume position), call `GET {base}/Users/{uid}/Items/{itemId}`.
4. The **player driver** launches the local player (see §4), passing the media path, start time, subtitles, and title.

### 2.3 Remote Control Commands

`Playstate`:
```json
{ "MessageType":"Playstate",
  "Data":{ "Command":"Seek", "SeekPositionTicks":6000000000, "ControllingUserId":"<uid>" } }
```
`Command` ∈ `Stop|Pause|Unpause|PlayPause|NextTrack|PreviousTrack|Seek|Rewind|FastForward|SeekRelative`. Mapped to player IPC control. `SeekPositionTicks` is the absolute target position (in 100ns ticks).

`GeneralCommand` (volume/subtitle/message): `Data.Name` is the `GeneralCommandType`, `Data.Arguments` is a string map (e.g. `DisplayMessage` → `Header/Text/TimeoutMs`; `SetVolume` → `Volume`).

### 2.4 Progress Reporting

Three REST endpoints (with `X-Emby-Token`), body is `PlaybackStart/Progress/StopInfo`:
- Start: `POST {base}/Sessions/Playing`
- Progress: `POST {base}/Sessions/Playing/Progress` (periodic ≈1–10s + on state change)
- Stop: `POST {base}/Sessions/Playing/Stopped` (send final position)

```json
{ "ItemId":"<id>", "PlaySessionId":"<psid>", "MediaSourceId":"<msid>",
  "PositionTicks":10460000000, "IsPaused":false, "CanSeek":true,
  "PlayMethod":"DirectStream", "AudioStreamIndex":1, "SubtitleStreamIndex":-1,
  "RepeatMode":"RepeatNone" }
```
- **PositionTicks unit = 100ns, i.e. seconds × 10,000,000**. `StartPositionTicks`/`SeekPositionTicks`/`RunTimeTicks` use the same unit.
- `PlayMethod` ∈ `DirectPlay|DirectStream|Transcode`: use `DirectPlay` for direct disk access, `DirectStream` for HTTP streaming.

## 3. Resolution Engine (resolver — Core Value)

Replicates and simplifies the logic of the reference project's `data_parser.py`. Input: `MediaSource` (`Path`/`Protocol`/`Container`/`DirectStreamUrl`/`TranscodingUrl`/`IsRemote`) + local config. Output: `Decision{Method, MediaPathOrURL, PlayMethod}`.

Evaluation priority (top-down, first match wins):

| # | Condition | Method | media | Report as `PlayMethod` |
|---|------|------|------|------|
| 1 | `MediaSource.Path` prefix matches `force_disk_prefixes` | `DirectDisk` | `pathmap(Path)` | DirectPlay |
| 2 | strm / `Protocol=Http` source with host in `direct_url_hosts` (cloud-drive direct) | `DirectURL` | source HTTP URL | DirectStream |
| 3 | `path_maps` configured and `pathmap(Path)` matches a prefix (NAS mounted locally) | `DirectDisk` | `pathmap(Path)` | DirectPlay |
| 4 | Everything else | `HTTPStream` | `base + DirectStreamUrl` (or `/Videos/{id}/stream?Static=true&MediaSourceId=..&PlaySessionId=..&api_key=..`); use `TranscodingUrl`(HLS) if server requires transcoding | DirectStream/Transcode |

Supporting sub-modules:
- **pathmap** (`resolver/pathmap.go`): `src`→`dst` prefix replacement + path separator normalization + optional `path_check` (file existence + NFC/NFD normalization). Corresponds to `translate_path_by_ini`.
- **version** (`resolver/version.go`): prefer the best `MediaSource` by `version.prefer` keywords when multiple sources exist. Corresponds to `version_prefer_emby`.
- **subtitle** (`resolver/subtitle.go`): select external/embedded subtitles by `subtitle.priority` keywords; generate external subtitle URL as `/Videos/{id}/{msid}/Subtitles/{idx}/Stream.{codec}?api_key=..` for HTTP streaming. Corresponds to `subtitle_checker`.

> The resolution engine is a pure function with no I/O (except `path_check`). It is the primary unit-test target (table-driven coverage of all four paths + strm multi-version edge cases).

## 4. Player Drivers

Each player implements the `Player`/`Handle` interfaces (see [04](04-go-project-structure.md) §3):

| Player | Progress/Control Channel | Launch Args | Remote Control |
|------|------|------|------|
| **mpv / mpv.net / IINA** | JSON IPC: Windows named pipe `\\.\pipe\<name>`, Unix `/tmp/<name>.sock` (`--input-ipc-server`) | `--start=`, `--sub-file=`, `--force-media-title=` | ✅ Full (pause/seek/next/playlist) |
| **VLC** | HTTP interface `:port/requests/status.xml` (`--http-port`, `--http-password`) | `:start-time=`, `--sub-file` | ✅ Most |
| **PotPlayer** | CLI + window messages | `/seek=`, `/sub=`, `/title=` | ⚠️ Limited (weak progress reporting) |
| **MPC-HC/BE** | Web interface `/variables.html`, `/command.html` | `/start ms`, `/sub` | ⚠️ Partial |
| **generic** | `os/exec` launch only | best-effort args | ❌ No reporting |

- Progress polling: `Handle.Progress()` periodically reads `(posSec, durSec)`, fed to progress reporting and auto-next logic.
- Auto-next: mpv uses native playlist; unsupported players fall back to "close at >0.9 progress → launch next episode" (mirroring `http_sub_auto_next_ep` in the reference project).
- Single instance / multi-launch: controlled by `one_instance` config.
- Player selection: `players.default` + `players.by_path` (switch by path keyword / bdmv / iso), corresponding to `select_player_by_path`.

## 5. Pluggable Integration — Three Forms

1. **Go library**: `import github.com/<org>/MeowFlixEmby/pkg/...`, assemble with `playback.NewController(server, resolver, playerRegistry, reporter)`.
2. **In-process interface injection**: third parties implement `mediaserver.Server` / `player.Player` / `resolver.Resolver` to inject custom backends or players.
3. **Cross-language FFI**: `go build -buildmode=c-shared -o meowflix.dll ./api/ffi`, exports `Start/Stop/OnEvent` for C/Python/.NET; events use JSON schema (under `api/`).
4. **Standalone**: `cmd/meowflix` single-file binary + config + auto-start scripts.

## 6. Security Considerations

- **Credentials**: `AccessToken` and passwords must not appear in logs (`log.mask_sensitive`, corresponding to `mix_log`); config file should have 0600 permissions; `api_key` direct setting avoids storing passwords.
- **No local listening port**: Under Approach A, the local machine acts as a WebSocket **client** with no listening port by default — a smaller attack surface than the reference project's local HTTP service. If a local control interface is needed later, listen on `127.0.0.1` only and require a token.
- **TLS**: validate server certificates by default; provide an explicit `skip_verify` flag (disabled by default) for self-signed scenarios (corresponding to `skip_certificate_verify`).
- **Player subprocess**: command-line arguments are escaped to prevent injection.

## 7. Jellyfin / Plex Differences (Adapter Layer)

| Item | Emby | Jellyfin | Plex |
|---|------|------|------|
| Auth header scheme | `Emby` | `MediaBrowser` (compatible with `Emby`) | `X-Plex-Token` |
| WebSocket path | `/embywebsocket` | `/socket` **[TBD]** | None (Companion/GDM) |
| Capabilities/commands/progress | See above | Largely identical (same wire names) | Entirely different: `/player/playback/*` + `/:/timeline` |

Plex's mechanism differs significantly from Emby's (HTTP commands + header-based targeting, not WS push). It is listed as **optional future adaptation**, with interface slots reserved for a `plex` implementation.
