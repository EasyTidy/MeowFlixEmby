//go:build windows

package mpv

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// ipcServerName returns the IPC endpoint mpv should create, and the address the
// client dials. On Windows this is a named pipe.
func ipcServerName(id string) (mpvArg, dialAddr string) {
	pipe := `\\.\pipe\mpv-` + id
	return pipe, pipe
}

// dialIPC connects to an mpv named pipe.
func dialIPC(addr string) (net.Conn, error) {
	t := 2 * time.Second
	return winio.DialPipe(addr, &t)
}
