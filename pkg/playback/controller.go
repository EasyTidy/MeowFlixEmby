package playback

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
	"github.com/EasyTidy/MeowFlixEmby/pkg/resolver"
)

// PlayerSelector picks a player for a given media path. *player.MapRegistry
// satisfies this; the controller needs only the selection method.
type PlayerSelector interface {
	Select(mediaPath string) (player.Player, bool)
}

// OpenlistResolver turns an openlist file path into a cloud-direct raw URL.
// *openlist.Client satisfies it. Optional; nil disables the openlist strategy.
type OpenlistResolver interface {
	RawURL(ctx context.Context, path string) (string, error)
}

// Options configures a Controller.
type Options struct {
	Server        mediaserver.Server
	Resolver      resolver.Resolver
	Players       PlayerSelector
	Openlist      OpenlistResolver // optional
	ResolverCfg   resolver.Config
	DeviceID      string
	Logger        *slog.Logger
	ProgressEvery time.Duration // progress report interval; default 5s
}

// Controller consumes remote commands and orchestrates local playback.
type Controller struct {
	opts Options
	log  *slog.Logger

	mu      sync.Mutex
	current *session
}

// New builds a Controller.
func New(opts Options) *Controller {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ProgressEvery <= 0 {
		opts.ProgressEvery = 5 * time.Second
	}
	return &Controller{opts: opts, log: opts.Logger}
}

// Consume reads commands until the channel closes or ctx is cancelled, then
// stops any in-flight playback. Blocks; run it in a goroutine.
func (c *Controller) Consume(ctx context.Context, cmds <-chan remote.Command) {
	defer c.stopCurrent()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-cmds:
			if !ok {
				return
			}
			c.dispatch(ctx, cmd)
		}
	}
}

// dispatch routes one command.
func (c *Controller) dispatch(ctx context.Context, cmd remote.Command) {
	switch cmd.Type {
	case remote.CmdPlay:
		if cmd.Play != nil {
			c.startPlay(ctx, cmd.Play)
		}
	case remote.CmdPlaystate:
		if cmd.Playstate != nil {
			c.handlePlaystate(cmd.Playstate)
		}
	case remote.CmdGeneral:
		if cmd.General != nil {
			c.handleGeneral(cmd.General)
		}
	}
}

// startPlay cancels any current session and begins a new one for the queue.
func (c *Controller) startPlay(ctx context.Context, req *remote.PlayRequest) {
	if len(req.ItemIDs) == 0 {
		c.log.Warn("play command with no items")
		return
	}
	c.stopCurrent()

	sctx, cancel := context.WithCancel(ctx)
	s := &session{
		ctrl:      c,
		ctx:       sctx,
		cancel:    cancel,
		queue:     req.ItemIDs,
		index:     req.StartIndex,
		startTick: req.StartPositionTick,
	}
	if s.index < 0 || s.index >= len(s.queue) {
		s.index = 0
	}
	c.mu.Lock()
	c.current = s
	c.mu.Unlock()

	go s.run()
}

// handlePlaystate applies a transport command to the current session.
func (c *Controller) handlePlaystate(req *remote.PlaystateRequest) {
	s := c.currentSession()
	if s == nil {
		return
	}
	switch req.Command {
	case remote.StateStop:
		c.stopCurrent()
	case remote.StateNextTrack:
		s.advance(+1)
	case remote.StatePreviousTrack:
		s.advance(-1)
	default:
		if ctrl, ok := mapPlaystate(req); ok {
			s.control(ctrl)
		}
	}
}

// handleGeneral forwards a general command (volume/mute/subtitle/audio/message)
// to the current player when it maps to a control; otherwise it is logged.
func (c *Controller) handleGeneral(req *remote.GeneralRequest) {
	ctrl, ok := mapGeneral(req)
	if !ok {
		c.log.Info("general command ignored (no player mapping)",
			slog.String("name", req.Name), slog.Any("args", req.Arguments))
		return
	}
	s := c.currentSession()
	if s == nil {
		c.log.Debug("general command with no active session", slog.String("name", req.Name))
		return
	}
	s.control(ctrl)
}

// currentSession returns the active session, or nil.
func (c *Controller) currentSession() *session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// stopCurrent tears down the active session, if any.
func (c *Controller) stopCurrent() {
	c.mu.Lock()
	s := c.current
	c.current = nil
	c.mu.Unlock()
	if s != nil {
		s.stop()
	}
}
