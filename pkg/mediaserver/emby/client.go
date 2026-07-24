// Package emby implements the mediaserver.Server contract for Emby (and, with
// minor scheme differences handled here, Jellyfin).
package emby

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

// Flavor selects Emby- vs Jellyfin-specific behaviour (auth scheme, ws path).
type Flavor int

const (
	FlavorEmby Flavor = iota
	FlavorJellyfin
)

// Options configures a Client.
type Options struct {
	Address       string // e.g. https://emby.example.com (no trailing slash needed)
	Username      string
	Password      string
	APIKey        string // optional; if set, used directly as the token
	DeviceName    string
	DeviceID      string // stable, persisted GUID
	ClientVersion string
	Flavor        Flavor
	SkipTLSVerify bool
	HTTPClient    *http.Client // optional; a sane default is used when nil
}

// Client is an Emby/Jellyfin REST client and mediaserver.Server implementation.
type Client struct {
	opts    Options
	base    string
	http    *http.Client
	token   string
	userID  string
	authScheme string
}

// New constructs a Client. It does not perform any network I/O.
func New(opts Options) *Client {
	base := strings.TrimRight(opts.Address, "/")
	hc := opts.HTTPClient
	if hc == nil {
		tr := &http.Transport{}
		if opts.SkipTLSVerify {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for self-signed
		}
		hc = &http.Client{Timeout: 30 * time.Second, Transport: tr}
	}
	scheme := "Emby"
	if opts.Flavor == FlavorJellyfin {
		scheme = "MediaBrowser"
	}
	c := &Client{opts: opts, base: base, http: hc, authScheme: scheme}
	if opts.APIKey != "" {
		c.token = opts.APIKey
	}
	if opts.ClientVersion == "" {
		c.opts.ClientVersion = "1.0.0"
	}
	return c
}

// BaseURL returns the normalised server base URL.
func (c *Client) BaseURL() string { return c.base }

// Token returns the current access token (after Authenticate).
func (c *Client) Token() string { return c.token }

// DeviceID returns the configured device id.
func (c *Client) DeviceID() string { return c.opts.DeviceID }

// authHeader builds the X-Emby-Authorization / Authorization header value.
func (c *Client) authHeader() string {
	// Emby UserId="", Client="..", Device="..", DeviceId="..", Version="..", Token=".."
	return fmt.Sprintf(`%s UserId="%s", Client="MeowFlix", Device=%q, DeviceId=%q, Version="%s", Token="%s"`,
		c.authScheme, c.userID, c.opts.DeviceName, c.opts.DeviceID, c.opts.ClientVersion, c.token)
}

// doJSON performs an HTTP request with auth headers and optional JSON body,
// decoding a JSON response into out (when non-nil).
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	url := c.base + path
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return fmt.Errorf("new request %s %s: %w", method, path, err)
	}
	req.Header.Set("X-Emby-Authorization", c.authHeader())
	req.Header.Set("Authorization", c.authHeader())
	if c.token != "" {
		req.Header.Set("X-Emby-Token", c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return nil
}
