// Package resolver decides the best playback method for a media source: direct
// local disk (mounted NAS), direct cloud/URL (strm/http), or HTTP stream via
// the server. It is the core, backend-agnostic value of MeowFlixEmby.
package resolver

import "github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"

// Method is the chosen playback strategy.
type Method int

const (
	// MethodDirectDisk plays a local filesystem path (NAS already mounted).
	MethodDirectDisk Method = iota
	// MethodDirectURL plays a cloud/http URL directly (strm/http source).
	MethodDirectURL
	// MethodOpenlist resolves the path through openlist to a cloud-direct raw
	// URL. Decision.OpenlistPath holds the mapped openlist path; the caller
	// (session) performs the API call and fills MediaPath.
	MethodOpenlist
	// MethodHTTPStream streams via the server's direct-stream URL.
	MethodHTTPStream
)

// String renders the method for logs.
func (m Method) String() string {
	switch m {
	case MethodDirectDisk:
		return "DirectDisk"
	case MethodDirectURL:
		return "DirectURL"
	case MethodOpenlist:
		return "Openlist"
	case MethodHTTPStream:
		return "HTTPStream"
	default:
		return "Unknown"
	}
}

// Config carries the local settings that influence the decision.
type Config struct {
	PathMaps          []PathMap
	ForceDiskPrefixes []string
	DirectURLHosts    []string
	PathCheck         bool
	VersionPrefer     []string
	SubtitlePriority  []string

	// OpenlistEnabled turns on the openlist direct-cloud strategy: a server
	// path not present on a local mount is mapped via OpenlistPathMaps and
	// resolved to a cloud-direct URL by the caller.
	OpenlistEnabled  bool
	OpenlistPathMaps []PathMap
}

// PathMap maps a server-side prefix to a local prefix.
type PathMap struct {
	Src string
	Dst string
}

// Decision is the resolver output.
type Decision struct {
	Method     Method
	Source     *mediaserver.MediaSource
	MediaPath  string // local path or absolute URL for the player
	PlayMethod string // DirectPlay | DirectStream | Transcode (for progress reports)
	MountDisk  bool
	SubFile    string
	SubIndex   int
	// OpenlistPath is the mapped openlist file path when Method is
	// MethodOpenlist; the caller resolves it to a raw URL and sets MediaPath.
	OpenlistPath string
}

// Resolver selects a playback method and target for a media item. It depends
// only on mediaserver.URLBuilder (StreamURL/SubtitleURL) — not the full Server
// contract — so the decision logic stays decoupled from auth/session concerns.
type Resolver interface {
	Resolve(item *mediaserver.MediaItem, urls mediaserver.URLBuilder, cfg Config) (Decision, error)
}
