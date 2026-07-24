package mpc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// mockMPC serves /variables.html and /command.html, recording commands.
type mockMPC struct {
	mu       sync.Mutex
	commands []url.Values
}

func (m *mockMPC) handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/variables.html":
		_, _ = w.Write([]byte(`<html><p id="position">42000</p><p id="duration">3600000</p><p id="state">2</p></html>`))
	case "/command.html":
		m.mu.Lock()
		m.commands = append(m.commands, r.URL.Query())
		m.mu.Unlock()
		_, _ = w.Write([]byte("OK"))
	default:
		http.NotFound(w, r)
	}
}

func newTestHandle(t *testing.T) (*handle, *mockMPC, func()) {
	t.Helper()
	mock := &mockMPC{}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	h := &handle{base: srv.URL, http: srv.Client(), webReady: true, done: make(chan struct{})}
	return h, mock, srv.Close
}

func TestMPCProgress(t *testing.T) {
	t.Parallel()
	h, _, closeFn := newTestHandle(t)
	defer closeFn()
	pos, dur, ok := h.Progress()
	if !ok || pos != 42 || dur != 3600 {
		t.Fatalf("progress = %v/%v ok=%v, want 42/3600", pos, dur, ok)
	}
}

func TestMPCControlMapping(t *testing.T) {
	t.Parallel()
	h, mock, closeFn := newTestHandle(t)
	defer closeFn()

	cases := []struct {
		ctrl    player.Control
		wantCmd string
		wantKV  [2]string // extra param key/value ("" to skip)
	}{
		{player.Control{Cmd: player.CtrlPause}, "888", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlUnpause}, "887", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlPlayPause}, "889", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlStop}, "890", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlNextTrack}, "922", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlPreviousTrack}, "921", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlMute}, "909", [2]string{"", ""}},
		{player.Control{Cmd: player.CtrlSetVolume, Volume: 40}, "-2", [2]string{"volume", "40"}},
		{player.Control{Cmd: player.CtrlSeekAbsolute, SeekSec: 3661}, "-1", [2]string{"position", "1:01:01"}},
	}
	for _, tc := range cases {
		if err := h.Control(tc.ctrl); err != nil {
			t.Fatalf("Control(%d): %v", tc.ctrl.Cmd, err)
		}
	}
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.commands) != len(cases) {
		t.Fatalf("got %d commands, want %d", len(mock.commands), len(cases))
	}
	for i, tc := range cases {
		q := mock.commands[i]
		if q.Get("wm_command") != tc.wantCmd {
			t.Errorf("cmd[%d] wm_command = %q, want %q", i, q.Get("wm_command"), tc.wantCmd)
		}
		if tc.wantKV[0] != "" && q.Get(tc.wantKV[0]) != tc.wantKV[1] {
			t.Errorf("cmd[%d] %s = %q, want %q", i, tc.wantKV[0], q.Get(tc.wantKV[0]), tc.wantKV[1])
		}
	}
}

func TestMPCLaunchOnlyRejectsControl(t *testing.T) {
	t.Parallel()
	h := &handle{webReady: false, done: make(chan struct{})}
	if err := h.Control(player.Control{Cmd: player.CtrlPause}); err == nil {
		t.Error("launch-only should reject Pause")
	}
}

func TestSecToHMS(t *testing.T) {
	t.Parallel()
	cases := map[float64]string{0: "0:00:00", 61: "0:01:01", 3661: "1:01:01", 3600: "1:00:00", -5: "0:00:00"}
	for sec, want := range cases {
		if got := secToHMS(sec); got != want {
			t.Errorf("secToHMS(%v) = %q, want %q", sec, got, want)
		}
	}
}
