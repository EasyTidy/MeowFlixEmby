package emby

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
	"github.com/coder/websocket"
)

// TestDecodeCommand covers the three inbound command envelopes, including the
// real-server quirk that ItemIds is a JSON array of numbers, not strings.
func TestDecodeCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		msgType string
		data    string
		check   func(t *testing.T, c remote.Command)
	}{
		{
			name:    "play with numeric item ids",
			msgType: "Play",
			data:    `{"Id":"s","ItemIds":[81535,42],"PlayCommand":"PlayNow","StartPositionTicks":600,"ControllingUserId":"u"}`,
			check: func(t *testing.T, c remote.Command) {
				if c.Type != remote.CmdPlay || c.Play == nil {
					t.Fatalf("want Play, got %+v", c)
				}
				if got := c.Play.ItemIDs; len(got) != 2 || got[0] != "81535" || got[1] != "42" {
					t.Fatalf("item ids = %v", got)
				}
				if c.Play.Command != remote.PlayNow {
					t.Fatalf("command = %v", c.Play.Command)
				}
				if c.Play.StartPositionTick != 600 {
					t.Fatalf("start = %d", c.Play.StartPositionTick)
				}
			},
		},
		{
			name:    "play with string item ids",
			msgType: "Play",
			data:    `{"ItemIds":["abc"],"PlayCommand":"PlayNext"}`,
			check: func(t *testing.T, c remote.Command) {
				if c.Play == nil || len(c.Play.ItemIDs) != 1 || c.Play.ItemIDs[0] != "abc" {
					t.Fatalf("item ids = %+v", c.Play)
				}
			},
		},
		{
			name:    "playstate seek",
			msgType: "Playstate",
			data:    `{"Command":"Seek","SeekPositionTicks":6000000000,"ControllingUserId":"u"}`,
			check: func(t *testing.T, c remote.Command) {
				if c.Type != remote.CmdPlaystate || c.Playstate == nil {
					t.Fatalf("want Playstate, got %+v", c)
				}
				if c.Playstate.Command != remote.StateSeek || c.Playstate.SeekPositionTicks != 6000000000 {
					t.Fatalf("playstate = %+v", c.Playstate)
				}
			},
		},
		{
			name:    "general display message",
			msgType: "GeneralCommand",
			data:    `{"Name":"DisplayMessage","Arguments":{"Header":"Hi","Text":"probe"}}`,
			check: func(t *testing.T, c remote.Command) {
				if c.Type != remote.CmdGeneral || c.General == nil {
					t.Fatalf("want General, got %+v", c)
				}
				if c.General.Name != "DisplayMessage" || c.General.Arguments["Text"] != "probe" {
					t.Fatalf("general = %+v", c.General)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := decodeCommand(tt.msgType, json.RawMessage(tt.data))
			if !ok {
				t.Fatalf("decode returned ok=false")
			}
			tt.check(t, cmd)
		})
	}
}

// TestWSURL verifies scheme/path derivation for both flavors.
func TestWSURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr   string
		flavor Flavor
		want   string
	}{
		{"http://h:8096", FlavorEmby, "ws://h:8096/embywebsocket?api_key=T&deviceId=D"},
		{"https://h", FlavorEmby, "wss://h/embywebsocket?api_key=T&deviceId=D"},
		{"http://h:8096", FlavorJellyfin, "ws://h:8096/socket?api_key=T&deviceId=D"},
	}
	for _, tc := range cases {
		c := New(Options{Address: tc.addr, DeviceID: "D", Flavor: tc.flavor})
		c.token = "T"
		if got := c.wsURL(); got != tc.want {
			t.Errorf("wsURL(%s,%d) = %s, want %s", tc.addr, tc.flavor, got, tc.want)
		}
	}
}

// TestSessionEndToEnd runs the session against a mock Emby that answers the
// capabilities POST, upgrades the WebSocket, sends a ForceKeepAlive (expecting a
// reply) and a Play command, and asserts the command surfaces on Commands().
func TestSessionEndToEnd(t *testing.T) {
	t.Parallel()
	gotKeepAlive := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/Sessions/Capabilities/Full") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/embywebsocket") {
			http.NotFound(w, r)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		_ = c.Write(ctx, websocket.MessageText, []byte(`{"MessageType":"ForceKeepAlive","Data":60}`))
		// The client sends SessionsStart on connect; read frames until we see the
		// KeepAlive reply to our ForceKeepAlive.
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if strings.Contains(string(data), `"MessageType":"KeepAlive"`) {
				select {
				case gotKeepAlive <- struct{}{}:
				default:
				}
				break
			}
		}
		_ = c.Write(ctx, websocket.MessageText,
			[]byte(`{"MessageType":"Play","Data":{"ItemIds":[7],"PlayCommand":"PlayNow","StartPositionTicks":0}}`))
		<-ctx.Done()
	}))
	defer srv.Close()

	client := New(Options{Address: srv.URL, DeviceID: "D", Flavor: FlavorEmby, HTTPClient: srv.Client()})
	client.token = "tok"
	sess := client.NewSession(nil, mediaserver.Capabilities{SupportsMediaControl: true})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = sess.Run(ctx) }()

	// Some servers use the KeepAlive we sent SessionsStart is fire-and-forget;
	// but ForceKeepAlive must be answered.
	select {
	case <-gotKeepAlive:
	case <-time.After(3 * time.Second):
		t.Fatal("server never received KeepAlive reply")
	}

	select {
	case cmd := <-sess.Commands():
		if cmd.Type != remote.CmdPlay || cmd.Play == nil || len(cmd.Play.ItemIDs) != 1 || cmd.Play.ItemIDs[0] != "7" {
			t.Fatalf("unexpected command: %+v", cmd)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("never received Play command")
	}
}
