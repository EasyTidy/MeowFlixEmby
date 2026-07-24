// Package vlc drives the VLC media player over its built-in HTTP interface
// (--extraintf http). Transport control is issued via /requests/status.xml
// command queries; progress is read from the same status document.
package vlc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// Player launches VLC and controls it over the HTTP interface.
type Player struct {
	exePath     string
	extraArgs   []string
	dialTimeout time.Duration
}

// Options configures the VLC Player.
type Options struct {
	ExePath     string        // VLC executable; defaults to "vlc" (on PATH)
	ExtraArgs   []string      // appended to each launch
	DialTimeout time.Duration // how long to wait for the HTTP interface
}

// New builds a VLC Player.
func New(opts Options) *Player {
	exe := opts.ExePath
	if exe == "" {
		exe = "vlc"
	}
	dt := opts.DialTimeout
	if dt == 0 {
		dt = 10 * time.Second
	}
	return &Player{exePath: exe, extraArgs: opts.ExtraArgs, dialTimeout: dt}
}

// Name is the registry key.
func (p *Player) Name() string { return "vlc" }

// Start launches VLC with the HTTP interface enabled and waits for it to answer.
func (p *Player) Start(ctx context.Context, req player.PlayRequest) (player.Handle, error) {
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("vlc: pick port: %w", err)
	}
	password := randomHex()

	args := []string{
		"--extraintf", "http",
		"--http-host", "127.0.0.1",
		"--http-port", strconv.Itoa(port),
		"--http-password", password,
		"--no-video-title-show",
		"--play-and-exit", // quit when playback ends so auto-next can trigger
	}
	if req.StartSec > 0 {
		args = append(args, "--start-time="+strconv.FormatFloat(req.StartSec, 'f', 0, 64))
	}
	if req.SubFile != "" {
		args = append(args, "--sub-file="+req.SubFile)
	}
	if req.Title != "" {
		args = append(args, "--meta-title="+req.Title)
	}
	args = append(args, p.extraArgs...)
	args = append(args, req.MediaPath)
	for _, q := range req.Playlist {
		args = append(args, q.MediaPath)
	}

	cmd := exec.CommandContext(ctx, p.exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch vlc: %w", err)
	}

	h := &handle{
		cmd:      cmd,
		base:     fmt.Sprintf("http://127.0.0.1:%d", port),
		password: password,
		http:     &http.Client{Timeout: 3 * time.Second},
		done:     make(chan struct{}),
	}
	if err := h.waitReady(ctx, p.dialTimeout); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("vlc http interface: %w", err)
	}
	go h.watch()
	return h, nil
}

// freePort asks the OS for an unused TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomHex() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// statusXML is the subset of /requests/status.xml we read.
type statusXML struct {
	Time   float64 `xml:"time"`
	Length float64 `xml:"length"`
	State  string  `xml:"state"`
}

// handle controls a running VLC via HTTP. Implements player.Handle.
type handle struct {
	cmd      *exec.Cmd
	base     string
	password string
	http     *http.Client

	mu     sync.RWMutex
	posSec float64
	durSec float64

	doneOnce sync.Once
	done     chan struct{}
}

var _ player.Handle = (*handle)(nil)

// waitReady polls the status endpoint until VLC answers or the deadline passes.
func (h *handle) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := h.status(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("not ready after %s", timeout)
}

// watch waits for VLC to exit and signals done.
func (h *handle) watch() {
	_ = h.cmd.Wait()
	h.doneOnce.Do(func() { close(h.done) })
}

// status fetches and parses the VLC status document, caching position/length.
func (h *handle) status(ctx context.Context) (statusXML, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", h.base+"/requests/status.xml", nil)
	if err != nil {
		return statusXML{}, err
	}
	req.SetBasicAuth("", h.password)
	resp, err := h.http.Do(req)
	if err != nil {
		return statusXML{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return statusXML{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var st statusXML
	if err := xml.NewDecoder(resp.Body).Decode(&st); err != nil {
		return statusXML{}, err
	}
	h.mu.Lock()
	h.posSec, h.durSec = st.Time, st.Length
	h.mu.Unlock()
	return st, nil
}

// Progress returns the last observed position and duration.
func (h *handle) Progress() (posSec, durSec float64, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = h.status(ctx) // refresh best-effort
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.posSec, h.durSec, true
}

// Wait blocks until VLC exits and returns the last observed position.
func (h *handle) Wait() (stopSec float64, err error) {
	<-h.done
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.posSec, nil
}

// Control forwards a transport command via the VLC HTTP command interface.
func (h *handle) Control(c player.Control) error {
	q := url.Values{}
	switch c.Cmd {
	case player.CtrlPause:
		q.Set("command", "pl_forcepause")
	case player.CtrlUnpause:
		q.Set("command", "pl_forceresume")
	case player.CtrlPlayPause:
		q.Set("command", "pl_pause")
	case player.CtrlStop:
		q.Set("command", "pl_stop")
	case player.CtrlNextTrack:
		q.Set("command", "pl_next")
	case player.CtrlPreviousTrack:
		q.Set("command", "pl_previous")
	case player.CtrlSeekAbsolute:
		q.Set("command", "seek")
		q.Set("val", strconv.FormatFloat(c.SeekSec, 'f', 0, 64))
	case player.CtrlSeekRelative:
		q.Set("command", "seek")
		q.Set("val", relSeek(c.SeekSec))
	case player.CtrlSetVolume:
		q.Set("command", "volume")
		q.Set("val", strconv.Itoa(volumeToVLC(c.Volume))) // 0-100 -> 0-256
	case player.CtrlMute:
		q.Set("command", "volume")
		q.Set("val", "0")
	case player.CtrlUnmute:
		// VLC HTTP has no unmute; restore to 100% (256).
		q.Set("command", "volume")
		q.Set("val", "256")
	case player.CtrlSetAudio:
		q.Set("command", "audio_track")
		q.Set("val", strconv.Itoa(c.TrackIndex))
	case player.CtrlSetSubtitle:
		q.Set("command", "subtitle_track")
		q.Set("val", strconv.Itoa(c.TrackIndex))
	case player.CtrlDisplayMsg:
		// VLC's HTTP interface exposes no OSD/marquee command, so on-screen
		// messages can't be shown. Treat as a successful no-op rather than an
		// error so the server-side command isn't reported as failed.
		return nil
	default:
		return fmt.Errorf("vlc: unsupported control %d", c.Cmd)
	}
	return h.sendCommand(q)
}

// sendCommand issues a status.xml command query.
func (h *handle) sendCommand(q url.Values) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", h.base+"/requests/status.xml?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("", h.password)
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vlc command %q: status %d", q.Get("command"), resp.StatusCode)
	}
	return nil
}

// relSeek renders a relative seek value with an explicit sign (e.g. "+30", "-30").
func relSeek(sec float64) string {
	if sec >= 0 {
		return "+" + strconv.FormatFloat(sec, 'f', 0, 64)
	}
	return strconv.FormatFloat(sec, 'f', 0, 64)
}

// volumeToVLC maps a 0-100 percentage to VLC's 0-256 (256 = 100%) scale.
func volumeToVLC(pct int) int {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct * 256 / 100
}

// Ensure Player satisfies the interface.
var _ player.Player = (*Player)(nil)
