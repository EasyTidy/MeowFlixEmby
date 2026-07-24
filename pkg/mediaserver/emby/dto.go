package emby

import "github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"

// authResult is the AuthenticateByName response (subset).
type authResult struct {
	AccessToken string `json:"AccessToken"`
	ServerID    string `json:"ServerId"`
	User        struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	} `json:"User"`
}

// clientCapabilities is the POST /Sessions/Capabilities/Full body.
type clientCapabilities struct {
	PlayableMediaTypes           []string `json:"PlayableMediaTypes"`
	SupportedCommands            []string `json:"SupportedCommands"`
	SupportsMediaControl         bool     `json:"SupportsMediaControl"`
	SupportsPersistentIdentifier bool     `json:"SupportsPersistentIdentifier"`
}

// playbackInfoResponse is the /Items/{id}/PlaybackInfo response (subset).
type playbackInfoResponse struct {
	MediaSources  []mediaSourceInfo `json:"MediaSources"`
	PlaySessionID string            `json:"PlaySessionId"`
}

// mediaSourceInfo is one MediaSources[] entry (subset).
type mediaSourceInfo struct {
	ID              string            `json:"Id"`
	Name            string            `json:"Name"`
	Path            string            `json:"Path"`
	Protocol        string            `json:"Protocol"`
	Container       string            `json:"Container"`
	IsRemote        bool              `json:"IsRemote"`
	DirectStreamURL string            `json:"DirectStreamUrl"`
	TranscodingURL  string            `json:"TranscodingUrl"`
	RunTimeTicks    int64             `json:"RunTimeTicks"`
	Size            int64             `json:"Size"`
	MediaStreams    []mediaStreamInfo `json:"MediaStreams"`
}

// mediaStreamInfo is one MediaStreams[] entry (subset).
type mediaStreamInfo struct {
	Type         string `json:"Type"`
	Index        int    `json:"Index"`
	Codec        string `json:"Codec"`
	DisplayTitle string `json:"DisplayTitle"`
	Title        string `json:"Title"`
	Language     string `json:"Language"`
	IsExternal   bool   `json:"IsExternal"`
	IsDefault    bool   `json:"IsDefault"`
	DeliveryURL  string `json:"DeliveryUrl"`
}

// playbackReport is the body for /Sessions/Playing[/Progress|/Stopped].
type playbackReport struct {
	ItemID              string `json:"ItemId"`
	PlaySessionID       string `json:"PlaySessionId"`
	MediaSourceID       string `json:"MediaSourceId"`
	PositionTicks       int64  `json:"PositionTicks"`
	IsPaused            bool   `json:"IsPaused"`
	CanSeek             bool   `json:"CanSeek"`
	PlayMethod          string `json:"PlayMethod"`
	AudioStreamIndex    int    `json:"AudioStreamIndex"`
	SubtitleStreamIndex int    `json:"SubtitleStreamIndex"`
	RepeatMode          string `json:"RepeatMode"`
}

// toDomain converts an Emby media source to the domain model.
func (m mediaSourceInfo) toDomain() mediaserver.MediaSource {
	src := mediaserver.MediaSource{
		ID:              m.ID,
		Name:            m.Name,
		Path:            m.Path,
		Protocol:        mediaserver.Protocol(m.Protocol),
		Container:       m.Container,
		IsRemote:        m.IsRemote,
		DirectStreamURL: m.DirectStreamURL,
		TranscodingURL:  m.TranscodingURL,
		RunTimeTicks:    m.RunTimeTicks,
		Size:            m.Size,
	}
	for _, s := range m.MediaStreams {
		src.Streams = append(src.Streams, mediaserver.MediaStream{
			Type:         s.Type,
			Index:        s.Index,
			Codec:        s.Codec,
			DisplayTitle: s.DisplayTitle,
			Title:        s.Title,
			Language:     s.Language,
			IsExternal:   s.IsExternal,
			IsDefault:    s.IsDefault,
			DeliveryURL:  s.DeliveryURL,
		})
	}
	return src
}
