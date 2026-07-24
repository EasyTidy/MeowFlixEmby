//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/EasyTidy/MeowFlixEmby/internal/app"
)

const serviceName = "MeowFlixEmby"
const serviceDisplay = "MeowFlixEmby Cast Daemon"
const serviceDesc = "Casts Emby/Jellyfin web playback to a local player and reports progress back."

// runService handles Windows service control actions and SCM-managed execution.
// It returns handled=true when it took over the process lifecycle (either a
// -service subcommand or running under the SCM), so main should not continue.
func runService(cfgPath, action string) (handled bool, err error) {
	// Detect launch by the Service Control Manager (no -service flag needed).
	isService, serr := svc.IsWindowsService()
	if serr != nil {
		return false, fmt.Errorf("detect service session: %w", serr)
	}
	if isService {
		return true, runUnderSCM(cfgPath)
	}

	switch action {
	case "":
		return false, nil // foreground console mode
	case "install":
		return true, installService(cfgPath)
	case "uninstall", "remove":
		return true, controlService(action)
	case "start", "stop", "status":
		return true, controlService(action)
	case "run":
		// Force running under SCM machinery (used by the service binary itself).
		return true, runUnderSCM(cfgPath)
	default:
		return true, fmt.Errorf("unknown -service action %q (install|uninstall|start|stop|status)", action)
	}
}

// meowflixService adapts the app to the svc.Handler interface.
type meowflixService struct {
	cfgPath string
}

// Execute is the SCM entry point: it starts the app and translates service
// control requests (stop/shutdown) into context cancellation.
func (m *meowflixService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	cfg, log, err := loadAndLogger(m.cfgPath)
	if err != nil {
		return true, 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc := make(chan error, 1)
	go func() { errc <- app.New(cfg, log, filepath.Dir(m.cfgPath)).Run(ctx) }()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-errc:
				case <-time.After(10 * time.Second):
				}
				return false, 0
			}
		case <-errc:
			// App exited on its own.
			return false, 0
		}
	}
}

// runUnderSCM runs the service handler loop.
func runUnderSCM(cfgPath string) error {
	return svc.Run(serviceName, &meowflixService{cfgPath: cfgPath})
}

// installService registers the current executable as an auto-start Windows
// service. It bakes in the absolute config path so the service still finds it
// when the SCM launches it from C:\Windows\System32.
func installService(cfgPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if abs, aerr := filepath.Abs(exePath); aerr == nil {
		exePath = abs
	}
	absCfg := cfgPath
	if abs, aerr := filepath.Abs(cfgPath); aerr == nil {
		absCfg = abs
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()

	if s, oerr := m.OpenService(serviceName); oerr == nil {
		s.Close()
		return fmt.Errorf("service %q already installed", serviceName)
	}

	cfg := mgr.Config{
		DisplayName:  serviceDisplay,
		Description:  serviceDesc,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}
	s, err := m.CreateService(serviceName, exePath, cfg, "-service", "run", "-config", absCfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Auto-restart on failure: 5s, 5s, then every 30s; reset window 1 day.
	if rerr := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}, uint32((24 * time.Hour).Seconds())); rerr != nil {
		// Non-fatal: the service is installed, recovery config is best-effort.
		fmt.Fprintf(os.Stderr, "warning: set recovery actions: %v\n", rerr)
	}

	fmt.Printf("installed service %q (exe=%s config=%s)\n", serviceName, exePath, absCfg)
	return nil
}

// controlService performs uninstall/start/stop/status against an installed service.
func controlService(action string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q not installed: %w", serviceName, err)
	}
	defer s.Close()

	switch action {
	case "uninstall", "remove":
		if err := s.Delete(); err != nil {
			return fmt.Errorf("delete service: %w", err)
		}
		fmt.Printf("uninstalled service %q\n", serviceName)
		return nil
	case "start":
		if err := s.Start(); err != nil {
			return fmt.Errorf("start service: %w", err)
		}
		fmt.Printf("started service %q\n", serviceName)
		return nil
	case "stop":
		status, err := s.Control(svc.Stop)
		if err != nil {
			return fmt.Errorf("stop service: %w", err)
		}
		timeout := time.Now().Add(10 * time.Second)
		for status.State != svc.Stopped {
			if time.Now().After(timeout) {
				return fmt.Errorf("timed out waiting for service to stop")
			}
			time.Sleep(300 * time.Millisecond)
			if status, err = s.Query(); err != nil {
				return fmt.Errorf("query service: %w", err)
			}
		}
		fmt.Printf("stopped service %q\n", serviceName)
		return nil
	case "status":
		status, err := s.Query()
		if err != nil {
			return fmt.Errorf("query service: %w", err)
		}
		fmt.Printf("service %q state: %s\n", serviceName, stateString(status.State))
		return nil
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

func stateString(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start-pending"
	case svc.StopPending:
		return "stop-pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue-pending"
	case svc.PausePending:
		return "pause-pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
