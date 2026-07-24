package resolver

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// TranslatePath converts a server-side path to a local path using the first
// matching prefix in maps. Separators are normalised to the local OS. When
// pathCheck is true it verifies the file exists (trying NFC/NFD forms) and
// falls back to the raw translation if none is found.
//
// If no prefix matches, the original path is returned unchanged.
func TranslatePath(serverPath string, maps []PathMap, pathCheck bool) string {
	if strings.HasPrefix(serverPath, "http://") || strings.HasPrefix(serverPath, "https://") {
		return serverPath
	}
	for _, m := range maps {
		if m.Src == "" || !strings.HasPrefix(serverPath, m.Src) {
			continue
		}
		rest := strings.TrimPrefix(serverPath, m.Src)
		joined := m.Dst + rest
		local := normalizeSeparators(joined)
		if !pathCheck {
			return local
		}
		if resolved, ok := existingForm(local); ok {
			return resolved
		}
		// Not found; return the raw translation so caller can decide/fallback.
		return local
	}
	return serverPath
}

// normalizeSeparators rewrites path separators to match the local OS and
// cleans redundant separators.
func normalizeSeparators(p string) string {
	if filepath.Separator == '\\' {
		p = strings.ReplaceAll(p, "/", "\\")
	} else {
		p = strings.ReplaceAll(p, "\\", "/")
	}
	return filepath.Clean(p)
}

// existingForm returns the first of the NFC/NFD normalised forms of p that
// exists on disk.
func existingForm(p string) (string, bool) {
	forms := []string{norm.NFC.String(p), norm.NFD.String(p)}
	seen := map[string]bool{}
	for _, f := range forms {
		if seen[f] {
			continue
		}
		seen[f] = true
		if _, err := os.Stat(f); err == nil {
			return f, true
		}
	}
	return "", false
}

// hasAnyPrefix reports whether s starts with any of the given prefixes
// (empty prefixes are ignored).
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
