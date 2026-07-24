package mpv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// observe ids for property-change events.
const (
	obsTimePos  int64 = 1
	obsDuration int64 = 2
	obsPause    int64 = 3
)

// Player launches mpv and controls it over JSON IPC. It implements player.Player.
type Player struct {
	exePath string
	// extraArgs are appended to every launch (e.g. --profile=...); optional.
	extraArgs   []string
	dialTimeout time.Duration
}

// Options configures the mpv Player.
type Options struct {
	// ExePath is the mpv executable path. Defaults to "mpv" (found on PATH).
	ExePath string
	// ExtraArgs are appended to each launch.
	ExtraArgs []string
	// DialTimeout bounds how long to wait for mpv's IPC socket after launch.
	DialTimeout time.Duration
}

// New builds an mpv Player.
func New(opts Options) *Player {
	exe := opts.ExePath
	if exe == "" {
		exe = "mpv"
	}
	dt := opts.DialTimeout
	if dt == 0 {
		dt = 10 * time.Second
	}
	return &Player{exePath: exe, extraArgs: opts.ExtraArgs, dialTimeout: dt}
}

// Name is the registry key.
func (p *Player) Name() string { return "mpv" }

// Start launches mpv on the request's media and connects the IPC channel.
func (p *Player) Start(ctx context.Context, req player.PlayRequest) (player.Handle, error) {
	id := randomID()
	ipcArg, dialAddr := ipcServerName(id)

	args := []string{
		"--input-ipc-server=" + ipcArg,
		"--force-window=yes",
		"--keep-open=no",
	}
	if req.StartSec > 0 {
		args = append(args, "--start="+strconv.FormatFloat(req.StartSec, 'f', 3, 64))
	}
	if req.Title != "" {
		args = append(args, "--force-media-title="+req.Title)
	}
	if req.SubFile != "" {
		args = append(args, "--sub-file="+req.SubFile)
	}
	if req.SubIndex > 0 {
		args = append(args, "--sid="+strconv.Itoa(req.SubIndex))
	}
	args = append(args, p.extraArgs...)
	args = append(args, "--", req.MediaPath)
	for _, q := range req.Playlist {
		args = append(args, q.MediaPath)
	}

	cmd := exec.CommandContext(ctx, p.exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch mpv: %w", err)
	}

	h := &handle{cmd: cmd, canSeek: true, done: make(chan struct{})}
	conn, err := dialWithRetry(ctx, dialAddr, p.dialTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("connect mpv ipc: %w", err)
	}
	h.ipc = newIPCClient(conn, h.onEvent)

	// Observe playback properties for progress reporting.
	obsCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.ipc.observeProperty(obsCtx, obsTimePos, "time-pos")
	_ = h.ipc.observeProperty(obsCtx, obsDuration, "duration")
	_ = h.ipc.observeProperty(obsCtx, obsPause, "pause")

	go h.watch()
	return h, nil
}

// randomID returns a short random hex id for the IPC endpoint name.
func randomID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// handle observes and controls a running mpv instance. Implements player.Handle.
type handle struct {
	cmd *exec.Cmd
	ipc *ipcClient

	mu      sync.RWMutex
	posSec  float64
	durSec  float64
	canSeek bool

	doneOnce sync.Once
	done     chan struct{}
}

var _ player.Handle = (*handle)(nil)

// onEvent caches property-change values pushed by mpv.
func (h *handle) onEvent(ev ipcEvent) {
	if ev.Event != "property-change" || len(ev.Data) == 0 {
		return
	}
	var f float64
	if err := jsonNumber(ev.Data, &f); err == nil {
		h.mu.Lock()
		switch ev.ID {
		case obsTimePos:
			h.posSec = f
		case obsDuration:
			h.durSec = f
		}
		h.mu.Unlock()
	}
}

// watch waits for the process to exit and signals done.
func (h *handle) watch() {
	_ = h.cmd.Wait()
	h.signalDone()
}

func (h *handle) signalDone() {
	h.doneOnce.Do(func() {
		close(h.done)
		_ = h.ipc.Close()
	})
}

// Progress returns the last observed position and duration.
func (h *handle) Progress() (posSec, durSec float64, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.posSec, h.durSec, true
}

// Control forwards a transport command over IPC.
func (h *handle) Control(c player.Control) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	switch c.Cmd {
	case player.CtrlPause:
		return h.ipc.setProperty(ctx, "pause", true)
	case player.CtrlUnpause:
		return h.ipc.setProperty(ctx, "pause", false)
	case player.CtrlPlayPause:
		_, err := h.ipc.command(ctx, "cycle", "pause")
		return err
	case player.CtrlStop:
		_, err := h.ipc.command(ctx, "quit")
		return err
	case player.CtrlNextTrack:
		_, err := h.ipc.command(ctx, "playlist-next", "force")
		return err
	case player.CtrlPreviousTrack:
		_, err := h.ipc.command(ctx, "playlist-prev", "force")
		return err
	case player.CtrlSeekAbsolute:
		_, err := h.ipc.command(ctx, "seek", c.SeekSec, "absolute")
		return err
	case player.CtrlSeekRelative:
		_, err := h.ipc.command(ctx, "seek", c.SeekSec, "relative")
		return err
	case player.CtrlSetVolume:
		return h.ipc.setProperty(ctx, "volume", clampVolume(c.Volume))
	case player.CtrlMute:
		return h.ipc.setProperty(ctx, "mute", true)
	case player.CtrlUnmute:
		return h.ipc.setProperty(ctx, "mute", false)
	case player.CtrlToggleMute:
		_, err := h.ipc.command(ctx, "cycle", "mute")
		return err
	case player.CtrlSetSubtitle:
		if c.TrackIndex < 0 {
			return h.ipc.setProperty(ctx, "sub-visibility", false)
		}
		_ = h.ipc.setProperty(ctx, "sub-visibility", true)
		return h.selectTrack(ctx, "sub", "sid", c.TrackIndex)
	case player.CtrlSetAudio:
		return h.selectTrack(ctx, "audio", "aid", c.TrackIndex)
	case player.CtrlDisplayMsg:
		text := c.Header
		if c.Text != "" {
			if text != "" {
				text += ": "
			}
			text += c.Text
		}
		ms := c.TimeoutMs
		if ms <= 0 {
			ms = 4000
		}
		_, err := h.ipc.command(ctx, "show-text", text, ms)
		return err
	default:
		return fmt.Errorf("mpv: unsupported control %d", c.Cmd)
	}
}

// selectTrack sets mpv's aid/sid property to the track whose ffmpeg stream
// index (ff-index) equals embyIndex. Emby reports absolute MediaStream indices
// (matching ffprobe), whereas mpv numbers aid/sid per track type — so we look
// up the mpv track id by ff-index. Falls back to setting the property to the
// raw index if the track-list can't be read.
func (h *handle) selectTrack(ctx context.Context, typ, prop string, embyIndex int) error {
	resp, err := h.ipc.command(ctx, "get_property", "track-list")
	if err != nil {
		return h.ipc.setProperty(ctx, prop, embyIndex)
	}
	var tracks []struct {
		ID      int    `json:"id"`
		Type    string `json:"type"`
		FFIndex int    `json:"ff-index"`
	}
	if json.Unmarshal(resp.Data, &tracks) != nil {
		return h.ipc.setProperty(ctx, prop, embyIndex)
	}
	for _, t := range tracks {
		if t.Type == typ && t.FFIndex == embyIndex {
			return h.ipc.setProperty(ctx, prop, t.ID)
		}
	}
	return fmt.Errorf("mpv: no %s track with ff-index %d", typ, embyIndex)
}

// clampVolume bounds an incoming volume to mpv's 0-100 range.
func clampVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Wait blocks until mpv exits and returns the last observed position.
func (h *handle) Wait() (stopSec float64, err error) {
	<-h.done
	h.mu.RLock()
	pos := h.posSec
	h.mu.RUnlock()
	return pos, nil
}

// Ensure Player satisfies the interface at compile time.
var _ player.Player = (*Player)(nil)
