package resolver

import (
	"path"
	"strings"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// SelectSource picks the preferred media source from a multi-version item.
//
// It scans the prefer keywords in order; the first keyword found in any
// source's name/path wins, and the source containing it is returned. When no
// keyword matches (or prefer is empty), the first source is returned.
//
// This mirrors embyToLocalPlayer's version_prefer_emby: for http sources it
// matches against the source Name, otherwise against the file base name.
func SelectSource(sources []mediaserver.MediaSource, prefer []string) *mediaserver.MediaSource {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 || len(prefer) == 0 {
		return &sources[0]
	}

	useName := strings.HasPrefix(sources[0].Path, "http")
	names := make([]string, len(sources))
	for i := range sources {
		if useName {
			names[i] = strings.ToLower(sources[i].Name)
		} else {
			names[i] = strings.ToLower(path.Base(filepathToSlash(sources[i].Path)))
		}
	}

	for _, rule := range prefer {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		for i, n := range names {
			if strings.Contains(n, rule) {
				return &sources[i]
			}
		}
	}
	return &sources[0]
}

// filepathToSlash converts backslashes to slashes so path.Base works on
// Windows-style server paths regardless of the local OS.
func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
