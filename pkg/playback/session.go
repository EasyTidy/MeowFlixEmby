package playback

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/resolver"
)

// autoNextThreshold is the fraction of runtime past which a natural player exit
// is treated as "finished" and the next queue item auto-plays.
const autoNextThreshold = 0.9

// session drives one Play request's queue from index to end. One session runs
// at a time per controller; a new Play cancels the previous.
type session struct {
	ctrl   *Controller
	ctx    context.Context
	cancel context.CancelFunc

	queue     []string
	index     int
	startTick int64

	mu          sync.Mutex
	handle      player.Handle
	userStopped bool
	stepDelta   int // set by advance(): +1 next, -1 prev, 0 natural
}

// run plays queue items until the queue is exhausted, stopped, or ctx is done.
func (s *session) run() {
	for s.ctx.Err() == nil && s.index >= 0 && s.index < len(s.queue) {
		itemID := s.queue[s.index]
		startTick := s.startTick
		s.startTick = 0 // only the first item honours the resume position

		next := s.playItem(itemID, startTick)
		if next == 0 {
			return // stopped or fatal error
		}
		s.index += next
	}
}

// playItem resolves, plays and reports one item. It returns the queue step to
// apply next: +1 to advance, -1 to go back, 0 to stop the session.
func (s *session) playItem(itemID string, startTick int64) int {
	log := s.ctrl.log.With(slog.String("item_id", itemID))

	item, dec, err := s.resolve(itemID)
	if err != nil {
		log.Error("resolve failed", slog.String("err", err.Error()))
		return +1 // skip to next item rather than stalling the queue
	}
	dec = s.finalizeOpenlist(log, item, dec)

	pl, ok := s.ctrl.opts.Players.Select(dec.MediaPath)
	if !ok {
		log.Error("no player available for media", slog.String("path", dec.MediaPath))
		return 0
	}

	req := player.PlayRequest{
		MediaPath: dec.MediaPath,
		MountDisk: dec.MountDisk,
		StartSec:  ticksToSec(startTick),
		SubFile:   dec.SubFile,
		SubIndex:  dec.SubIndex,
		Title:     titleFor(item),
	}
	handle, err := pl.Start(s.ctx, req)
	if err != nil {
		log.Error("player start failed", slog.String("err", err.Error()))
		return 0
	}
	log.Info("playing", slog.String("method", dec.Method.String()),
		slog.String("player", pl.Name()), slog.String("play_method", dec.PlayMethod))

	s.setHandle(handle)
	return s.reportUntilExit(item, dec, handle)
}

// resolve fetches playback info and runs the resolver decision for itemID.
func (s *session) resolve(itemID string) (*mediaserver.MediaItem, resolver.Decision, error) {
	item, err := s.ctrl.opts.Server.ResolveItem(s.ctx, itemID)
	if err != nil {
		return nil, resolver.Decision{}, err
	}
	item.ID = itemID
	dec, err := s.ctrl.opts.Resolver.Resolve(item, s.ctrl.opts.Server, s.ctrl.opts.ResolverCfg)
	return item, dec, err
}

// finalizeOpenlist resolves a MethodOpenlist decision to a cloud-direct raw URL
// via the openlist API. On any failure (or no resolver configured) it falls
// back to an HTTP stream from the media server so playback still succeeds.
func (s *session) finalizeOpenlist(log *slog.Logger, item *mediaserver.MediaItem, dec resolver.Decision) resolver.Decision {
	if dec.Method != resolver.MethodOpenlist {
		return dec
	}
	if s.ctrl.opts.Openlist != nil {
		raw, err := s.ctrl.opts.Openlist.RawURL(s.ctx, dec.OpenlistPath)
		if err == nil {
			dec.MediaPath = raw
			log.Info("openlist resolved", slog.String("path", dec.OpenlistPath))
			return dec
		}
		log.Warn("openlist resolve failed, falling back to http stream",
			slog.String("path", dec.OpenlistPath), slog.String("err", err.Error()))
	}
	// Fallback: stream from the server.
	dec.Method = resolver.MethodHTTPStream
	dec.MediaPath = s.ctrl.opts.Server.StreamURL(item, dec.Source)
	dec.MountDisk = false
	dec.PlayMethod = "DirectStream"
	return dec
}

// reportUntilExit sends start/progress/stop check-ins while the player runs,
// then returns the next queue step based on how playback ended.
func (s *session) reportUntilExit(item *mediaserver.MediaItem, dec resolver.Decision, h player.Handle) int {
	srv := s.ctrl.opts.Server
	base := s.baseState(item, dec)

	_ = srv.ReportStart(s.ctx, base)

	ticker := time.NewTicker(s.ctrl.opts.ProgressEvery)
	defer ticker.Stop()
	done := make(chan float64, 1)
	go func() { pos, _ := h.Wait(); done <- pos }()

	var lastPos, lastDur float64
	for {
		select {
		case <-s.ctx.Done():
			s.reportStop(base, lastPos)
			return 0
		case <-ticker.C:
			if pos, dur, ok := h.Progress(); ok {
				lastPos, lastDur = pos, dur
				st := base
				st.PositionTicks = secToTicks(pos)
				_ = srv.ReportProgress(s.ctx, st)
			}
		case pos := <-done:
			// Take a final reading so short items (no ticker tick yet) still get
			// an accurate position/duration for the auto-next decision.
			if p, d, ok := h.Progress(); ok {
				lastPos, lastDur = p, d
			}
			if pos > 0 {
				lastPos = pos
			}
			s.reportStop(base, lastPos)
			return s.nextStep(lastPos, lastDur)
		}
	}
}

// nextStep decides the queue movement after the player exits.
func (s *session) nextStep(lastPos, lastDur float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.userStopped {
		return 0
	}
	if s.stepDelta != 0 {
		d := s.stepDelta
		s.stepDelta = 0
		return d
	}
	// Natural exit: auto-advance only if we finished (watched ≥ threshold).
	if lastDur > 0 && lastPos >= autoNextThreshold*lastDur {
		return +1
	}
	return 0
}

// baseState builds the PlaybackState template for reports.
func (s *session) baseState(item *mediaserver.MediaItem, dec resolver.Decision) mediaserver.PlaybackState {
	msID := ""
	if dec.Source != nil {
		msID = dec.Source.ID
	}
	subIdx := -1
	if dec.SubIndex > 0 {
		subIdx = dec.SubIndex
	}
	return mediaserver.PlaybackState{
		ItemID:              item.ID,
		PlaySessionID:       item.PlaySessionID,
		MediaSourceID:       msID,
		CanSeek:             true,
		PlayMethod:          dec.PlayMethod,
		SubtitleStreamIndex: subIdx,
	}
}

func (s *session) reportStop(base mediaserver.PlaybackState, pos float64) {
	st := base
	st.PositionTicks = secToTicks(pos)
	// Use a short detached context so stop still reports after ctx cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.ctrl.opts.Server.ReportStopped(ctx, st)
}

// titleFor picks the best available display title for the player window.
func titleFor(item *mediaserver.MediaItem) string {
	if item.SeriesName != "" && item.Name != "" {
		return item.SeriesName + " - " + item.Name
	}
	if item.Name != "" {
		return item.Name
	}
	if len(item.Sources) > 0 {
		return item.Sources[0].Name
	}
	return ""
}

// --- handle/state accessors used by the controller from other goroutines ---

func (s *session) setHandle(h player.Handle) {
	s.mu.Lock()
	s.handle = h
	s.mu.Unlock()
}

func (s *session) control(c player.Control) {
	s.mu.Lock()
	h := s.handle
	s.mu.Unlock()
	if h != nil {
		if err := h.Control(c); err != nil {
			s.ctrl.log.Warn("player control failed", slog.String("err", err.Error()))
		}
	}
}

// advance requests moving to another queue item: it records the delta and quits
// the current player so the run loop applies it.
func (s *session) advance(delta int) {
	s.mu.Lock()
	s.stepDelta = delta
	h := s.handle
	s.mu.Unlock()
	if h != nil {
		_ = h.Control(player.Control{Cmd: player.CtrlStop})
	}
}

// stop marks the session as user-stopped and tears down the player + context.
func (s *session) stop() {
	s.mu.Lock()
	s.userStopped = true
	h := s.handle
	s.mu.Unlock()
	if h != nil {
		_ = h.Control(player.Control{Cmd: player.CtrlStop})
	}
	s.cancel()
}
