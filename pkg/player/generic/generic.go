// Package generic is a last-resort player driver: it launches an arbitrary
// player executable with the media path (and any configured extra args) and
// nothing more. It exposes no progress and supports no transport control except
// Stop (terminating the process). Use it for players without an automation
// channel; prefer mpv for full remote control.
package generic

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// Player launches a configured executable. It implements player.Player.
type Player struct {
	name      string
	exePath   string
	extraArgs []string
	// argTemplate, when non-nil, builds the full arg list from the request.
	// Default appends just the media path.
	argTemplate func(req player.PlayRequest) []string
}

// Options configures the generic Player.
type Options struct {
	Name      string   // registry key; defaults to "generic"
	ExePath   string   // executable to launch (required to be useful)
	ExtraArgs []string // appended before the media path
	// ArgTemplate overrides argument construction (advanced). Optional.
	ArgTemplate func(req player.PlayRequest) []string
}

// New builds a generic Player.
func New(opts Options) *Player {
	name := opts.Name
	if name == "" {
		name = "generic"
	}
	return &Player{name: name, exePath: opts.ExePath, extraArgs: opts.ExtraArgs, argTemplate: opts.ArgTemplate}
}

// Name is the registry key.
func (p *Player) Name() string { return p.name }

// Start launches the executable on the media path.
func (p *Player) Start(ctx context.Context, req player.PlayRequest) (player.Handle, error) {
	if p.exePath == "" {
		return nil, fmt.Errorf("%s: no executable configured", p.name)
	}
	var args []string
	if p.argTemplate != nil {
		args = p.argTemplate(req)
	} else {
		args = append(args, p.extraArgs...)
		args = append(args, req.MediaPath)
	}
	cmd := exec.CommandContext(ctx, p.exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %s: %w", p.name, err)
	}
	h := &handle{name: p.name, cmd: cmd, done: make(chan struct{})}
	go h.watch()
	return h, nil
}

// handle is a launch-only handle. Implements player.Handle.
type handle struct {
	name     string
	cmd      *exec.Cmd
	doneOnce sync.Once
	done     chan struct{}
}

var _ player.Handle = (*handle)(nil)

func (h *handle) watch() {
	_ = h.cmd.Wait()
	h.doneOnce.Do(func() { close(h.done) })
}

// Progress reports ok=false: a generic player exposes no readable position.
func (h *handle) Progress() (posSec, durSec float64, ok bool) { return 0, 0, false }

// Control supports only Stop (process termination).
func (h *handle) Control(c player.Control) error {
	if c.Cmd == player.CtrlStop {
		if h.cmd.Process != nil {
			return h.cmd.Process.Kill()
		}
		return nil
	}
	return fmt.Errorf("%s: control %d not supported (launch-only)", h.name, c.Cmd)
}

// Wait blocks until the player exits. Final position is unknown (0).
func (h *handle) Wait() (stopSec float64, err error) {
	<-h.done
	return 0, nil
}

// Ensure Player satisfies the interface.
var _ player.Player = (*Player)(nil)
