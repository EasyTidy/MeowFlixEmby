package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
	"github.com/coder/websocket"
)

// socketPath returns the WebSocket endpoint path for the flavor.
func (c *Client) socketPath() string {
	if c.opts.Flavor == FlavorJellyfin {
		return "/socket"
	}
	return "/embywebsocket"
}

// wsURL derives the WebSocket URL from the HTTP base (http->ws, https->wss),
// carrying the access token and device id as query parameters.
func (c *Client) wsURL() string {
	scheme := "ws"
	rest := strings.TrimPrefix(c.base, "http://")
	if s := strings.TrimPrefix(c.base, "https://"); s != c.base {
		scheme, rest = "wss", s
	}
	q := url.Values{"api_key": {c.token}, "deviceId": {c.opts.DeviceID}}
	return fmt.Sprintf("%s://%s%s?%s", scheme, rest, c.socketPath(), q.Encode())
}

// Session is the Emby WebSocket remote-control channel. It satisfies
// remote.Session: Run maintains the connection (keep-alive + reconnect) and
// Commands streams decoded inbound commands.
type Session struct {
	client *Client
	log    *slog.Logger
	caps   mediaserver.Capabilities
	cmds   chan remote.Command

	// tunables (defaults set by NewSession)
	reconnectMin time.Duration
	reconnectMax time.Duration
}

// NewSession builds a remote session bound to an authenticated Client. The
// client should already have a token (call Client.Authenticate first); on
// reconnect the session re-announces caps so it stays a "Play On" target.
func (c *Client) NewSession(log *slog.Logger, caps mediaserver.Capabilities) *Session {
	if log == nil {
		log = slog.Default()
	}
	return &Session{
		client:       c,
		log:          log,
		caps:         caps,
		cmds:         make(chan remote.Command, 16),
		reconnectMin: 1 * time.Second,
		reconnectMax: 30 * time.Second,
	}
}

// Commands returns the stream of decoded inbound commands. Closed when Run returns.
func (s *Session) Commands() <-chan remote.Command { return s.cmds }

// Run maintains the connection until ctx is cancelled, reconnecting with
// capped exponential backoff. It always closes Commands before returning.
func (s *Session) Run(ctx context.Context) error {
	defer close(s.cmds)
	backoff := s.reconnectMin
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		start := time.Now()
		err := s.connectOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		// A connection that lived a while resets backoff.
		if time.Since(start) > 30*time.Second {
			backoff = s.reconnectMin
		}
		s.log.Warn("remote session dropped, reconnecting",
			slog.String("err", errString(err)), slog.Duration("in", backoff))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > s.reconnectMax {
			backoff = s.reconnectMax
		}
	}
}

// connectOnce re-announces capabilities, dials the WebSocket, and pumps frames
// until the connection ends or ctx is cancelled.
func (s *Session) connectOnce(ctx context.Context) error {
	// Re-announce so the server re-marks us controllable after any gap.
	if err := s.client.AnnounceCapabilities(ctx, s.caps); err != nil {
		return fmt.Errorf("announce: %w", err)
	}
	conn, _, err := websocket.Dial(ctx, s.client.wsURL(), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Emby-Authorization": {s.client.authHeader()}},
	})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	conn.SetReadLimit(1 << 20)
	s.log.Info("remote session connected", slog.String("device_id", s.client.opts.DeviceID))

	// Ask the server to start pushing session state (harmless if ignored).
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"MessageType":"SessionsStart","Data":"0,1500"}`))

	return s.readLoop(ctx, conn)
}

// readLoop reads frames, answers keep-alives, and forwards decoded commands.
func (s *Session) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var env struct {
			MessageType string          `json:"MessageType"`
			Data        json.RawMessage `json:"Data"`
		}
		if e := json.Unmarshal(data, &env); e != nil {
			s.log.Debug("remote: undecodable frame", slog.String("err", e.Error()))
			continue
		}
		if env.MessageType != "Sessions" {
			s.log.Debug("ws frame", slog.String("type", env.MessageType), slog.Int("bytes", len(data)))
		}
		switch env.MessageType {
		case "KeepAlive", "ForceKeepAlive":
			if e := conn.Write(ctx, websocket.MessageText, []byte(`{"MessageType":"KeepAlive"}`)); e != nil {
				return e
			}
		case "Play", "Playstate", "GeneralCommand":
			if cmd, ok := decodeCommand(env.MessageType, env.Data); ok {
				s.deliver(ctx, cmd)
			}
		default:
			// Sessions, UserDataChanged, etc. — not remote-control, ignore.
		}
	}
}

// deliver pushes a command without blocking the read loop if the consumer is slow.
func (s *Session) deliver(ctx context.Context, cmd remote.Command) {
	select {
	case s.cmds <- cmd:
	case <-ctx.Done():
	default:
		s.log.Warn("remote: command buffer full, dropping", slog.String("type", string(cmd.Type)))
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// itemIDs decodes an Emby ItemIds array that may contain numbers or strings.
type itemIDs []string

func (ids *itemIDs) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(string(r))
		t = strings.Trim(t, `"`)
		out = append(out, t)
	}
	*ids = out
	return nil
}

// decodeCommand converts a raw Emby Data payload into a remote.Command.
func decodeCommand(msgType string, data json.RawMessage) (remote.Command, bool) {
	switch msgType {
	case "Play":
		var d struct {
			ItemIds            itemIDs `json:"ItemIds"`
			PlayCommand        string  `json:"PlayCommand"`
			StartPositionTicks int64   `json:"StartPositionTicks"`
			StartIndex         int     `json:"StartIndex"`
			ControllingUserID  string  `json:"ControllingUserId"`
		}
		if json.Unmarshal(data, &d) != nil {
			return remote.Command{}, false
		}
		return remote.Command{Type: remote.CmdPlay, Play: &remote.PlayRequest{
			ItemIDs:           []string(d.ItemIds),
			Command:           remote.PlayCommand(d.PlayCommand),
			StartPositionTick: d.StartPositionTicks,
			StartIndex:        d.StartIndex,
			ControllingUserID: d.ControllingUserID,
		}}, true
	case "Playstate":
		var d struct {
			Command           string `json:"Command"`
			SeekPositionTicks int64  `json:"SeekPositionTicks"`
			ControllingUserID string `json:"ControllingUserId"`
		}
		if json.Unmarshal(data, &d) != nil {
			return remote.Command{}, false
		}
		return remote.Command{Type: remote.CmdPlaystate, Playstate: &remote.PlaystateRequest{
			Command:           remote.PlaystateCommand(d.Command),
			SeekPositionTicks: d.SeekPositionTicks,
			ControllingUserID: d.ControllingUserID,
		}}, true
	case "GeneralCommand":
		var d struct {
			Name      string            `json:"Name"`
			Arguments map[string]string `json:"Arguments"`
		}
		if json.Unmarshal(data, &d) != nil {
			return remote.Command{}, false
		}
		return remote.Command{Type: remote.CmdGeneral, General: &remote.GeneralRequest{
			Name:      d.Name,
			Arguments: d.Arguments,
		}}, true
	}
	return remote.Command{}, false
}

// Ensure Session satisfies the remote contract at compile time.
var _ remote.Session = (*Session)(nil)
