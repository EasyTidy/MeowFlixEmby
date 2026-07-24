package emby

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(Options{
		Address:    srv.URL,
		Username:   "alice",
		Password:   "secret",
		DeviceName: "MeowFlix (Test)",
		DeviceID:   "dev-guid-123",
		HTTPClient: srv.Client(),
	})
	return c, srv
}

func TestAuthenticate(t *testing.T) {
	var gotAuthHeader, gotBody string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/AuthenticateByName" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		gotAuthHeader = r.Header.Get("X-Emby-Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(authResult{AccessToken: "tok-xyz", User: struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		}{ID: "user-1", Name: "alice"}})
	})

	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if c.Token() != "tok-xyz" {
		t.Errorf("token = %q, want tok-xyz", c.Token())
	}
	if c.userID != "user-1" {
		t.Errorf("userID = %q, want user-1", c.userID)
	}
	if !strings.HasPrefix(gotAuthHeader, `Emby UserId=""`) {
		t.Errorf("auth header scheme wrong: %q", gotAuthHeader)
	}
	if !strings.Contains(gotAuthHeader, `DeviceId="dev-guid-123"`) {
		t.Errorf("auth header missing device id: %q", gotAuthHeader)
	}
	if !strings.Contains(gotBody, `"Username":"alice"`) || !strings.Contains(gotBody, `"Pw":"secret"`) {
		t.Errorf("auth body wrong: %q", gotBody)
	}
}

func TestJellyfinAuthScheme(t *testing.T) {
	c := New(Options{Address: "http://x", Flavor: FlavorJellyfin, DeviceID: "d"})
	if !strings.HasPrefix(c.authHeader(), "MediaBrowser ") {
		t.Errorf("jellyfin scheme wrong: %q", c.authHeader())
	}
}

func TestAnnounceCapabilities(t *testing.T) {
	var body clientCapabilities
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Sessions/Capabilities/Full" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(204)
	})
	c.token = "tok"
	err := c.AnnounceCapabilities(context.Background(), mediaserver.Capabilities{
		PlayableMediaTypes:   []string{"Video", "Audio"},
		SupportedCommands:    []string{"PlayPause", "Seek"},
		SupportsMediaControl: true,
	})
	if err != nil {
		t.Fatalf("AnnounceCapabilities: %v", err)
	}
	if !body.SupportsMediaControl {
		t.Error("SupportsMediaControl not sent as true")
	}
	if len(body.SupportedCommands) != 2 {
		t.Errorf("SupportedCommands = %v", body.SupportedCommands)
	}
}

func TestResolveItemAndStreamURL(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/Items/it9/PlaybackInfo") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(playbackInfoResponse{
			PlaySessionID: "psid-1",
			MediaSources: []mediaSourceInfo{{
				ID: "ms1", Path: `\\Tower\Movies\a.mkv`, Protocol: "File", Container: "mkv",
				RunTimeTicks: 10460000000,
				MediaStreams: []mediaStreamInfo{{Type: "Subtitle", Index: 3, Codec: "srt", DisplayTitle: "中文", IsExternal: true}},
			}},
		})
	})
	c.token = "tok"
	item, err := c.ResolveItem(context.Background(), "it9")
	if err != nil {
		t.Fatalf("ResolveItem: %v", err)
	}
	if item.PlaySessionID != "psid-1" || len(item.Sources) != 1 {
		t.Fatalf("unexpected item %+v", item)
	}
	src := &item.Sources[0]
	if src.Protocol != mediaserver.ProtocolFile || src.RunTimeTicks != 10460000000 {
		t.Errorf("source mapping wrong: %+v", src)
	}
	got := c.StreamURL(item, src)
	for _, want := range []string{"/Videos/it9/stream.mkv", "Static=true", "MediaSourceId=ms1", "PlaySessionId=psid-1", "api_key=tok"} {
		if !strings.Contains(got, want) {
			t.Errorf("StreamURL %q missing %q", got, want)
		}
	}
	sub := c.SubtitleURL(item, src, &src.Streams[0])
	if !strings.Contains(sub, "/Videos/it9/ms1/Subtitles/3/0/Stream.srt") {
		t.Errorf("SubtitleURL wrong: %q", sub)
	}
}

func TestReportProgressPayload(t *testing.T) {
	var path string
	var rep playbackReport
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&rep)
		w.WriteHeader(204)
	})
	c.token = "tok"
	err := c.ReportProgress(context.Background(), mediaserver.PlaybackState{
		ItemID: "it9", PlaySessionID: "psid-1", MediaSourceID: "ms1",
		PositionTicks: 10460000000, PlayMethod: "DirectStream", CanSeek: true,
	})
	if err != nil {
		t.Fatalf("ReportProgress: %v", err)
	}
	if path != "/Sessions/Playing/Progress" {
		t.Errorf("path = %q", path)
	}
	if rep.PositionTicks != 10460000000 || rep.PlayMethod != "DirectStream" {
		t.Errorf("payload wrong: %+v", rep)
	}
}

func TestErrorStatusPropagates(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad creds", http.StatusUnauthorized)
	})
	if err := c.Authenticate(context.Background()); err == nil {
		t.Error("expected error on 401")
	}
}
