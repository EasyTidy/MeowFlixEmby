package playback

import (
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/remote"
)

func TestMapGeneral(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     *remote.GeneralRequest
		wantCmd player.ControlCmd
		check   func(t *testing.T, c player.Control)
	}{
		{
			name:    "SetVolume",
			req:     &remote.GeneralRequest{Name: "SetVolume", Arguments: map[string]string{"Volume": "40"}},
			wantCmd: player.CtrlSetVolume,
			check:   func(t *testing.T, c player.Control) { eq(t, c.Volume, 40) },
		},
		{
			name:    "Mute",
			req:     &remote.GeneralRequest{Name: "Mute"},
			wantCmd: player.CtrlMute,
		},
		{
			name:    "Unmute",
			req:     &remote.GeneralRequest{Name: "Unmute"},
			wantCmd: player.CtrlUnmute,
		},
		{
			name:    "SetSubtitleStreamIndex",
			req:     &remote.GeneralRequest{Name: "SetSubtitleStreamIndex", Arguments: map[string]string{"Index": "3"}},
			wantCmd: player.CtrlSetSubtitle,
			check:   func(t *testing.T, c player.Control) { eq(t, c.TrackIndex, 3) },
		},
		{
			name:    "SetSubtitle off (no index)",
			req:     &remote.GeneralRequest{Name: "SetSubtitleStreamIndex", Arguments: map[string]string{}},
			wantCmd: player.CtrlSetSubtitle,
			check:   func(t *testing.T, c player.Control) { eq(t, c.TrackIndex, -1) },
		},
		{
			name:    "SetAudioStreamIndex",
			req:     &remote.GeneralRequest{Name: "SetAudioStreamIndex", Arguments: map[string]string{"Index": "2"}},
			wantCmd: player.CtrlSetAudio,
			check:   func(t *testing.T, c player.Control) { eq(t, c.TrackIndex, 2) },
		},
		{
			name:    "DisplayMessage",
			req:     &remote.GeneralRequest{Name: "DisplayMessage", Arguments: map[string]string{"Header": "H", "Text": "T", "TimeoutMs": "2000"}},
			wantCmd: player.CtrlDisplayMsg,
			check: func(t *testing.T, c player.Control) {
				eq(t, c.Header, "H")
				eq(t, c.Text, "T")
				eq(t, c.TimeoutMs, 2000)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := mapGeneral(tc.req)
			if !ok {
				t.Fatal("mapGeneral returned ok=false")
			}
			if c.Cmd != tc.wantCmd {
				t.Fatalf("Cmd = %d, want %d", c.Cmd, tc.wantCmd)
			}
			if tc.check != nil {
				tc.check(t, c)
			}
		})
	}

	if _, ok := mapGeneral(&remote.GeneralRequest{Name: "Unknown"}); ok {
		t.Error("unknown command should map to ok=false")
	}
}

// eq is a tiny generic helper for scalar comparisons.
func eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
