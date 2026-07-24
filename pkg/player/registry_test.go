package player

import (
	"context"
	"testing"
)

// stubPlayer is a no-op Player for registry tests.
type stubPlayer struct{ name string }

func (s stubPlayer) Name() string                                       { return s.name }
func (s stubPlayer) Start(context.Context, PlayRequest) (Handle, error) { return nil, nil }

func TestRegistrySelect(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(
		[]Player{stubPlayer{"mpv"}, stubPlayer{"vlc"}},
		"mpv",
		[]PathRule{{Player: "vlc", Match: []string{".iso", "__bdmv"}}},
	)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"default", "/m/movie.mkv", "mpv"},
		{"by-path iso", "/m/disc.ISO", "vlc"},
		{"by-path bdmv", "/m/Title/__BDMV/index", "vlc"},
		{"no rule match -> default", "/m/x.mp4", "mpv"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := reg.Select(tc.path)
			if !ok || p.Name() != tc.want {
				t.Fatalf("Select(%q) = %v (ok=%v), want %s", tc.path, p, ok, tc.want)
			}
		})
	}

	if _, ok := reg.Get("mpv"); !ok {
		t.Error("Get(mpv) should exist")
	}
	if _, ok := reg.Get("nope"); ok {
		t.Error("Get(nope) should not exist")
	}
}

// TestRegistrySelectMissingDefault verifies ok=false when nothing is available.
func TestRegistrySelectMissingDefault(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(nil, "mpv", nil)
	if _, ok := reg.Select("/m/x.mkv"); ok {
		t.Error("empty registry should return ok=false")
	}
}
