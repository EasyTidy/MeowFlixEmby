//go:build !windows

package main

import "fmt"

// runService is a no-op on non-Windows platforms. The Windows Service Control
// Manager only exists on Windows; elsewhere the daemon runs in the foreground
// (or under systemd/launchd via the provided unit files).
func runService(_, action string) (handled bool, err error) {
	if action != "" {
		return true, fmt.Errorf("-service is only supported on Windows")
	}
	return false, nil
}
