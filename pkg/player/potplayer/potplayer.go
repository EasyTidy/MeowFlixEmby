// Package potplayer drives PotPlayer (Windows). PotPlayer has no stable
// runtime IPC/HTTP control channel, so this driver is launch-only: it passes
// start position / subtitle on the command line, can Stop by terminating the
// process, and does not report progress. Transport controls other than Stop are
// reported as unsupported. Use mpv for full remote control.
package potplayer

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"sync"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// Player launches PotPlayer. It implements player.Player.
type Player struct {
	exePath   string
	extraArgs []string
}

// Options configures the PotPlayer Player.
type Options struct {
	ExePath   string // PotPlayer exe; defaults to "PotPlayerMini64.exe" (on PATH)
	ExtraArgs []string
}

// New builds a PotPlayer Player.
func New(opts Options) *Player {
	exe := opts.ExePath
	if exe == "" {
		exe = "PotPlayerMini64.exe"
	}
	return &Player{exePath: exe, extraArgs: opts.ExtraArgs}
}

// Name is the registry key.
func (p *Player) Name() string { return "potplayer" }

// Start launches PotPlayer on the media with start position and subtitle.
func (p *Player) Start(ctx context.Context, req player.PlayRequest) (player.Handle, error) {
	args := []string{req.MediaPath}
	if req.StartSec > 0 {
		// PotPlayer accepts /seek=SECONDS.
		args = append(args, "/seek="+strconv.FormatFloat(req.StartSec, 'f', 0, 64))
	}
	if req.SubFile != "" {
		args = append(args, "/sub="+req.SubFile)
	}
	args = append(args, p.extraArgs...)

	cmd := exec.CommandContext(ctx, p.exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch potplayer: %w", err)
	}
	h := &handle{cmd: cmd, done: make(chan struct{})}
	go h.watch()
	return h, nil
}

// handle is a launch-only PotPlayer handle. Implements player.Handle.
type handle struct {
	cmd      *exec.Cmd
	doneOnce sync.Once
	done     chan struct{}
}

var _ player.Handle = (*handle)(nil)

func (h *handle) watch() {
	_ = h.cmd.Wait()
	h.doneOnce.Do(func() { close(h.done) })
}

// Progress reports ok=false: PotPlayer exposes no readable position here.
func (h *handle) Progress() (posSec, durSec float64, ok bool) {
	return 0, 0, false
}

// Control supports only Stop (process termination); other commands are
// unsupported for PotPlayer's launch-only integration.
func (h *handle) Control(c player.Control) error {
	if c.Cmd == player.CtrlStop {
		if h.cmd.Process != nil {
			return h.cmd.Process.Kill()
		}
		return nil
	}
	return fmt.Errorf("potplayer: control %d not supported (launch-only)", c.Cmd)
}

// Wait blocks until PotPlayer exits. Final position is unknown (0).
func (h *handle) Wait() (stopSec float64, err error) {
	<-h.done
	return 0, nil
}

// Ensure Player satisfies the interface.
var _ player.Player = (*Player)(nil)
