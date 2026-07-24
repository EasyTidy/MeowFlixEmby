package resolver

import (
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

func TestSelectSource(t *testing.T) {
	sources := []mediaserver.MediaSource{
		{ID: "a", Path: "/m/Show.720p.WEB-DL.mkv"},
		{ID: "b", Path: "/m/Show.2160p.Remux.mkv"},
		{ID: "c", Path: "/m/Show.1080p.mkv"},
	}
	tests := []struct {
		name   string
		prefer []string
		wantID string
	}{
		{"prefer remux", []string{"remux", "1080"}, "b"},
		{"prefer 1080 first", []string{"1080", "remux"}, "c"},
		{"no prefer -> first", nil, "a"},
		{"no match -> first", []string{"xyz"}, "a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SelectSource(sources, tc.prefer)
			if got == nil || got.ID != tc.wantID {
				t.Errorf("got %v, want %s", got, tc.wantID)
			}
		})
	}

	if SelectSource(nil, nil) != nil {
		t.Error("empty sources should return nil")
	}
}

func TestSelectSource_HTTPUsesName(t *testing.T) {
	sources := []mediaserver.MediaSource{
		{ID: "a", Path: "https://pan/x", Name: "720p version"},
		{ID: "b", Path: "https://pan/y", Name: "2160p remux"},
	}
	if got := SelectSource(sources, []string{"remux"}); got == nil || got.ID != "b" {
		t.Errorf("http source select by name failed: %v", got)
	}
}

func TestTranslatePath(t *testing.T) {
	maps := []PathMap{
		{Src: "/mnt/disk1", Dst: "/local1"},
		{Src: "/mnt/disk2/media", Dst: "/local2"},
	}
	tests := []struct {
		in   string
		want string
	}{
		{"/mnt/disk1/a/b.mkv", "/local1/a/b.mkv"},
		{"/mnt/disk2/media/c.mkv", "/local2/c.mkv"},
		{"/mnt/unknown/d.mkv", "/mnt/unknown/d.mkv"}, // no match -> unchanged
		{"https://pan/x.mkv", "https://pan/x.mkv"},   // http -> unchanged
	}
	for _, tc := range tests {
		got := TranslatePath(tc.in, maps, false)
		// Normalise slashes for cross-OS comparison of relative expectations.
		if normalizeSeparators(got) != normalizeSeparators(tc.want) {
			t.Errorf("TranslatePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSelectSubtitle(t *testing.T) {
	streams := []mediaserver.MediaStream{
		{Type: "Video", Index: 0},
		{Type: "Audio", Index: 1},
		{Type: "Subtitle", Index: 2, DisplayTitle: "English", IsExternal: false},
		{Type: "Subtitle", Index: 3, DisplayTitle: "简体中文", IsExternal: true},
		{Type: "Subtitle", Index: 4, DisplayTitle: "中英双语", IsExternal: true},
	}

	// HTTP mode: external subtitle chosen by priority.
	got := SelectSubtitle(streams, []string{"中英", "简"}, false)
	if got.External == nil || got.External.Index != 4 {
		t.Errorf("expected external index 4 (中英), got %+v", got)
	}

	// Mount-disk mode: external ignored; internal preference computed.
	got = SelectSubtitle(streams, []string{"english"}, true)
	if got.External != nil {
		t.Errorf("mount-disk should not pick external, got %+v", got.External)
	}
	if got.InnerIndex != 1 {
		t.Errorf("expected inner index 1 for English, got %d", got.InnerIndex)
	}

	// No priority match -> nothing.
	got = SelectSubtitle(streams, []string{"nomatch"}, false)
	if got.External != nil || got.InnerIndex != 0 {
		t.Errorf("expected empty choice, got %+v", got)
	}
}
