package resolver

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// Engine is the default Resolver implementation.
type Engine struct{}

// New returns a ready-to-use resolver Engine.
func New() *Engine { return &Engine{} }

// Resolve selects the best playback method following the priority order:
//  1. force_disk_prefixes hit           -> DirectDisk
//  2. http/strm source & direct_url host -> DirectURL (cloud direct)
//  3. path_maps translate to local path  -> DirectDisk (mounted NAS)
//  4. otherwise                          -> HTTPStream via server
//
// See docs/03-架构设计.md §3 for the full table.
func (e *Engine) Resolve(item *mediaserver.MediaItem, srv mediaserver.Server, cfg Config) (Decision, error) {
	if item == nil {
		return Decision{}, fmt.Errorf("resolve: nil item")
	}
	src := SelectSource(item.Sources, cfg.VersionPrefer)
	if src == nil {
		return Decision{}, fmt.Errorf("resolve: item %q has no media sources", item.ID)
	}

	isHTTPSource := strings.HasPrefix(src.Path, "http://") || strings.HasPrefix(src.Path, "https://")

	d := Decision{Source: src}

	switch {
	// 1. Forced local disk by server-path prefix.
	case !isHTTPSource && hasAnyPrefix(src.Path, cfg.ForceDiskPrefixes):
		d.Method = MethodDirectDisk
		d.MediaPath = TranslatePath(src.Path, cfg.PathMaps, cfg.PathCheck)
		d.MountDisk = true
		d.PlayMethod = "DirectPlay"

	// 2. Cloud/http source that the player may hit directly.
	case isHTTPSource && hostMatches(src.Path, cfg.DirectURLHosts):
		d.Method = MethodDirectURL
		d.MediaPath = src.Path
		d.MountDisk = false
		d.PlayMethod = "DirectStream"

	// 3. Mounted NAS: server path translates to an existing local prefix.
	case !isHTTPSource && translatable(src.Path, cfg.PathMaps):
		d.Method = MethodDirectDisk
		d.MediaPath = TranslatePath(src.Path, cfg.PathMaps, cfg.PathCheck)
		d.MountDisk = true
		d.PlayMethod = "DirectPlay"

	// 4. Fallback: stream over HTTP from the server.
	default:
		d.Method = MethodHTTPStream
		d.MediaPath = srv.StreamURL(item, src)
		d.MountDisk = false
		d.PlayMethod = "DirectStream"
		if src.TranscodingURL != "" && src.DirectStreamURL == "" {
			d.PlayMethod = "Transcode"
		}
	}

	// Subtitle selection: external subtitles only make sense for HTTP playback.
	sub := SelectSubtitle(src.Streams, cfg.SubtitlePriority, d.MountDisk)
	d.SubIndex = sub.InnerIndex
	if sub.External != nil && !d.MountDisk {
		d.SubFile = srv.SubtitleURL(item, src, sub.External)
	}

	return d, nil
}

// translatable reports whether serverPath starts with any configured src prefix.
func translatable(serverPath string, maps []PathMap) bool {
	for _, m := range maps {
		if m.Src != "" && strings.HasPrefix(serverPath, m.Src) {
			return true
		}
	}
	return false
}

// hostMatches reports whether the URL's host (or the whole URL) contains any
// of the configured host keywords.
func hostMatches(rawURL string, hosts []string) bool {
	if len(hosts) == 0 {
		return false
	}
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		host = u.Host
	}
	host = strings.ToLower(host)
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" && strings.Contains(host, h) {
			return true
		}
	}
	return false
}
