// Package openlist is a thin client for the openlist (AList-compatible) API. It
// resolves a file path inside openlist to a cloud-direct raw URL via
// POST /api/fs/get, so the player can stream straight from the storage backend
// instead of proxying through the media server.
package openlist

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an openlist server.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// Options configures a Client.
type Options struct {
	Host          string // e.g. http://192.168.31.10:5255
	Token         string // openlist API key
	SkipTLSVerify bool
	HTTPClient    *http.Client // optional; a sane default is used when nil
}

// New constructs a Client. It performs no network I/O.
func New(opts Options) *Client {
	hc := opts.HTTPClient
	if hc == nil {
		tr := &http.Transport{}
		if opts.SkipTLSVerify {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in
		}
		hc = &http.Client{Timeout: 20 * time.Second, Transport: tr}
	}
	return &Client{base: strings.TrimRight(opts.Host, "/"), token: opts.Token, http: hc}
}

// fsGetResponse is the /api/fs/get envelope (subset).
type fsGetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		RawURL string `json:"raw_url"`
	} `json:"data"`
}

// RawURL resolves an openlist file path (e.g. "/123Pan/电影/x.mp4") to its
// cloud-direct raw URL. Returns an error if openlist reports failure or the
// response carries no raw URL.
func (c *Client) RawURL(ctx context.Context, path string) (string, error) {
	body, err := json.Marshal(map[string]string{"path": path, "password": ""})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/api/fs/get", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openlist fs/get %q: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("openlist fs/get %q: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var out fsGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode fs/get %q: %w", path, err)
	}
	if out.Code != 200 {
		return "", fmt.Errorf("openlist fs/get %q: code %d: %s", path, out.Code, out.Message)
	}
	if out.Data.RawURL == "" {
		return "", fmt.Errorf("openlist fs/get %q: empty raw_url", path)
	}
	return out.Data.RawURL, nil
}
