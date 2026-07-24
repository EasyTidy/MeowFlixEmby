package resolver

import (
	"context"
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// fakeServer provides deterministic URLs for HTTP-stream decisions.
type fakeServer struct{}

func (fakeServer) Authenticate(context.Context) error                                  { return nil }
func (fakeServer) AnnounceCapabilities(context.Context, mediaserver.Capabilities) error { return nil }
func (fakeServer) ResolveItem(context.Context, string) (*mediaserver.MediaItem, error)  { return nil, nil }
func (fakeServer) ReportStart(context.Context, mediaserver.PlaybackState) error         { return nil }
func (fakeServer) ReportProgress(context.Context, mediaserver.PlaybackState) error      { return nil }
func (fakeServer) ReportStopped(context.Context, mediaserver.PlaybackState) error       { return nil }

func (fakeServer) StreamURL(item *mediaserver.MediaItem, s *mediaserver.MediaSource) string {
	return "https://srv/videos/" + item.ID + "/stream?MediaSourceId=" + s.ID
}

func (fakeServer) SubtitleURL(item *mediaserver.MediaItem, s *mediaserver.MediaSource, st *mediaserver.MediaStream) string {
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
			d, err := e.Resolve(item(tc.src), fakeServer{}, tc.cfg)
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

func TestResolve_Errors(t *testing.T) {
	e := New()
	if _, err := e.Resolve(nil, fakeServer{}, Config{}); err == nil {
		t.Error("expected error for nil item")
	}
	if _, err := e.Resolve(&mediaserver.MediaItem{ID: "x"}, fakeServer{}, Config{}); err == nil {
		t.Error("expected error for item with no sources")
	}
}
