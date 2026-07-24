// Command ffi exposes the MeowFlixEmby daemon as a C-shared library so it can
// be embedded by native hosts (Electron/Tauri sidecars, Qt/WinUI shells, other
// languages via their C FFI). Build it with:
//
//	go build -buildmode=c-shared -o meowflix.dll ./api/ffi     # Windows
//	go build -buildmode=c-shared -o libmeowflix.so ./api/ffi   # Linux
//	go build -buildmode=c-shared -o libmeowflix.dylib ./api/ffi # macOS
//
// This produces the shared library plus a generated meowflix.h header.
//
// Lifecycle model: a single daemon instance per process. Call MeowflixStart
// with a config-file path; it runs on a background goroutine. Register an event
// callback (MeowflixSetEventCallback) BEFORE starting to receive lifecycle
// events as JSON (see EVENTS.md for the schema). Call MeowflixStop to shut down.
package main

/*
#include <stdlib.h>

// Event callback: receives a NUL-terminated JSON string owned by the library.
// The pointer is only valid for the duration of the call; copy it if you need
// to keep it. Invoked from a Go goroutine, so the host must be thread-safe.
typedef void (*meowflix_event_cb)(const char* json_event);

// Bridge so cgo can invoke a C function pointer stored on the Go side.
static inline void meowflix_invoke_cb(meowflix_event_cb cb, const char* s) {
    if (cb != NULL) {
        cb(s);
    }
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/EasyTidy/MeowFlixEmby/internal/app"
	"github.com/EasyTidy/MeowFlixEmby/internal/config"
)

// version is overridable via -ldflags "-X main.version=...".
var version = "dev"

var (
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
	running  bool
	lastErr  string
	eventCB  C.meowflix_event_cb
	cbActive bool
)

func main() {} // required for c-shared, never called

// event is the JSON envelope delivered to the host callback. See EVENTS.md.
type event struct {
	Type    string `json:"type"`              // "starting" | "running" | "stopped" | "error"
	Time    string `json:"time"`              // RFC3339
	Message string `json:"message,omitempty"` // human-readable detail
}

// emit sends an event to the registered callback, if any.
func emit(t, msg string) {
	mu.Lock()
	cb := eventCB
	active := cbActive
	mu.Unlock()
	if !active {
		return
	}
	payload, err := json.Marshal(event{
		Type:    t,
		Time:    time.Now().UTC().Format(time.RFC3339),
		Message: msg,
	})
	if err != nil {
		return
	}
	cs := C.CString(string(payload))
	C.meowflix_invoke_cb(cb, cs)
	C.free(unsafe.Pointer(cs))
}

//export MeowflixVersion
//
// MeowflixVersion returns the library version as a NUL-terminated C string.
// The caller must free the returned pointer with MeowflixFreeString.
func MeowflixVersion() *C.char {
	return C.CString(version)
}

//export MeowflixFreeString
//
// MeowflixFreeString frees a string returned by this library.
func MeowflixFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

//export MeowflixSetEventCallback
//
// MeowflixSetEventCallback registers a callback for lifecycle events. Pass NULL
// to clear. Register before MeowflixStart to catch the "starting" event.
func MeowflixSetEventCallback(cb C.meowflix_event_cb) {
	mu.Lock()
	eventCB = cb
	cbActive = cb != nil
	mu.Unlock()
}

//export MeowflixStart
//
// MeowflixStart loads the config at cfgPath and starts the daemon on a
// background goroutine. Returns 0 on success, non-zero if already running or on
// a config error (see MeowflixLastError). Non-blocking.
func MeowflixStart(cfgPath *C.char) C.int {
	mu.Lock()
	defer mu.Unlock()
	if running {
		lastErr = "already running"
		return 1
	}

	path := C.GoString(cfgPath)
	abs := path
	if p, err := filepath.Abs(path); err == nil {
		abs = p
	}
	cfg, err := config.Load(abs)
	if err != nil {
		lastErr = fmt.Sprintf("load config: %v", err)
		return 2
	}

	log := newLogger(cfg.Log)
	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	done = make(chan struct{})
	running = true
	lastErr = ""

	emit("starting", "loading configuration")
	go func() {
		emit("running", "daemon started")
		runErr := app.New(cfg, log, filepath.Dir(abs)).Run(ctx)
		mu.Lock()
		running = false
		if runErr != nil {
			lastErr = runErr.Error()
		}
		mu.Unlock()
		if runErr != nil {
			emit("error", runErr.Error())
		} else {
			emit("stopped", "daemon exited")
		}
		close(done)
	}()
	return 0
}

//export MeowflixStop
//
// MeowflixStop signals the daemon to shut down and blocks up to timeoutMs for a
// clean exit (<=0 means wait indefinitely). Returns 0 on clean stop, 1 if not
// running, 2 on timeout.
func MeowflixStop(timeoutMs C.int) C.int {
	mu.Lock()
	if !running || cancel == nil {
		mu.Unlock()
		return 1
	}
	cancelFn := cancel
	d := done
	mu.Unlock()

	cancelFn()
	if timeoutMs <= 0 {
		<-d
		return 0
	}
	select {
	case <-d:
		return 0
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		return 2
	}
}

//export MeowflixIsRunning
//
// MeowflixIsRunning returns 1 if the daemon is currently running, else 0.
func MeowflixIsRunning() C.int {
	mu.Lock()
	defer mu.Unlock()
	if running {
		return 1
	}
	return 0
}

//export MeowflixLastError
//
// MeowflixLastError returns the last error message (empty string if none) as a
// NUL-terminated C string. The caller must free it with MeowflixFreeString.
func MeowflixLastError() *C.char {
	mu.Lock()
	defer mu.Unlock()
	return C.CString(lastErr)
}

// newLogger builds a logger for the embedded daemon, honouring the config's
// level and optional file (falling back to stderr).
func newLogger(c config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(c.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	w := os.Stderr
	if c.File != "" {
		if f, err := os.OpenFile(c.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			w = f
		}
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
