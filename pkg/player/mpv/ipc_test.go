package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
)

// fakeMPV plays the server side of an mpv IPC connection over a net.Pipe. It
// echoes success for every command and can push events.
type fakeMPV struct {
	conn net.Conn
	dec  *bufio.Scanner
	enc  *json.Encoder
}

func newFakeMPV(conn net.Conn) *fakeMPV {
	return &fakeMPV{conn: conn, dec: bufio.NewScanner(conn), enc: json.NewEncoder(conn)}
}

// serveEcho reads commands and replies with success, capturing them on cmds.
func (f *fakeMPV) serveEcho(cmds chan<- []any) {
	for f.dec.Scan() {
		var msg struct {
			Command   []any `json:"command"`
			RequestID int64 `json:"request_id"`
		}
		if json.Unmarshal(f.dec.Bytes(), &msg) != nil {
			continue
		}
		select {
		case cmds <- msg.Command:
		default:
		}
		_ = f.enc.Encode(map[string]any{"error": "success", "request_id": msg.RequestID, "data": nil})
	}
}

func (f *fakeMPV) pushProperty(id int64, name string, value any) {
	_ = f.enc.Encode(map[string]any{"event": "property-change", "id": id, "name": name, "data": value})
}

func TestIPCCommandRoundTrip(t *testing.T) {
	t.Parallel()
	cli, srv := net.Pipe()
	fake := newFakeMPV(srv)
	cmds := make(chan []any, 8)
	go fake.serveEcho(cmds)

	c := newIPCClient(cli, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.setProperty(ctx, "pause", true); err != nil {
		t.Fatalf("setProperty: %v", err)
	}
	select {
	case got := <-cmds:
		if len(got) != 3 || got[0] != "set_property" || got[1] != "pause" || got[2] != true {
			t.Fatalf("unexpected command: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("fake mpv never received command")
	}

	if _, err := c.command(ctx, "seek", 42.0, "absolute"); err != nil {
		t.Fatalf("seek command: %v", err)
	}
	select {
	case got := <-cmds:
		if got[0] != "seek" || got[2] != "absolute" {
			t.Fatalf("unexpected seek: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("seek not received")
	}
}

func TestIPCCommandError(t *testing.T) {
	t.Parallel()
	cli, srv := net.Pipe()
	enc := json.NewEncoder(srv)
	sc := bufio.NewScanner(srv)
	go func() {
		for sc.Scan() {
			var msg struct {
				RequestID int64 `json:"request_id"`
			}
			_ = json.Unmarshal(sc.Bytes(), &msg)
			_ = enc.Encode(map[string]any{"error": "property unavailable", "request_id": msg.RequestID})
		}
	}()

	c := newIPCClient(cli, nil)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.command(ctx, "get_property", "duration"); err == nil {
		t.Fatal("expected error for unavailable property")
	}
}

func TestHandleProgressFromEvents(t *testing.T) {
	t.Parallel()
	cli, srv := net.Pipe()
	fake := newFakeMPV(srv)
	go fake.serveEcho(make(chan []any, 8))

	h := &handle{done: make(chan struct{})}
	h.ipc = newIPCClient(cli, h.onEvent)
	defer h.ipc.Close()

	fake.pushProperty(obsTimePos, "time-pos", 123.5)
	fake.pushProperty(obsDuration, "duration", 3600.0)

	// events are async; poll briefly
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pos, dur, _ := h.Progress(); pos == 123.5 && dur == 3600.0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	pos, dur, _ := h.Progress()
	t.Fatalf("progress not updated: pos=%v dur=%v", pos, dur)
}

// TestControlCommands verifies the new volume/mute/message controls map to the
// expected mpv IPC commands.
func TestControlCommands(t *testing.T) {
	t.Parallel()
	cli, srv := net.Pipe()
	fake := newFakeMPV(srv)
	cmds := make(chan []any, 16)
	go fake.serveEcho(cmds)

	h := &handle{done: make(chan struct{})}
	h.ipc = newIPCClient(cli, nil)
	defer h.ipc.Close()

	tests := []struct {
		ctrl  player.Control
		want0 string // first element of the mpv command array
	}{
		{player.Control{Cmd: player.CtrlSetVolume, Volume: 60}, "set_property"},
		{player.Control{Cmd: player.CtrlMute}, "set_property"},
		{player.Control{Cmd: player.CtrlUnmute}, "set_property"},
		{player.Control{Cmd: player.CtrlToggleMute}, "cycle"},
		{player.Control{Cmd: player.CtrlDisplayMsg, Header: "H", Text: "T", TimeoutMs: 1000}, "show-text"},
	}
	for _, tc := range tests {
		if err := h.Control(tc.ctrl); err != nil {
			t.Fatalf("Control(%d): %v", tc.ctrl.Cmd, err)
		}
		select {
		case got := <-cmds:
			if len(got) == 0 || got[0] != tc.want0 {
				t.Errorf("cmd for %d = %v, want first=%q", tc.ctrl.Cmd, got, tc.want0)
			}
		case <-time.After(time.Second):
			t.Fatalf("no command received for %d", tc.ctrl.Cmd)
		}
	}
}

// TestSelectTrackByFFIndex verifies audio/subtitle selection maps Emby's
// absolute ff-index to the correct mpv track id.
func TestSelectTrackByFFIndex(t *testing.T) {
	t.Parallel()
	cli, srv := net.Pipe()
	// Fake that answers get_property track-list with a real list and records sets.
	sets := make(chan []any, 4)
	go func() {
		sc := bufio.NewScanner(srv)
		enc := json.NewEncoder(srv)
		for sc.Scan() {
			var msg struct {
				Command   []any `json:"command"`
				RequestID int64 `json:"request_id"`
			}
			if json.Unmarshal(sc.Bytes(), &msg) != nil {
				continue
			}
			if len(msg.Command) >= 2 && msg.Command[0] == "get_property" && msg.Command[1] == "track-list" {
				list := `[{"id":1,"type":"video","ff-index":0},{"id":1,"type":"audio","ff-index":1},{"id":2,"type":"audio","ff-index":2},{"id":1,"type":"sub","ff-index":3}]`
				_, _ = srv.Write([]byte(`{"error":"success","request_id":` + itoa(msg.RequestID) + `,"data":` + list + "}\n"))
				continue
			}
			if len(msg.Command) >= 1 && msg.Command[0] == "set_property" {
				sets <- msg.Command
			}
			_ = enc.Encode(map[string]any{"error": "success", "request_id": msg.RequestID})
		}
	}()

	h := &handle{done: make(chan struct{})}
	h.ipc = newIPCClient(cli, nil)
	defer h.ipc.Close()

	// Emby audio stream index 2 -> mpv audio track id 2.
	if err := h.Control(player.Control{Cmd: player.CtrlSetAudio, TrackIndex: 2}); err != nil {
		t.Fatalf("SetAudio: %v", err)
	}
	select {
	case s := <-sets:
		// s = ["set_property","aid",2]
		if len(s) != 3 || s[1] != "aid" || asInt(s[2]) != 2 {
			t.Fatalf("aid set = %v, want aid=2", s)
		}
	case <-time.After(time.Second):
		t.Fatal("no aid set_property")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
func asInt(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return -1
}

// TestClampVolume covers the mpv 0-100 clamp.
func TestClampVolume(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]int{-5: 0, 0: 0, 55: 55, 100: 100, 120: 100} {
		if got := clampVolume(in); got != want {
			t.Errorf("clampVolume(%d) = %d, want %d", in, got, want)
		}
	}
}

// TestHandleProgressIgnoresNull ensures a null property (pre-playback) does not
// corrupt the cached position.
func TestHandleProgressIgnoresNull(t *testing.T) {
	t.Parallel()
	h := &handle{done: make(chan struct{})}
	h.onEvent(ipcEvent{Event: "property-change", ID: obsTimePos, Name: "time-pos", Data: json.RawMessage("null")})
	if pos, _, _ := h.Progress(); pos != 0 {
		t.Fatalf("null should not set pos, got %v", pos)
	}
}
