// Package playback is the M5 orchestration layer: it consumes remote-control
// commands, resolves each item to a playable target, drives the local player,
// reports progress back to the server, and chains next-episode playback.
package playback

import (
	"strconv"
	"strings"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
)

// ticksPerSec converts Emby 100ns ticks to seconds and back.
const ticksPerSec = 10_000_000

func ticksToSec(t int64) float64 { return float64(t) / ticksPerSec }
func secToTicks(s float64) int64 { return int64(s * ticksPerSec) }

// relativeSeekSec is the jump applied for FastForward / Rewind commands.
const relativeSeekSec = 30.0

// mapGeneral translates a GeneralCommand into a player.Control. It returns
// ok=false for commands with no player mapping (logged and ignored).
func mapGeneral(req *remote.GeneralRequest) (player.Control, bool) {
	switch req.Name {
	case "SetVolume":
		return player.Control{Cmd: player.CtrlSetVolume, Volume: atoiDefault(req.Arguments["Volume"], 100)}, true
	case "Mute":
		return player.Control{Cmd: player.CtrlMute}, true
	case "Unmute":
		return player.Control{Cmd: player.CtrlUnmute}, true
	case "ToggleMute":
		return player.Control{Cmd: player.CtrlToggleMute}, true
	case "SetSubtitleStreamIndex":
		return player.Control{Cmd: player.CtrlSetSubtitle, TrackIndex: atoiDefault(req.Arguments["Index"], -1)}, true
	case "SetAudioStreamIndex":
		return player.Control{Cmd: player.CtrlSetAudio, TrackIndex: atoiDefault(req.Arguments["Index"], 0)}, true
	case "DisplayMessage":
		return player.Control{
			Cmd:       player.CtrlDisplayMsg,
			Header:    req.Arguments["Header"],
			Text:      req.Arguments["Text"],
			TimeoutMs: atoiDefault(req.Arguments["TimeoutMs"], 0),
		}, true
	default:
		return player.Control{}, false
	}
}

// atoiDefault parses s as an int, returning def on any error.
func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

// mapPlaystate translates an in-player Playstate command into a player.Control.
// It returns ok=false for commands handled at the session level (Stop,
// NextTrack, PreviousTrack) which affect the queue rather than the player.
func mapPlaystate(req *remote.PlaystateRequest) (player.Control, bool) {
	switch req.Command {
	case remote.StatePause:
		return player.Control{Cmd: player.CtrlPause}, true
	case remote.StateUnpause:
		return player.Control{Cmd: player.CtrlUnpause}, true
	case remote.StatePlayPause:
		return player.Control{Cmd: player.CtrlPlayPause}, true
	case remote.StateSeek:
		return player.Control{Cmd: player.CtrlSeekAbsolute, SeekSec: ticksToSec(req.SeekPositionTicks)}, true
	case remote.StateFastForward:
		return player.Control{Cmd: player.CtrlSeekRelative, SeekSec: relativeSeekSec}, true
	case remote.StateRewind:
		return player.Control{Cmd: player.CtrlSeekRelative, SeekSec: -relativeSeekSec}, true
	case remote.StateSeekRelative:
		return player.Control{Cmd: player.CtrlSeekRelative, SeekSec: ticksToSec(req.SeekPositionTicks)}, true
	default:
		return player.Control{}, false
	}
}
