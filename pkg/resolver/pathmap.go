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

// MapPath rewrites serverPath using the first matching prefix in maps by
// stripping Src and prepending Dst, keeping forward slashes (openlist paths use
// "/"). Unlike TranslatePath it does no OS-separator normalisation or filesystem
// check. If no prefix matches, serverPath is returned unchanged.
func MapPath(serverPath string, maps []PathMap) string {
	for _, m := range maps {
		if m.Src == "" || !strings.HasPrefix(serverPath, m.Src) {
			continue
		}
		rest := strings.TrimPrefix(serverPath, m.Src)
		joined := strings.TrimRight(m.Dst, "/") + rest
		if !strings.HasPrefix(joined, "/") {
			joined = "/" + joined
		}
		return strings.ReplaceAll(joined, "\\", "/")
	}
	return serverPath
}

// translatableExisting reports whether serverPath maps to a local file that
// actually exists on disk (trying NFC/NFD forms), used to prefer a verified
// local mount over cloud strategies.
func translatableExisting(serverPath string, maps []PathMap) bool {
	if !translatable(serverPath, maps) {
		return false
	}
	local := normalizeSeparators(translateRaw(serverPath, maps))
	_, ok := existingForm(local)
	return ok
}

// translateRaw applies the first matching prefix map without a filesystem check.
func translateRaw(serverPath string, maps []PathMap) string {
	for _, m := range maps {
		if m.Src != "" && strings.HasPrefix(serverPath, m.Src) {
			return m.Dst + strings.TrimPrefix(serverPath, m.Src)
		}
	}
	return serverPath
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
