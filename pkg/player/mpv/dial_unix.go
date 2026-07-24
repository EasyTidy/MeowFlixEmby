//go:build !windows

package mpv

import (
	"net"
	"os"
	"path/filepath"
)

// ipcServerName returns the IPC endpoint mpv should create, and the address the
// client dials. On unix this is a filesystem socket in the temp dir.
func ipcServerName(id string) (mpvArg, dialAddr string) {
	sock := filepath.Join(os.TempDir(), "mpv-"+id+".sock")
	return sock, sock
}

// dialIPC connects to an mpv unix socket.
func dialIPC(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}
