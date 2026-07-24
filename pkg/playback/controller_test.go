package playback

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
	"github.com/EasyTidy/MeowFlixEmby/pkg/resolver"
)

// --- mock server ---

type mockServer struct {
	mu       sync.Mutex
	starts   []string // item ids reported started
	stops    []string
	progress int
}

func (m *mockServer) Authenticate(context.Context) error { return nil }
func (m *mockServer) AnnounceCapabilities(context.Context, mediaserver.Capabilities) error {
	return nil
}
func (m *mockServer) ResolveItem(_ context.Context, id string) (*mediaserver.MediaItem, error) {
	return &mediaserver.MediaItem{
		ID:            id,
		PlaySessionID: "ps-" + id,
		Sources:       []mediaserver.MediaSource{{ID: "src-" + id, Path: "/mnt/disk1/" + id + ".mkv", Name: id}},
	}, nil
}
func (m *mockServer) StreamURL(*mediaserver.MediaItem, *mediaserver.MediaSource) string {
	return "http://s/stream"
}
func (m *mockServer) SubtitleURL(*mediaserver.MediaItem, *mediaserver.MediaSource, *mediaserver.MediaStream) string {
	return ""
}
func (m *mockServer) ReportStart(_ context.Context, s mediaserver.PlaybackState) error {
	m.mu.Lock()
	m.starts = append(m.starts, s.ItemID)
	m.mu.Unlock()
	return nil
}
func (m *mockServer) ReportProgress(context.Context, mediaserver.PlaybackState) error {
	m.mu.Lock()
	m.progress++
	m.mu.Unlock()
	return nil
}
func (m *mockServer) ReportStopped(_ context.Context, s mediaserver.PlaybackState) error {
	m.mu.Lock()
	m.stops = append(m.stops, s.ItemID)
	m.mu.Unlock()
	return nil
}

func (m *mockServer) startedItems() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.starts...)
}

// --- fake player: finishes immediately at a controllable final position ---

type fakePlayer struct {
	finalPos float64
	durSec   float64
	started  chan player.PlayRequest
}

func (f *fakePlayer) Name() string { return "fake" }
func (f *fakePlayer) Start(_ context.Context, req player.PlayRequest) (player.Handle, error) {
	if f.started != nil {
		f.started <- req
	}
	h := &fakeHandle{finalPos: f.finalPos, durSec: f.durSec, done: make(chan struct{})}
	close(h.done) // exits immediately
	return h, nil
}

type fakeHandle struct {
	finalPos float64
	durSec   float64
	done     chan struct{}
}

func (h *fakeHandle) Progress() (float64, float64, bool) { return h.finalPos, h.durSec, true }
func (h *fakeHandle) Control(player.Control) error       { return nil }
func (h *fakeHandle) Wait() (float64, error)             { <-h.done; return h.finalPos, nil }

// selector wraps a single player.
type oneSelector struct{ p player.Player }

func (o oneSelector) Select(string) (player.Player, bool) { return o.p, true }

func newTestController(srv mediaserver.Server, pl player.Player) *Controller {
	return New(Options{
		Server:        srv,
		Resolver:      resolver.New(),
		Players:       oneSelector{pl},
		ResolverCfg:   resolver.Config{PathMaps: []resolver.PathMap{{Src: "/mnt/disk1", Dst: "/local"}}},
		ProgressEvery: 10 * time.Millisecond,
	})
}

// TestAutoNextOnNaturalFinish: an item that ends past the threshold advances.
func TestAutoNextOnNaturalFinish(t *testing.T) {
	t.Parallel()
	srv := &mockServer{}
	fp := &fakePlayer{finalPos: 95, durSec: 100, started: make(chan player.PlayRequest, 4)}
	c := newTestController(srv, fp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.dispatch(ctx, remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
		ItemIDs: []string{"a", "b"}, Command: remote.PlayNow,
	}})

	// both a and b should start (a finishes >90% -> auto-next b)
	waitForStarts(t, srv, 2)
	got := srv.startedItems()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("started = %v, want [a b]", got)
	}
}

// TestNoAutoNextWhenStoppedEarly: ending before threshold does not advance.
func TestNoAutoNextWhenStoppedEarly(t *testing.T) {
	t.Parallel()
	srv := &mockServer{}
	fp := &fakePlayer{finalPos: 10, durSec: 100}
	c := newTestController(srv, fp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.dispatch(ctx, remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
		ItemIDs: []string{"a", "b"}, Command: remote.PlayNow,
	}})

	time.Sleep(200 * time.Millisecond)
	if got := srv.startedItems(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("started = %v, want only [a]", got)
	}
}

// TestResumePositionOnFirstItem: StartPositionTicks maps to StartSec on item 0.
func TestResumePositionOnFirstItem(t *testing.T) {
	t.Parallel()
	srv := &mockServer{}
	fp := &fakePlayer{finalPos: 10, durSec: 100, started: make(chan player.PlayRequest, 2)}
	c := newTestController(srv, fp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.dispatch(ctx, remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
		ItemIDs: []string{"a"}, Command: remote.PlayNow, StartPositionTick: 600 * ticksPerSec,
	}})

	select {
	case req := <-fp.started:
		if req.StartSec != 600 {
			t.Fatalf("StartSec = %v, want 600", req.StartSec)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("player never started")
	}
}

// fakeOpenlist returns a fixed raw URL or an error.
type fakeOpenlist struct {
	url string
	err error
}

func (f fakeOpenlist) RawURL(context.Context, string) (string, error) { return f.url, f.err }

// openlistServer maps every source path so the resolver routes to openlist.
type openlistServer struct{ mockServer }

func (m *openlistServer) ResolveItem(_ context.Context, id string) (*mediaserver.MediaItem, error) {
	return &mediaserver.MediaItem{
		ID:            id,
		PlaySessionID: "ps-" + id,
		Sources:       []mediaserver.MediaSource{{ID: "src", Path: "/volume1/video/123Pan/" + id + ".mp4", Name: id}},
	}, nil
}

func newOpenlistController(srv mediaserver.Server, ol OpenlistResolver, cap chan player.PlayRequest) *Controller {
	return New(Options{
		Server:   srv,
		Resolver: resolver.New(),
		Players:  oneSelector{&fakePlayer{finalPos: 10, durSec: 100, started: cap}},
		Openlist: ol,
		ResolverCfg: resolver.Config{
			OpenlistEnabled:  true,
			OpenlistPathMaps: []resolver.PathMap{{Src: "/volume1/video", Dst: ""}},
		},
		ProgressEvery: 10 * time.Millisecond,
	})
}

func TestOpenlistResolvesToRawURL(t *testing.T) {
	t.Parallel()
	cap := make(chan player.PlayRequest, 2)
	c := newOpenlistController(&openlistServer{}, fakeOpenlist{url: "https://cloud/raw.mp4"}, cap)
	c.dispatch(context.Background(), remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
		ItemIDs: []string{"a"}, Command: remote.PlayNow,
	}})
	select {
	case req := <-cap:
		if req.MediaPath != "https://cloud/raw.mp4" {
			t.Fatalf("MediaPath = %q, want raw url", req.MediaPath)
		}
		if req.MountDisk {
			t.Error("openlist play should not be MountDisk")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("player never started")
	}
}

func TestOpenlistFallsBackToHTTPStream(t *testing.T) {
	t.Parallel()
	cap := make(chan player.PlayRequest, 2)
	// openlist errors -> session falls back to server StreamURL.
	c := newOpenlistController(&openlistServer{}, fakeOpenlist{err: context.DeadlineExceeded}, cap)
	c.dispatch(context.Background(), remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
		ItemIDs: []string{"a"}, Command: remote.PlayNow,
	}})
	select {
	case req := <-cap:
		if req.MediaPath != "http://s/stream" {
			t.Fatalf("MediaPath = %q, want http stream fallback", req.MediaPath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("player never started")
	}
}

func waitForStarts(t *testing.T, srv *mockServer, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.startedItems()) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d starts, got %v", n, srv.startedItems())
}
