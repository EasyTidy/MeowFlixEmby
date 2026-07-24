package mediaserver

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
