# MeowFlixEmby — Architecture Decision & Language Rationale

> Part 2. Previous: [01-Requirements & Background](01-requirements-and-background.md); Next: [03-Architecture Design](03-architecture-design.md)

## 1. Trigger Mechanism Selection

Without Tampermonkey, there are three technical approaches to "browser click → local player playback":

| Approach | Mechanism | Pros | Cons | Verdict |
|------|------|------|------|------|
| **A. Cast Target** | Local program registers via WebSocket as an Emby remote-controllable session, appears in the "Play On" menu; server pushes playback commands to local program upon selection | Zero browser changes, zero server changes, cross-device by design, officially supported protocol, remote control (pause/seek/stop/next) | User must click "Cast" and pick target once (Emby remembers) | ✅ **Chosen** |
| B. Emby Server Plugin JS Injection | .NET server plugin injects hooking JS into the web UI | Retains "click native play button" behavior | Requires plugin on Emby server (.NET only); Emby's web injection support is fragile; breaks when frontend updates | ❌ |
| C. Local Reverse Proxy JS Injection | Local reverse proxy injects JS when accessing Emby | No Tampermonkey, no server plugin | User must use the proxy to access Emby; TLS/certificate complexity | ❌ |

**Stakeholders confirmed Approach A (Cast Target).**

### Approach A User Experience

- First use: open video in web UI → "Play On / Cast" → select `MeowFlix (hostname)`.
- Afterwards: Emby remembers the last cast target, behavior approaches "click play = cast to local".
- Bonus: web UI progress bar, pause, seek, prev/next episode all propagate to local player.

## 2. Language Selection: Rust / Go / .NET

**Go** was chosen. Full rationale follows.

### 2.1 Project Technical Profile

- Workload is primarily **I/O orchestration**: maintaining WebSocket long connections, concurrent HTTP requests, child process (player) management, player IPC (named pipe / unix socket) read/write.
- Almost no CPU- or memory-intensive computation (no transcoding, no decoding).
- Runs on **client machines** as a **background daemon**.
- Requires **cross-platform** support (Windows / macOS / Linux).
- Must be **usable as a library** embedded in other projects, as well as standalone.

### 2.2 Three-Language Comparison

| Dimension | Go | .NET | Rust |
|------|----|----|----|
| Concurrency fit (WS+HTTP+subprocess+IPC) | ✅ goroutine + channel, natural fit | ✅ async/await, good | ✅ tokio, good but high cognitive load |
| Standalone / distribution | ✅ Single static binary, no runtime | ⚠️ Needs runtime or large self-contained artifact | ✅ Single static binary |
| Cross-platform cross-compilation | ✅ `GOOS/GOARCH`, one command | ✅ Good | ✅ Good (needs target toolchain) |
| Library embeddability | ✅ Go module; also `c-shared` for C ABI dylib | ⚠️ NuGet only for .NET; extra work for cross-language | ✅ crate; `cdylib` for C ABI, best |
| Emby server plugin capability | ❌ | ✅ (unique, but unneeded with Approach A) | ❌ |
| Dev iteration speed | ✅ Fast, low barrier | ✅ Fast | ⚠️ Borrow checker slows I/O-oriented dev |
| Binary size | Medium (~5–15 MB) | Large (self-contained ~60 MB+) | Small |
| Ecosystem: Emby/media client references | Medium | Medium | Weak |

### 2.3 Conclusion

- This is an **I/O-orchestration client daemon**. Rust's performance/memory-safety advantage is unnecessary here and slows iteration; .NET's unique advantage (server plugin) is not needed with Approach A, and distribution artifacts are heavy.
- **Go** excels across "single static binary, concurrency orchestration, cross-platform, dev velocity". It satisfies "pluggability" via two paths: Go module (native) + `-buildmode=c-shared` (cross-language FFI).
- **Final choice: Go**.

### 2.4 How Pluggability Works in Go

1. **Go library integration**: core logic in `pkg/`, other Go projects `import` and go; only stable interfaces and constructors are exposed.
2. **In-process plugin**: domain capabilities (media server adapter, playback routing, player driver) all based on Go `interface`; third parties can implement interfaces to inject custom servers or players.
3. **Cross-language FFI**: when needed, use `go build -buildmode=c-shared` to produce `.dll`/`.so`/`.dylib` + header files for C/C++/Python/.NET FFI.
4. **Standalone**: `cmd/meowflix` produces a single-file executable with systemd/launchd/Windows auto-start scripts.

## 3. Conventions & Best Practices (checklist)

This project follows these established conventions; details in [04-Go Project Layering & Conventions](04-go-project-structure.md):

- **Standard Go Project Layout** (`cmd/`, `internal/`, `pkg/` layering convention).
- **Effective Go** and **Google Go Style Guide** (naming, error handling, interface design).
- **Go Code Review Comments** (community review conventions).
- **Dependency injection + interface-oriented programming** (pluggable, testable).
- **`context.Context`** on all blocking/network/subprocess calls, supporting cancellation and timeout.
- **`log/slog`** structured logging (Go 1.21+).
- **`golangci-lint`** static analysis; **`go test` + race detector** for testing.
- **Semantic Versioning (SemVer)** for public API and releases.
- **Conventional Commits** for commit messages.
