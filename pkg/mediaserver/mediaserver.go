// Package mediaserver defines the pluggable abstraction over a media server
// (Emby, Jellyfin, or Plex). Concrete implementations live in subpackages and
// are selected at wire-up time; nothing here depends on a specific backend.
//
// Types are grouped by concern: media.go holds the media model (Protocol,
// MediaStream, MediaSource, MediaItem), playback.go the PlaybackState report,
// capabilities.go the advertised Capabilities, and this file the Server
// behavior contract that ties them together.
package mediaserver

import "context"

// URLBuilder builds absolute playback URLs for a resolved item/source. It is
// split out from Server so consumers that only need URL construction (e.g. the
// resolver) can depend on this narrow contract instead of the full server.
type URLBuilder interface {
	// StreamURL builds an absolute HTTP stream URL for a resolved source.
	StreamURL(item *MediaItem, source *MediaSource) string
	// SubtitleURL builds an absolute external-subtitle URL for a stream.
	SubtitleURL(item *MediaItem, source *MediaSource, stream *MediaStream) string
}

// Reporter posts playback check-ins (start / progress / stop) to the server.
type Reporter interface {
	ReportStart(ctx context.Context, s PlaybackState) error
	ReportProgress(ctx context.Context, s PlaybackState) error
	ReportStopped(ctx context.Context, s PlaybackState) error
}

// Server is the pluggable media-server contract. It composes the narrower
// URLBuilder and Reporter contracts with session/auth/resolve concerns.
type Server interface {
	// Authenticate obtains and stores an access token / session.
	Authenticate(ctx context.Context) error

	// AnnounceCapabilities registers this client as a controllable target.
	AnnounceCapabilities(ctx context.Context, caps Capabilities) error

	// ResolveItem fetches playback info (media sources + PlaySessionId) for an item.
	ResolveItem(ctx context.Context, itemID string) (*MediaItem, error)

	URLBuilder
	Reporter
}
