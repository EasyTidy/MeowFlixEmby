// Package player defines the pluggable local-player abstraction. Each supported
// player (mpv, VLC, PotPlayer, MPC, or a generic fallback) implements Player.
package player

import "context"

// ControlCmd is a transport-control instruction forwarded to a running player.
type ControlCmd int

const (
	CtrlPause ControlCmd = iota
	CtrlUnpause
	CtrlPlayPause
	CtrlStop
	CtrlNextTrack
	CtrlPreviousTrack
	CtrlSeekAbsolute // uses SeekSec
	CtrlSeekRelative // uses SeekSec (delta)
	CtrlSetVolume    // uses Volume (0-100)
	CtrlMute
	CtrlUnmute
	CtrlToggleMute
	CtrlSetSubtitle // uses TrackIndex (server subtitle stream index; -1 = off)
	CtrlSetAudio    // uses TrackIndex (server audio stream index)
	CtrlDisplayMsg  // uses Header/Text/TimeoutMs
)

// Control bundles a command with its optional arguments. Only the field(s)
// relevant to Cmd are populated.
type Control struct {
	Cmd        ControlCmd
	SeekSec    float64 // CtrlSeek*
	Volume     int     // CtrlSetVolume (0-100)
	TrackIndex int     // CtrlSetSubtitle / CtrlSetAudio (server stream index)
	Header     string  // CtrlDisplayMsg
	Text       string  // CtrlDisplayMsg
	TimeoutMs  int     // CtrlDisplayMsg (0 = player default)
}

// PlayRequest is what to play and how.
type PlayRequest struct {
	// MediaPath is a local filesystem path or an absolute stream/direct URL.
	MediaPath string
	// MountDisk is true when MediaPath is a local file (DirectPlay), false for HTTP.
	MountDisk bool
	StartSec  float64
	SubFile   string // absolute subtitle URL or local path, optional
	SubIndex  int    // internal subtitle track index, 0 = none/auto
	Title     string
	// Playlist holds the up-next queue (for players with native playlists).
	Playlist []QueueItem
}

// QueueItem is one entry in the up-next queue.
type QueueItem struct {
	MediaPath string
	Title     string
	SubFile   string
}

// Player launches a media file in a specific external player.
type Player interface {
	// Name is the player's registry key (e.g. "mpv").
	Name() string
	// Start launches playback and returns a Handle to observe/control it.
	Start(ctx context.Context, req PlayRequest) (Handle, error)
}

// Handle observes and controls a running player instance.
type Handle interface {
	// Progress returns the current position and duration in seconds. ok is
	// false if the player does not expose progress (generic fallback).
	Progress() (posSec, durSec float64, ok bool)
	// Control forwards a transport command; returns an error if unsupported.
	Control(c Control) error
	// Wait blocks until the player exits and returns the final position (sec).
	Wait() (stopSec float64, err error)
}

// Registry resolves a player by name; implementations back the "by_path" and
// default selection logic.
type Registry interface {
	Get(name string) (Player, bool)
}
