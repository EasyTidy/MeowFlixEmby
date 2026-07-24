package resolver

import (
	"strings"

	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
)

// SubtitleChoice is the resolved subtitle selection.
type SubtitleChoice struct {
	// External is the chosen external subtitle stream, or nil.
	External *mediaserver.MediaStream
	// InnerIndex is the 1-based index among internal subtitle tracks to force
	// (mpv --sid). 0 means none / let the player decide.
	InnerIndex int
}

// SelectSubtitle chooses a subtitle track for a source.
//
// When mountDisk is true, external subtitles are left to the player (returns
// none) and only an internal preference is computed. Otherwise external
// subtitles are matched against the priority keywords (in order), and if none
// match, the highest-priority internal track index is returned for mpv.
//
// This mirrors embyToLocalPlayer's subtitle_checker for the common
// "unspecified" case (sub_index == -1).
func SelectSubtitle(streams []mediaserver.MediaStream, priority []string, mountDisk bool) SubtitleChoice {
	var external, internal []mediaserver.MediaStream
	for _, s := range streams {
		if s.Type != "Subtitle" {
			continue
		}
		if s.IsExternal {
			external = append(external, s)
		} else {
			internal = append(internal, s)
		}
	}

	if !mountDisk {
		if pick := bestByPriority(external, priority); pick != nil {
			return SubtitleChoice{External: pick}
		}
	}

	// No external match (or mount-disk mode): prefer an internal track by keyword.
	if pick := bestByPriority(internal, priority); pick != nil {
		for i := range internal {
			if internal[i].Index == pick.Index {
				return SubtitleChoice{InnerIndex: i + 1}
			}
		}
	}
	return SubtitleChoice{}
}

// bestByPriority returns the stream whose title/displayTitle contains the
// earliest-listed priority keyword. Returns nil if none match.
func bestByPriority(streams []mediaserver.MediaStream, priority []string) *mediaserver.MediaStream {
	bestRank := -1
	var best *mediaserver.MediaStream
	for i := range streams {
		hay := strings.ToLower(streams[i].Title + "," + streams[i].DisplayTitle)
		for rank, kw := range priority {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw == "" {
				continue
			}
			if strings.Contains(hay, kw) {
				if bestRank == -1 || rank < bestRank {
					bestRank = rank
					best = &streams[i]
				}
				break
			}
		}
	}
	return best
}
