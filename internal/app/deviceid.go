package app

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// deviceIDFileName is where the stable device id is persisted alongside config.
const deviceIDFileName = ".meowflix-device-id"

// stableDeviceID returns a persistent device identifier, creating and storing
// one on first run. The same id must be used for auth, capabilities and the
// WebSocket, and must survive restarts so the server recognises the session.
//
// dir is the directory to persist into (typically the config file's dir); an
// empty or unwritable dir falls back to an in-memory id for this run.
func stableDeviceID(dir string) string {
	path := filepath.Join(dir, deviceIDFileName)
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}
	id := newGUID()
	// Best-effort persist; a failure just means we regenerate next run.
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}

// newGUID returns a random 32-hex-char identifier (128 bits).
func newGUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; degrade to a fixed-length marker.
		return "meowflixfallbackdeviceid00000000"
	}
	return hex.EncodeToString(b[:])
}
