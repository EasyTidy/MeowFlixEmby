# MeowFlixEmby — Implementation Plan

> Part 5. Previous: [04-Go Project Layering & Conventions](04-go-project-structure.md)

## 1. Milestones

| Phase | Goal | Deliverables | Verification |
|------|------|------|------|
| **M0 Scaffold** | Go module, directory layering, config model, logging, lint/CI | Compilable stub `cmd/meowflix`, `configs/*.example.yaml`, interface definitions | `go build ./...`, `golangci-lint run` |
| **M1 Resolution Engine** | resolver + pathmap + version + subtitle (pure logic, no I/O) | `pkg/resolver/*` + table-driven unit tests | `go test -race ./pkg/resolver/...` all green |
| **M2 Server Adapter** | Emby auth / capability declaration / PlaybackInfo / progress reporting (REST) | `pkg/mediaserver/emby`, `pkg/progress` | Against real/test Emby instance: auth succeeds, can fetch MediaSources, can report progress |
| **M3 Remote Session** | WebSocket connection / keep-alive / receive Play·Playstate·GeneralCommand / reconnection | `pkg/remote` | Machine appears in web "Play On"; cast triggers Play command delivery |
| **M4 Player Drivers** | mpv (IPC) first, then vlc, potplayer, generic | `pkg/player/*` | Cast → local mpv launches, remote pause/seek/stop work |
| **M5 Orchestration Loop** | playback controller full chain + progress reporting + auto-next | `pkg/playback`, `internal/app` | End-to-end: web cast → local playback → progress back to web; auto-next plays |
| **M6 Cross-Platform & Integration** | Win/Mac/Linux build, auto-start scripts, `c-shared` FFI export, README | `scripts/`, `api/ffi`, release artifacts | Three-platform single-file run; FFI demo call succeeds |

## 2. M0 Implementation Checklist

1. `go.mod` (module name, Go version 1.22+).
2. Directory skeleton and stubs (see [04](04-go-project-structure.md) §2).
3. `internal/config`: YAML loading + validation + environment variable overrides.
4. `pkg/*` core interface definitions (mediaserver / resolver / player / progress / remote).
5. `cmd/meowflix/main.go`: assembly skeleton, `context` lifecycle, graceful shutdown on signal, `log/slog` init.
6. `configs/meowflix.example.yaml`, `.golangci.yml`, `README.md`.

## 3. Dependency Selection

| Purpose | Choice | Rationale |
|------|------|------|
| WebSocket | `github.com/coder/websocket` (formerly nhooyr) or `gorilla/websocket` | Actively maintained, context-friendly |
| YAML | `gopkg.in/yaml.v3` | De facto standard |
| Logging | stdlib `log/slog` | Zero dependencies, structured |
| HTTP | stdlib `net/http` | Sufficient |
| mpv IPC | Custom JSON-over-pipe/socket (thin wrapper) | No mature general-purpose library; fine-grained control needed |
| Test assertions | stdlib `testing` (table-driven) + optional `testify` | Prefer standard library by convention |

> Dependency policy: prefer standard library; third-party must be actively maintained and license-compatible; lock versions in `go.mod`.

## 4. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|------|------|
| Protocol field drift across server versions | Command reception / progress reporting fails | Adapter layer isolation; `SupportedCommands` runtime enumeration; capture-based verification of **[TBD]** items |
| Session idle timeout removes from cast list | User cannot find target | Heartbeat keep-alive + reconnection + re-POST Capabilities |
| Jellyfin WebSocket path `/socket` not finalized | Jellyfin won't connect | M3 verification against jellyfin-apiclient-python |
| Weak progress reporting for non-mpv players | Inaccurate progress | mpv as first-class citizen; degraded strategy for weak players with explicit user notification |
| Mounted drive path case/encoding differences (cross-OS) | File not found | `path_check` + NFC/NFD normalization; fallback to HTTP stream |
| Cast UX differs from "click native play button" | User habit friction | Document the flow; Emby remembers last cast target |

## 5. Test Strategy

- **Unit**: resolver all paths, pathmap, version, subtitle table-driven; `-race`.
- **Contract**: mediaserver / player via interfaces + mocks, verifying orchestration logic without real backends.
- **Integration**: against a real or Dockerized Emby instance, run M2/M3/M5 end-to-end scripts.
- **Manual acceptance**: one run per platform of "web cast → local playback → progress report → auto-next → remote control."

## 6. Delivery & Distribution

- `scripts/build.sh`: `GOOS/GOARCH` cross-compilation producing win/mac/linux single files.
- Auto-start: Windows Task Scheduler / Startup folder, macOS launchd plist, Linux systemd user unit (templates in `scripts/`).
- FFI: `scripts/build-shared.sh` produces `.dll`/`.so`/`.dylib` + header files.
- Versioning: SemVer; `CHANGELOG.md`; Conventional Commits.
