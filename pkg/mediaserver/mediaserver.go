// Package mediaserver defines the pluggable abstraction over a media server
// (Emby, Jellyfin, or Plex). Concrete implementations live in subpackages and
// are selected at wire-up time; nothing here depends on a specific backend.
package mediaserver

import "context"

// Protocol identifies how a media source is delivered by the server.
type Protocol string

const (
	ProtocolFile Protocol = "File" // server-side filesystem path
	ProtocolHTTP Protocol = "Http" // remote/streamed source (e.g. cloud-mounted strm)
)

// Capabilities is what the client advertises so the server lists it as a
// controllable "Play On" / cast target.
type Capabilities struct {
	PlayableMediaTypes           []string
	SupportedCommands            []string
	SupportsMediaControl         bool
	SupportsPersistentIdentifier bool
}

// MediaStream describes one audio/video/subtitle track of a media source.
type MediaStream struct {
	Type          string // Video | Audio | Subtitle
	Index         int
	Codec         string
	DisplayTitle  string
	Title         string
	Language      string
	IsExternal    bool
	IsDefault     bool
	DeliveryURL   string // relative URL for external subtitle delivery, if any
}

// MediaSource is one playable version of an item.
type MediaSource struct {
	ID              string
	Name            string
	Path            string // server-side path (or strm content for http sources)
	Protocol        Protocol
	Container       string
	IsRemote        bool
	DirectStreamURL string // relative; join with server base for HTTP direct stream
	TranscodingURL  string // relative; present only when the server forces transcode
	RunTimeTicks    int64  // 100ns ticks
	Size            int64
	Streams         []MediaStream
}

// MediaItem is the resolved playback information for a library item.
type MediaItem struct {
	ID            string
	Name          string
	SeriesName    string
	Type          string // Movie | Episode | ...
	PlaySessionID string
	RunTimeTicks  int64
	Sources       []MediaSource
}

// PlaybackState is reported to the server on start / progress / stop.
type PlaybackState struct {
	ItemID              string
	PlaySessionID       string
	MediaSourceID       string
	PositionTicks       int64 // 100ns ticks
	IsPaused            bool
	CanSeek             bool
	PlayMethod          string // DirectPlay | DirectStream | Transcode
	AudioStreamIndex    int
	SubtitleStreamIndex int
}

// Server is the pluggable media-server contract.
type Server interface {
	// Authenticate obtains and stores an access token / session.
	Authenticate(ctx context.Context) error

	// AnnounceCapabilities registers this client as a controllable target.
	AnnounceCapabilities(ctx context.Context, caps Capabilities) error

	// ResolveItem fetches playback info (media sources + PlaySessionId) for an item.
	ResolveItem(ctx context.Context, itemID string) (*MediaItem, error)

	// StreamURL builds an absolute HTTP stream URL for a resolved source.
	StreamURL(item *MediaItem, source *MediaSource) string

	// SubtitleURL builds an absolute external-subtitle URL for a stream.
	SubtitleURL(item *MediaItem, source *MediaSource, stream *MediaStream) string

	// Playback check-ins.
	ReportStart(ctx context.Context, s PlaybackState) error
	ReportProgress(ctx context.Context, s PlaybackState) error
	ReportStopped(ctx context.Context, s PlaybackState) error
}
