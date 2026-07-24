// Package mpc drives MPC-HC / MPC-BE (Windows) over its built-in web interface
// (/command.html + /variables.html). If the web interface can't be reached
// (disabled in options, or the port not answering) the driver degrades to
// launch-only: it can still Stop by terminating the process but reports no
// progress and rejects other controls.
//
// Command IDs and endpoints verified against clsid2/mpc-hc source
// (resource.h, WebClientSocket.cpp, mpc-hc.h).
package mpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// MPC-HC wm_command IDs (from resource.h). CMD_SETPOS/-VOLUME are magic ids
// from mpc-hc.h that read extra query params.
const (
	cmdPlay      = 887
	cmdPause     = 888
	cmdPlayPause = 889
	cmdStop      = 890
	cmdVolMute   = 909
	cmdSkipBack  = 921
	cmdSkipFwd   = 922
	cmdSetPos    = -1 // needs &position=H:MM:SS
	cmdSetVolume = -2 // needs &volume=0-100
)

// Player launches MPC-HC and controls it over the web interface.
type Player struct {
	exePath     string
	extraArgs   []string
	dialTimeout time.Duration
}

// Options configures the MPC Player.
type Options struct {
	ExePath     string // MPC-HC exe; defaults to "mpc-hc64.exe" (on PATH)
	ExtraArgs   []string
	DialTimeout time.Duration // wait for the web interface; default 8s
}

// New builds an MPC Player.
func New(opts Options) *Player {
	exe := opts.ExePath
	if exe == "" {
		exe = "mpc-hc64.exe"
	}
	dt := opts.DialTimeout
	if dt == 0 {
		dt = 8 * time.Second
	}
	return &Player{exePath: exe, extraArgs: opts.ExtraArgs, dialTimeout: dt}
}

// Name is the registry key.
func (p *Player) Name() string { return "mpc-hc" }

// Start launches MPC-HC with the web interface on a free port and connects.
func (p *Player) Start(ctx context.Context, req player.PlayRequest) (player.Handle, error) {
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("mpc: pick port: %w", err)
	}
	args := []string{req.MediaPath, "/play", "/new", "/webport", strconv.Itoa(port)}
	if req.StartSec > 0 {
		// MPC-HC /start takes milliseconds.
		args = append(args, "/start", strconv.FormatInt(int64(req.StartSec*1000), 10))
	}
	if req.SubFile != "" {
		args = append(args, "/sub", req.SubFile)
	}
	args = append(args, p.extraArgs...)

	cmd := exec.CommandContext(ctx, p.exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch mpc-hc: %w", err)
	}
	h := &handle{
		cmd:  cmd,
		base: fmt.Sprintf("http://127.0.0.1:%d", port),
		http: &http.Client{Timeout: 3 * time.Second},
		done: make(chan struct{}),
	}
	h.webReady = h.waitReady(ctx, p.dialTimeout) == nil
	go h.watch()
	return h, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// handle controls a running MPC-HC. Implements player.Handle.
type handle struct {
	cmd      *exec.Cmd
	base     string
	http     *http.Client
	webReady bool

	mu     sync.RWMutex
	posSec float64
	durSec float64

	doneOnce sync.Once
	done     chan struct{}
}

var _ player.Handle = (*handle)(nil)

func (h *handle) watch() {
	_ = h.cmd.Wait()
	h.doneOnce.Do(func() { close(h.done) })
}

// waitReady polls variables.html until MPC-HC's web interface answers.
func (h *handle) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, _, err := h.readVariables(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return fmt.Errorf("web interface not ready after %s", timeout)
}

var reVar = regexp.MustCompile(`<p id="(\w+)">([^<]*)</p>`)

// readVariables fetches /variables.html and returns position and duration (sec).
func (h *handle) readVariables(ctx context.Context) (posSec, durSec float64, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", h.base+"/variables.html", nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("variables status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	vars := map[string]string{}
	for _, m := range reVar.FindAllStringSubmatch(string(body), -1) {
		vars[m[1]] = m[2]
	}
	posMs, okP := vars["position"]
	durMs, okD := vars["duration"]
	if !okP || !okD {
		return 0, 0, fmt.Errorf("variables missing position/duration")
	}
	pos := parseMsToSec(posMs)
	dur := parseMsToSec(durMs)
	h.mu.Lock()
	h.posSec, h.durSec = pos, dur
	h.mu.Unlock()
	return pos, dur, nil
}

func parseMsToSec(s string) float64 {
	ms, _ := strconv.ParseFloat(s, 64)
	return ms / 1000
}

// Progress refreshes from the web interface (best-effort) and returns the cache.
// ok is false when the web interface was never reachable.
func (h *handle) Progress() (posSec, durSec float64, ok bool) {
	if !h.webReady {
		return 0, 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _ = h.readVariables(ctx)
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.posSec, h.durSec, true
}

// Wait blocks until MPC-HC exits and returns the last observed position.
func (h *handle) Wait() (stopSec float64, err error) {
	<-h.done
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.posSec, nil
}

// Control forwards a transport command via the MPC-HC web interface. When the
// web interface is unreachable only Stop (process kill) is honoured.
func (h *handle) Control(c player.Control) error {
	if !h.webReady {
		if c.Cmd == player.CtrlStop && h.cmd.Process != nil {
			return h.cmd.Process.Kill()
		}
		return fmt.Errorf("mpc-hc: web interface unavailable, control %d unsupported", c.Cmd)
	}
	q := url.Values{}
	switch c.Cmd {
	case player.CtrlPause:
		q.Set("wm_command", strconv.Itoa(cmdPause))
	case player.CtrlUnpause:
		q.Set("wm_command", strconv.Itoa(cmdPlay))
	case player.CtrlPlayPause:
		q.Set("wm_command", strconv.Itoa(cmdPlayPause))
	case player.CtrlStop:
		q.Set("wm_command", strconv.Itoa(cmdStop))
	case player.CtrlNextTrack:
		q.Set("wm_command", strconv.Itoa(cmdSkipFwd))
	case player.CtrlPreviousTrack:
		q.Set("wm_command", strconv.Itoa(cmdSkipBack))
	case player.CtrlSetVolume:
		q.Set("wm_command", strconv.Itoa(cmdSetVolume))
		q.Set("volume", strconv.Itoa(clampPct(c.Volume)))
	case player.CtrlMute:
		q.Set("wm_command", strconv.Itoa(cmdVolMute))
	case player.CtrlSeekAbsolute:
		q.Set("wm_command", strconv.Itoa(cmdSetPos))
		q.Set("position", secToHMS(c.SeekSec))
	case player.CtrlSeekRelative:
		return h.seekRelative(c.SeekSec)
	default:
		// Unmute/ToggleMute/track selection/DisplayMessage: no clean web command.
		return fmt.Errorf("mpc-hc: unsupported control %d", c.Cmd)
	}
	return h.sendCommand(q)
}

// seekRelative reads the current position and issues an absolute seek.
func (h *handle) seekRelative(deltaSec float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pos, dur, err := h.readVariables(ctx)
	if err != nil {
		return err
	}
	target := pos + deltaSec
	if target < 0 {
		target = 0
	}
	if dur > 0 && target > dur {
		target = dur
	}
	q := url.Values{}
	q.Set("wm_command", strconv.Itoa(cmdSetPos))
	q.Set("position", secToHMS(target))
	return h.sendCommand(q)
}

// sendCommand issues a command.html query.
func (h *handle) sendCommand(q url.Values) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", h.base+"/command.html?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mpc-hc command %q: status %d", q.Get("wm_command"), resp.StatusCode)
	}
	return nil
}

// secToHMS renders seconds as H:MM:SS for CMD_SETPOS.
func secToHMS(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	total := int(sec)
	return fmt.Sprintf("%d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}

// clampPct bounds a percentage to 0-100.
func clampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Ensure Player satisfies the interface.
var _ player.Player = (*Player)(nil)
