package player

import "strings"

// MapRegistry is a simple name→Player registry satisfying Registry. It also
// implements the default + by-path selection policy from configuration.
type MapRegistry struct {
	players map[string]Player
	def     string
	byPath  []PathRule
}

// PathRule selects a player when the media path contains any of Match (case-
// insensitive). The first matching rule wins.
type PathRule struct {
	Player string
	Match  []string
}

// NewRegistry builds a registry from the given players, a default player name,
// and optional path rules.
func NewRegistry(players []Player, def string, byPath []PathRule) *MapRegistry {
	m := &MapRegistry{players: make(map[string]Player, len(players)), def: def, byPath: byPath}
	for _, p := range players {
		if p != nil {
			m.players[p.Name()] = p
		}
	}
	return m
}

// Get returns the player registered under name.
func (m *MapRegistry) Get(name string) (Player, bool) {
	p, ok := m.players[name]
	return p, ok
}

// Select chooses a player for a media path: the first by-path rule that matches
// wins, otherwise the default. It falls back to any registered player only if
// the chosen name is missing, returning ok=false when nothing is available.
func (m *MapRegistry) Select(mediaPath string) (Player, bool) {
	lower := strings.ToLower(mediaPath)
	for _, rule := range m.byPath {
		for _, kw := range rule.Match {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" && strings.Contains(lower, kw) {
				if p, ok := m.players[rule.Player]; ok {
					return p, true
				}
			}
		}
	}
	if p, ok := m.players[m.def]; ok {
		return p, true
	}
	return nil, false
}

// Ensure MapRegistry satisfies Registry.
var _ Registry = (*MapRegistry)(nil)
