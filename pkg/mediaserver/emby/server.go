package emby

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// prefix is the API path prefix. Emby historically uses /emby; modern servers
// accept both. Jellyfin uses the root. We use the root and rely on the server
// accepting it; adjust here if a deployment needs /emby.
const prefix = ""

// Authenticate performs AuthenticateByName and stores the token + user id.
// If an API key was supplied it is used as-is and no request is made.
func (c *Client) Authenticate(ctx context.Context) error {
	if c.opts.APIKey != "" {
		c.token = c.opts.APIKey
		// API-key mode cannot resolve a user id; ResolveItem needs UserId, so
		// callers using api_key must also supply it out of band in future.
		return nil
	}
	var res authResult
	body := map[string]string{"Username": c.opts.Username, "Pw": c.opts.Password}
	if err := c.doJSON(ctx, "POST", prefix+"/Users/AuthenticateByName", body, &res); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	if res.AccessToken == "" {
		return fmt.Errorf("authenticate: empty access token")
	}
	c.token = res.AccessToken
	c.userID = res.User.ID
	return nil
}

// AnnounceCapabilities registers this client as a controllable cast target.
func (c *Client) AnnounceCapabilities(ctx context.Context, caps mediaserver.Capabilities) error {
	body := clientCapabilities{
		PlayableMediaTypes:           caps.PlayableMediaTypes,
		SupportedCommands:            caps.SupportedCommands,
		SupportsMediaControl:         caps.SupportsMediaControl,
		SupportsPersistentIdentifier: caps.SupportsPersistentIdentifier,
	}
	if err := c.doJSON(ctx, "POST", prefix+"/Sessions/Capabilities/Full", body, nil); err != nil {
		return fmt.Errorf("announce capabilities: %w", err)
	}
	return nil
}

// ResolveItem fetches playback info (media sources + PlaySessionId) for itemID.
func (c *Client) ResolveItem(ctx context.Context, itemID string) (*mediaserver.MediaItem, error) {
	q := url.Values{}
	if c.userID != "" {
		q.Set("UserId", c.userID)
	}
	path := fmt.Sprintf("%s/Items/%s/PlaybackInfo", prefix, itemID)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var pir playbackInfoResponse
	body := map[string]any{"UserId": c.userID}
	if err := c.doJSON(ctx, "POST", path, body, &pir); err != nil {
		return nil, fmt.Errorf("resolve item %s: %w", itemID, err)
	}
	item := &mediaserver.MediaItem{
		ID:            itemID,
		PlaySessionID: pir.PlaySessionID,
	}
	for _, s := range pir.MediaSources {
		item.Sources = append(item.Sources, s.toDomain())
	}
	if len(item.Sources) > 0 {
		item.RunTimeTicks = item.Sources[0].RunTimeTicks
	}
	return item, nil
}

// StreamURL builds an absolute static direct-stream URL for a source.
func (c *Client) StreamURL(item *mediaserver.MediaItem, source *mediaserver.MediaSource) string {
	if source.DirectStreamURL != "" {
		return c.joinAbs(source.DirectStreamURL)
	}
	container := strings.TrimPrefix(source.Container, ".")
	q := url.Values{}
	q.Set("Static", "true")
	q.Set("MediaSourceId", source.ID)
	q.Set("PlaySessionId", item.PlaySessionID)
	q.Set("DeviceId", c.opts.DeviceID)
	if c.token != "" {
		q.Set("api_key", c.token)
	}
	name := "stream"
	if container != "" {
		name = "stream." + container
	}
	return fmt.Sprintf("%s%s/Videos/%s/%s?%s", c.base, prefix, item.ID, name, q.Encode())
}

// SubtitleURL builds an absolute external-subtitle URL for a stream.
func (c *Client) SubtitleURL(item *mediaserver.MediaItem, source *mediaserver.MediaSource, stream *mediaserver.MediaStream) string {
	if stream.DeliveryURL != "" {
		return c.joinAbs(stream.DeliveryURL)
	}
	codec := stream.Codec
	if codec == "" {
		codec = "srt"
	}
	suffix := ""
	if c.token != "" {
		suffix = "?api_key=" + c.token
	}
	return fmt.Sprintf("%s%s/Videos/%s/%s/Subtitles/%d/0/Stream.%s%s",
		c.base, prefix, item.ID, source.ID, stream.Index, codec, suffix)
}

// ReportStart posts a playback-start check-in.
func (c *Client) ReportStart(ctx context.Context, s mediaserver.PlaybackState) error {
	return c.report(ctx, "/Sessions/Playing", s)
}

// ReportProgress posts a playback-progress check-in.
func (c *Client) ReportProgress(ctx context.Context, s mediaserver.PlaybackState) error {
	return c.report(ctx, "/Sessions/Playing/Progress", s)
}

// ReportStopped posts a playback-stopped check-in.
func (c *Client) ReportStopped(ctx context.Context, s mediaserver.PlaybackState) error {
	return c.report(ctx, "/Sessions/Playing/Stopped", s)
}

func (c *Client) report(ctx context.Context, path string, s mediaserver.PlaybackState) error {
	body := playbackReport{
		ItemID:              s.ItemID,
		PlaySessionID:       s.PlaySessionID,
		MediaSourceID:       s.MediaSourceID,
		PositionTicks:       s.PositionTicks,
		IsPaused:            s.IsPaused,
		CanSeek:             s.CanSeek,
		PlayMethod:          s.PlayMethod,
		AudioStreamIndex:    s.AudioStreamIndex,
		SubtitleStreamIndex: s.SubtitleStreamIndex,
		RepeatMode:          "RepeatNone",
	}
	if err := c.doJSON(ctx, "POST", prefix+path, body, nil); err != nil {
		return fmt.Errorf("report %s: %w", path, err)
	}
	return nil
}

// joinAbs turns a relative server URL into an absolute one, appending api_key.
func (c *Client) joinAbs(rel string) string {
	abs := c.base + rel
	if c.token == "" || strings.Contains(rel, "api_key=") {
		return abs
	}
	sep := "?"
	if strings.Contains(rel, "?") {
		sep = "&"
	}
	return abs + sep + "api_key=" + c.token
}

// Ensure Client satisfies the interface at compile time.
var _ mediaserver.Server = (*Client)(nil)
