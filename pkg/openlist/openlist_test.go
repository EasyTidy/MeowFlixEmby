package openlist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRawURL(t *testing.T) {
	t.Parallel()
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var req struct{ Path string }
		_ = json.Unmarshal(body, &req)
		gotPath = req.Path
		_, _ = io.WriteString(w, `{"code":200,"message":"success","data":{"name":"x.mkv","size":123,"raw_url":"https://cloud.example/x.mkv?sign=abc"}}`)
	}))
	defer srv.Close()

	c := New(Options{Host: srv.URL, Token: "TKN", HTTPClient: srv.Client()})
	url, err := c.RawURL(context.Background(), "/baidu/movie/x.mkv")
	if err != nil {
		t.Fatalf("RawURL: %v", err)
	}
	if url != "https://cloud.example/x.mkv?sign=abc" {
		t.Fatalf("url = %q", url)
	}
	if gotAuth != "TKN" {
		t.Errorf("auth header = %q, want TKN", gotAuth)
	}
	if gotPath != "/baidu/movie/x.mkv" {
		t.Errorf("path sent = %q", gotPath)
	}
}

func TestRawURLErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"code non-200", `{"code":500,"message":"OpenAPI仅限购买","data":null}`},
		{"empty raw_url", `{"code":200,"message":"success","data":{"raw_url":""}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			c := New(Options{Host: srv.URL, Token: "T", HTTPClient: srv.Client()})
			if _, err := c.RawURL(context.Background(), "/x"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestRawURLHTTPError covers a non-2xx status.
func TestRawURLHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "unauthorized")
	}))
	defer srv.Close()
	c := New(Options{Host: srv.URL, Token: "T", HTTPClient: srv.Client()})
	_, err := c.RawURL(context.Background(), "/x")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 error, got %v", err)
	}
}
