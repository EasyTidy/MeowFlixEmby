// Package remote models the WebSocket remote-control / cast session: the
// commands the server pushes to this client once it is a "Play On" target.
package remote

import "context"

// CommandType enumerates the inbound message kinds this client handles.
type CommandType string

const (
	CmdPlay      CommandType = "Play"
	CmdPlaystate CommandType = "Playstate"
	CmdGeneral   CommandType = "GeneralCommand"
)

// PlayCommand is the sub-kind of a Play message.
type PlayCommand string

const (
	PlayNow  PlayCommand = "PlayNow"
	PlayNext PlayCommand = "PlayNext"
	PlayLast PlayCommand = "PlayLast"
)

// PlaystateCommand is the sub-kind of a Playstate message.
type PlaystateCommand string

const (
	StateStop          PlaystateCommand = "Stop"
	StatePause         PlaystateCommand = "Pause"
	StateUnpause       PlaystateCommand = "Unpause"
	StatePlayPause     PlaystateCommand = "PlayPause"
	StateNextTrack     PlaystateCommand = "NextTrack"
	StatePreviousTrack PlaystateCommand = "PreviousTrack"
	StateSeek          PlaystateCommand = "Seek"
	StateRewind        PlaystateCommand = "Rewind"
	StateFastForward   PlaystateCommand = "FastForward"
	StateSeekRelative  PlaystateCommand = "SeekRelative"
)

// PlayRequest is the decoded payload of a Play message.
type PlayRequest struct {
	ItemIDs           []string
	Command           PlayCommand
	StartPositionTick int64 // 100ns ticks
	StartIndex        int
	ControllingUserID string
}

// PlaystateRequest is the decoded payload of a Playstate message.
type PlaystateRequest struct {
	Command           PlaystateCommand
	SeekPositionTicks int64 // 100ns ticks (Seek / SeekRelative)
	ControllingUserID string
}

// GeneralRequest is the decoded payload of a GeneralCommand message.
type GeneralRequest struct {
	Name      string
	Arguments map[string]string
}

// Command is a decoded inbound remote-control command. Exactly one of the
// typed payloads is populated according to Type.
type Command struct {
	Type      CommandType
	Play      *PlayRequest
	Playstate *PlaystateRequest
	General   *GeneralRequest
}

// Session is a live remote-control channel to the media server.
type Session interface {
	// Commands returns the stream of decoded inbound commands. It is closed
	// when the session ends (context cancelled or unrecoverable error).
	Commands() <-chan Command

	// Run maintains the connection (keep-alive, reconnect) until ctx is done.
	Run(ctx context.Context) error
}
