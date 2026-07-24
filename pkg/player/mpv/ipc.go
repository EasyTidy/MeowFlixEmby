// Package mpv drives the mpv media player over its JSON IPC channel: a named
// pipe on Windows (\\.\pipe\<name>) and a unix socket elsewhere. The wire
// protocol is identical across platforms; only the dial differs (dial_*.go).
package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ipcClient is a JSON-IPC connection to a running mpv instance. It multiplexes
// request/response by request_id and fans out asynchronous events to a handler.
type ipcClient struct {
	conn net.Conn
	enc  *json.Encoder

	reqID   atomic.Int64
	mu      sync.Mutex
	pending map[int64]chan ipcResponse

	onEvent func(ipcEvent)

	closeOnce sync.Once
	closed    chan struct{}
}

// ipcResponse is the reply to a command (matched by request_id).
type ipcResponse struct {
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	RequestID int64           `json:"request_id"`
	Event     string          `json:"event"`
}

// ipcEvent is an asynchronous mpv event (no request_id), e.g. property-change.
type ipcEvent struct {
	Event  string          `json:"event"`
	ID     int64           `json:"id"`   // observe id for property-change
	Name   string          `json:"name"` // property name for property-change
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason"` // for end-file
}

// newIPCClient wraps an established connection and starts the read loop.
func newIPCClient(conn net.Conn, onEvent func(ipcEvent)) *ipcClient {
	c := &ipcClient{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		pending: make(map[int64]chan ipcResponse),
		onEvent: onEvent,
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// readLoop reads newline-delimited JSON messages, routing replies to waiters
// and events to the handler until the connection closes.
func (c *ipcClient) readLoop() {
	defer c.Close()
	sc := bufio.NewScanner(c.conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		// A message with a request_id (or "error" field) is a response; one with
		// an "event" and no request_id is an asynchronous event.
		var probe struct {
			RequestID *int64 `json:"request_id"`
			Event     string `json:"event"`
		}
		if json.Unmarshal(line, &probe) != nil {
			continue
		}
		if probe.Event != "" && probe.RequestID == nil {
			var ev ipcEvent
			if json.Unmarshal(line, &ev) == nil && c.onEvent != nil {
				c.onEvent(ev)
			}
			continue
		}
		var resp ipcResponse
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		c.deliver(resp)
	}
}

// deliver routes a response to its waiting caller.
func (c *ipcClient) deliver(resp ipcResponse) {
	c.mu.Lock()
	ch, ok := c.pending[resp.RequestID]
	if ok {
		delete(c.pending, resp.RequestID)
	}
	c.mu.Unlock()
	if ok {
		ch <- resp
	}
}

// command sends an mpv command array and waits for its response.
func (c *ipcClient) command(ctx context.Context, args ...any) (ipcResponse, error) {
	id := c.reqID.Add(1)
	ch := make(chan ipcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	msg := map[string]any{"command": args, "request_id": id}
	c.mu.Lock() // serialise writes
	err := c.enc.Encode(msg)
	c.mu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ipcResponse{}, fmt.Errorf("mpv write: %w", err)
	}

	select {
	case <-ctx.Done():
		return ipcResponse{}, ctx.Err()
	case <-c.closed:
		return ipcResponse{}, fmt.Errorf("mpv connection closed")
	case resp := <-ch:
		if resp.Error != "" && resp.Error != "success" {
			return resp, fmt.Errorf("mpv command %v: %s", args, resp.Error)
		}
		return resp, nil
	}
}

// observeProperty subscribes to change events for a property under observe id.
func (c *ipcClient) observeProperty(ctx context.Context, id int64, name string) error {
	_, err := c.command(ctx, "observe_property", id, name)
	return err
}

// setProperty sets an mpv property.
func (c *ipcClient) setProperty(ctx context.Context, name string, value any) error {
	_, err := c.command(ctx, "set_property", name, value)
	return err
}

// Close shuts the connection and unblocks any waiters.
func (c *ipcClient) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
	return nil
}

// jsonNumber decodes a JSON number into f. It returns an error for JSON null
// (mpv sends null for time-pos/duration before playback data is available).
func jsonNumber(raw json.RawMessage, f *float64) error {
	return json.Unmarshal(raw, f)
}

// dialWithRetry repeatedly attempts to dial the IPC endpoint until it succeeds
// or the deadline passes (mpv needs a moment after launch to create the socket).
func dialWithRetry(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := dialIPC(addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("dial mpv ipc %q: %w", addr, lastErr)
}
