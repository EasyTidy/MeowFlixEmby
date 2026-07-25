# MeowFlixEmby — Go Project Layering & Conventions

> Part 4. Previous: [03-Architecture Design](03-architecture-design.md); Next: [05-Implementation Plan](05-implementation-plan.md)

## 1. Standards Followed

| Standard | Purpose |
|------|------|
| [Standard Go Project Layout](https://github.com/golang-standards/project-layout) | Directory layering: `cmd/ internal/ pkg/` |
| [Effective Go](https://go.dev/doc/effective_go) | Language idioms |
| [Google Go Style Guide](https://google.github.io/styleguide/go/) | Naming, formatting, interfaces |
| [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) | Review conventions |
| SemVer 2.0 / Conventional Commits | Versioning & commits |

Core principle: **interface-oriented programming + dependency injection**, enabling the three major domains — media server, player, playback decision — to be replaced by third-party implementations. This is the engineering foundation of "pluggability."

## 2. Directory Structure

```
MeowFlixEmby/
├── cmd/
│   └── meowflix/                # Standalone entry point (main package, keep thin)
│       └── main.go
├── internal/                    # Project-private, no external imports
│   ├── app/                     # App assembly (DI, lifecycle)
│   │   └── app.go
│   ├── config/                  # Config loading/validation (YAML + env vars)
│   │   ├── config.go
│   │   └── config_test.go
│   └── platform/                # OS-specific (auto-start, process activation, pipe paths)
│       ├── platform_windows.go
│       ├── platform_darwin.go
│       └── platform_linux.go
├── pkg/                         # Stable public API, reusable by other projects/languages
│   ├── mediaserver/             # Media server abstraction (pluggable)
│   │   ├── mediaserver.go       # Server interface definition
│   │   ├── emby/                # Emby implementation
│   │   ├── jellyfin/            # Jellyfin implementation (reuses most of emby)
│   │   └── plex/                # Plex implementation (optional)
│   ├── remote/                  # WebSocket remote/cast session
│   │   ├── session.go           # Capability registration, command reception, heartbeat
│   │   └── commands.go          # Play/Playstate command models
│   ├── resolver/                # ★Playback resolution engine (core value)
│   │   ├── resolver.go          # PlaybackMethod decisions
│   │   ├── pathmap.go           # src→dst path translation
│   │   ├── version.go           # Multi-version preference
│   │   └── subtitle.go          # Subtitle preference
│   ├── player/                  # Player abstraction (pluggable)
│   │   ├── player.go            # Player interface (Start/Progress/Control/Stop)
│   │   ├── mpv/                 # mpv JSON IPC (named pipe / unix socket)
│   │   ├── potplayer/
│   │   ├── vlc/                 # VLC HTTP interface
│   │   └── generic/             # Unadapted player (launch only, no reporting)
│   ├── progress/                # Progress reporting (Sessions/Playing*)
│   │   └── reporter.go
│   └── playback/                # Session orchestration: auto-next, progress loop, prefetch
│       └── controller.go
├── api/                         # Public contract: FFI headers, event JSON schema
│   └── ffi/                     # c-shared export (optional build)
│       └── export.go
├── configs/                     # Config templates
│   └── meowflix.example.yaml
├── scripts/                     # Build / auto-start scripts
├── docs/                        # Design documents
├── go.mod
├── go.sum
├── .golangci.yml
└── README.md
```

### Dependency Direction (strictly one-way)

```
cmd/ ─► internal/app ─► pkg/*     (app assembles the pkg components)
pkg/playback ─► pkg/{mediaserver, resolver, player, progress, remote}
pkg/* packages only depend on each other's interfaces, never on concrete implementations
internal/ is never imported by pkg/ (pkg must be independently reusable)
```

## 3. Core Interfaces (Pluggability Contract Draft)

```go
// pkg/mediaserver/mediaserver.go
type Server interface {
    Authenticate(ctx context.Context) error
    // Open a remote session: register capabilities + return command stream
    OpenSession(ctx context.Context, caps Capabilities) (<-chan RemoteCommand, error)
    // Retrieve media playback info (server-side file path, direct stream, strm source,
    // subtitles, multi-version)
    ResolveItem(ctx context.Context, itemID string) (*MediaItem, error)
    // Progress reporting
    ReportStart(ctx context.Context, p PlaybackState) error
    ReportProgress(ctx context.Context, p PlaybackState) error
    ReportStopped(ctx context.Context, p PlaybackState) error
}

// pkg/resolver/resolver.go
type Method int
const (
    MethodDirectDisk Method = iota // Local disk path (NAS mounted)
    MethodDirectURL                // Direct cloud-drive URL (strm / HTTP source)
    MethodHTTPStream               // Emby HTTP Direct Stream
)
type Resolver interface {
    // Select best playback method and final media path/URL from MediaItem + local config
    Resolve(item *mediaserver.MediaItem, cfg Config) (Decision, error)
}

// pkg/player/player.go
type Player interface {
    Name() string
    Start(ctx context.Context, req PlayRequest) (Handle, error)
}
type Handle interface {
    // Position, duration (for progress reporting & auto-next logic)
    Progress() (posSec, durSec float64, ok bool)
    // Remote control: pause/resume/seek/stop/change track
    Control(cmd ControlCmd) error
    Wait() (stopSec float64, err error) // Block until exit
}
```

## 4. Engineering Rules (Details)

- **Error handling**: Return `error` and wrap with `fmt.Errorf("...: %w", err)`; no `panic` (except for unrecoverable assembly errors in `main`).
- **Context**: All network/subprocess/IPC calls take `ctx context.Context` as first parameter; the daemon uses a cancellable root ctx and shuts down gracefully on signal.
- **Concurrency**: Every goroutine must have a clear exit path (`ctx.Done()` or channel close); shared state uses channels or `sync` primitives; `go test -race` must pass.
- **Logging**: `log/slog`, structured fields; supports levels; sensitive data (tokens/domains) redacted by default (mirroring reference project's `mix_log`).
- **Configuration**: YAML-based (more expressive than the reference project's ini), with environment variable overrides; validated on startup with readable error messages.
- **Testing**: The resolver engine, path translation, version preference, and subtitle preference must have unit tests (table-driven); media server and player use interfaces + mocks.
- **Lint**: `golangci-lint` (govet, staticcheck, errcheck, revive, etc.).
- **Naming**: Package names are short, all-lowercase, no underscores; exported identifiers start with uppercase and include doc comments; interfaces use `-er` suffix or domain names.
- **Versioning/commits**: SemVer for `pkg/` public API; Conventional Commits.

## 5. Configuration Model (YAML Draft)

```yaml
server:
  type: emby                     # emby | jellyfin | plex
  address: https://emby.example.com
  username: alice
  password: ""                   # or use api_key instead
  api_key: ""
  device_name: "MeowFlix (Living Room PC)"

playback:
  # Server-side path prefix → local disk prefix (NAS mount)
  path_maps:
    - src: /mnt/disk1
      dst: E:\
    - src: /mnt/disk2/media
      dst: F:\media
  # Force disk mode when path matches these prefixes
  force_disk_prefixes: []
  # Direct cloud-drive URL when strm/http source matches these hosts/keywords
  direct_url_hosts: []
  path_check: false              # Verify file existence + NFC/NFD normalization after conversion

players:
  default: mpv
  by_path:                       # Select player by path keywords
    - player: vlc
      match: [".iso", "__bdmv"]
  exe:
    mpv: C:\Green\mpv\mpv.exe
    potplayer: C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
    vlc: C:\Green\vlc\vlc.exe

subtitle:
  priority: ["中英", "简", "chi", "ass", "srt"]
version:
  prefer: ["remux", "2160", "1080"]

log:
  level: info
  file: ./meowflix.log
  mask_sensitive: true
```
