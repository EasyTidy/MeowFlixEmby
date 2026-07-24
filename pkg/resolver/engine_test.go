package resolver

import (
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// fakeURLs provides deterministic URLs for HTTP-stream decisions. It satisfies
// the narrow mediaserver.URLBuilder the resolver now depends on.
type fakeURLs struct{}

func (fakeURLs) StreamURL(item *mediaserver.MediaItem, s *mediaserver.MediaSource) string {
	return "https://srv/videos/" + item.ID + "/stream?MediaSourceId=" + s.ID
}

func (fakeURLs) SubtitleURL(item *mediaserver.MediaItem, s *mediaserver.MediaSource, st *mediaserver.MediaStream) string {
	return "https://srv/sub/" + item.ID
}

func item(src mediaserver.MediaSource) *mediaserver.MediaItem {
	return &mediaserver.MediaItem{ID: "it1", Sources: []mediaserver.MediaSource{src}}
}

func TestResolve_Methods(t *testing.T) {
	tests := []struct {
		name       string
		src        mediaserver.MediaSource
		cfg        Config
		wantMethod Method
		wantPath   string
		wantMount  bool
		wantPlay   string
	}{
		{
			name:       "force disk prefix wins",
			src:        mediaserver.MediaSource{ID: "s1", Path: "/mnt/disk1/m/a.mkv"},
			cfg:        Config{ForceDiskPrefixes: []string{"/mnt/disk1"}, PathMaps: []PathMap{{Src: "/mnt/disk1", Dst: "/local"}}},
			wantMethod: MethodDirectDisk,
			wantPath:   "/local/m/a.mkv",
			wantMount:  true,
			wantPlay:   "DirectPlay",
		},
		{
			name:       "cloud http source with direct host",
			src:        mediaserver.MediaSource{ID: "s2", Path: "https://pan.cloud.com/x/a.mkv", Protocol: mediaserver.ProtocolHTTP},
			cfg:        Config{DirectURLHosts: []string{"pan.cloud.com"}},
			wantMethod: MethodDirectURL,
			wantPath:   "https://pan.cloud.com/x/a.mkv",
			wantMount:  false,
			wantPlay:   "DirectStream",
		},
		{
			name:       "mounted nas via path map",
			src:        mediaserver.MediaSource{ID: "s3", Path: "/mnt/disk2/media/b.mkv"},
			cfg:        Config{PathMaps: []PathMap{{Src: "/mnt/disk2/media", Dst: "/f/media"}}},
			wantMethod: MethodDirectDisk,
			wantPath:   "/f/media/b.mkv",
			wantMount:  true,
			wantPlay:   "DirectPlay",
		},
		{
			name:       "fallback http stream",
			src:        mediaserver.MediaSource{ID: "s4", Path: "/mnt/other/c.mkv", DirectStreamURL: "/videos/it1/stream"},
			cfg:        Config{},
			wantMethod: MethodHTTPStream,
			wantPath:   "https://srv/videos/it1/stream?MediaSourceId=s4",
			wantMount:  false,
			wantPlay:   "DirectStream",
		},
		{
			name:       "fallback forces transcode when only transcoding url",
			src:        mediaserver.MediaSource{ID: "s5", Path: "/mnt/other/d.iso", TranscodingURL: "/videos/it1/master.m3u8"},
			cfg:        Config{},
			wantMethod: MethodHTTPStream,
			wantPath:   "https://srv/videos/it1/stream?MediaSourceId=s5",
			wantMount:  false,
			wantPlay:   "Transcode",
		},
		{
			name:       "http source without direct host falls back to stream",
			src:        mediaserver.MediaSource{ID: "s6", Path: "https://pan.cloud.com/x/e.mkv", Protocol: mediaserver.ProtocolHTTP},
			cfg:        Config{DirectURLHosts: []string{"other.host"}},
			wantMethod: MethodHTTPStream,
			wantPath:   "https://srv/videos/it1/stream?MediaSourceId=s6",
			wantMount:  false,
			wantPlay:   "DirectStream",
		},
	}

	e := New()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := e.Resolve(item(tc.src), fakeURLs{}, tc.cfg)
			if err != nil {
				t.Fatalf("Resolve error: %v", err)
			}
			if d.Method != tc.wantMethod {
				t.Errorf("Method = %v, want %v", d.Method, tc.wantMethod)
			}
			// Local disk paths use the OS separator; normalise before comparing.
			if normalizeSeparators(d.MediaPath) != normalizeSeparators(tc.wantPath) {
				t.Errorf("MediaPath = %q, want %q", d.MediaPath, tc.wantPath)
			}
			if d.MountDisk != tc.wantMount {
				t.Errorf("MountDisk = %v, want %v", d.MountDisk, tc.wantMount)
			}
			if d.PlayMethod != tc.wantPlay {
				t.Errorf("PlayMethod = %q, want %q", d.PlayMethod, tc.wantPlay)
			}
		})
	}
}

func TestResolve_Openlist(t *testing.T) {
	e := New()
	cfg := Config{
		OpenlistEnabled:  true,
		OpenlistPathMaps: []PathMap{{Src: "/volume1/video", Dst: ""}},
	}
	// A server path with no verified local mount routes to openlist with the
	// prefix stripped.
	src := mediaserver.MediaSource{ID: "s", Path: "/volume1/video/123Pan/电影/x/movie.mp4"}
	d, err := e.Resolve(item(src), fakeURLs{}, cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if d.Method != MethodOpenlist {
		t.Fatalf("method = %v, want Openlist", d.Method)
	}
	if d.OpenlistPath != "/123Pan/电影/x/movie.mp4" {
		t.Fatalf("openlist path = %q", d.OpenlistPath)
	}
	if d.MediaPath != "" {
		t.Fatalf("MediaPath should be empty until caller resolves, got %q", d.MediaPath)
	}

	// With Dst prefix.
	cfg2 := Config{OpenlistEnabled: true, OpenlistPathMaps: []PathMap{{Src: "/电视剧", Dst: "/电视剧"}}}
	d2, _ := e.Resolve(item(mediaserver.MediaSource{ID: "s2", Path: "/电视剧/show/ep1.mkv"}), fakeURLs{}, cfg2)
	if d2.Method != MethodOpenlist || d2.OpenlistPath != "/电视剧/show/ep1.mkv" {
		t.Fatalf("mapped = %v %q", d2.Method, d2.OpenlistPath)
	}

	// Openlist disabled -> falls through to HTTP stream (no local maps).
	d3, _ := e.Resolve(item(src), fakeURLs{}, Config{})
	if d3.Method != MethodHTTPStream {
		t.Fatalf("disabled openlist should fall to HTTPStream, got %v", d3.Method)
	}
}

func TestMapPath(t *testing.T) {
	t.Parallel()
	maps := []PathMap{{Src: "/volume1/video", Dst: ""}, {Src: "/A", Dst: "/mnt/A"}}
	tests := []struct{ in, want string }{
		{"/volume1/video/123Pan/电影/a.mp4", "/123Pan/电影/a.mp4"},
		{"/A/电影/b.mkv", "/mnt/A/电影/b.mkv"},
		{"/other/c.mkv", "/other/c.mkv"}, // no match -> unchanged
	}
	for _, tc := range tests {
		if got := MapPath(tc.in, maps); got != tc.want {
			t.Errorf("MapPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolve_Errors(t *testing.T) {
	e := New()
	if _, err := e.Resolve(nil, fakeURLs{}, Config{}); err == nil {
		t.Error("expected error for nil item")
	}
	if _, err := e.Resolve(&mediaserver.MediaItem{ID: "x"}, fakeURLs{}, Config{}); err == nil {
		t.Error("expected error for item with no sources")
	}
}
