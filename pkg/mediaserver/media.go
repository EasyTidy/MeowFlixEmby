package mediaserver

// Protocol identifies how a media source is delivered by the server.
type Protocol string

const (
	ProtocolFile Protocol = "File" // server-side filesystem path
	ProtocolHTTP Protocol = "Http" // remote/streamed source (e.g. cloud-mounted strm)
)

// MediaStream describes one audio/video/subtitle track of a media source.
type MediaStream struct {
	Type         string // Video | Audio | Subtitle
	Index        int
	Codec        string
	DisplayTitle string
	Title        string
	Language     string
	IsExternal   bool
	IsDefault    bool
	DeliveryURL  string // relative URL for external subtitle delivery, if any
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
