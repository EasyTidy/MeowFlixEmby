package vlc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// mockVLC serves a minimal /requests/status.xml and records command queries.
type mockVLC struct {
	mu       sync.Mutex
	commands []string
}

func (m *mockVLC) handler(w http.ResponseWriter, r *http.Request) {
	if _, pw, ok := r.BasicAuth(); !ok || pw == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if cmd := r.URL.Query().Get("command"); cmd != "" {
		m.mu.Lock()
		m.commands = append(m.commands, cmd+"|"+r.URL.Query().Get("val"))
		m.mu.Unlock()
	}
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write([]byte(`<root><time>42</time><length>3600</length><state>playing</state></root>`))
}

func newTestHandle(t *testing.T) (*handle, *mockVLC, func()) {
	t.Helper()
	mock := &mockVLC{}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	h := &handle{
		base:     srv.URL,
		password: "pw",
		http:     srv.Client(),
		done:     make(chan struct{}),
	}
	return h, mock, srv.Close
}

func TestVLCProgress(t *testing.T) {
	t.Parallel()
	h, _, closeFn := newTestHandle(t)
	defer closeFn()
	pos, dur, ok := h.Progress()
	if !ok || pos != 42 || dur != 3600 {
		t.Fatalf("progress = %v/%v ok=%v, want 42/3600", pos, dur, ok)
	}
}

func TestVLCControlMapping(t *testing.T) {
	t.Parallel()
	h, mock, closeFn := newTestHandle(t)
	defer closeFn()

	cases := []struct {
		ctrl player.Control
		want string
	}{
		{player.Control{Cmd: player.CtrlPause}, "pl_forcepause|"},
		{player.Control{Cmd: player.CtrlUnpause}, "pl_forceresume|"},
		{player.Control{Cmd: player.CtrlStop}, "pl_stop|"},
		{player.Control{Cmd: player.CtrlSeekAbsolute, SeekSec: 120}, "seek|120"},
		{player.Control{Cmd: player.CtrlSeekRelative, SeekSec: 30}, "seek|+30"},
		{player.Control{Cmd: player.CtrlSeekRelative, SeekSec: -30}, "seek|-30"},
		{player.Control{Cmd: player.CtrlSetVolume, Volume: 50}, "volume|128"}, // 50% -> 128/256
		{player.Control{Cmd: player.CtrlSetAudio, TrackIndex: 2}, "audio_track|2"},
	}
	for _, tc := range cases {
		if err := h.Control(tc.ctrl); err != nil {
			t.Fatalf("Control(%d): %v", tc.ctrl.Cmd, err)
		}
	}
	mock.mu.Lock()
	got := mock.commands
	mock.mu.Unlock()
	if len(got) != len(cases) {
		t.Fatalf("got %d commands, want %d: %v", len(got), len(cases), got)
	}
	for i, tc := range cases {
		if got[i] != tc.want {
			t.Errorf("command[%d] = %q, want %q", i, got[i], tc.want)
		}
	}

	// DisplayMessage is a graceful no-op (VLC HTTP has no OSD command).
	if err := h.Control(player.Control{Cmd: player.CtrlDisplayMsg, Text: "x"}); err != nil {
		t.Errorf("DisplayMessage should no-op, got %v", err)
	}
	// Unmute restores volume to 256.
	if err := h.Control(player.Control{Cmd: player.CtrlUnmute}); err != nil {
		t.Errorf("Unmute: %v", err)
	}
	mock.mu.Lock()
	last := mock.commands[len(mock.commands)-1]
	mock.mu.Unlock()
	if last != "volume|256" {
		t.Errorf("unmute command = %q, want volume|256", last)
	}
}

func TestVolumeToVLC(t *testing.T) {
	t.Parallel()
	cases := map[int]int{-10: 0, 0: 0, 50: 128, 100: 256, 150: 256}
	for pct, want := range cases {
		if got := volumeToVLC(pct); got != want {
			t.Errorf("volumeToVLC(%d) = %d, want %d", pct, got, want)
		}
	}
}

// TestWaitReadyTimeout ensures waitReady gives up when nothing answers.
func TestWaitReadyTimeout(t *testing.T) {
	t.Parallel()
	h := &handle{base: "http://127.0.0.1:1", password: "pw", http: &http.Client{Timeout: 200 * time.Millisecond}, done: make(chan struct{})}
	if err := h.waitReady(context.Background(), 300*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	}
}
