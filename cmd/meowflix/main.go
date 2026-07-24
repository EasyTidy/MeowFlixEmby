// Command meowflix is the standalone entry point for the MeowFlixEmby daemon.
//
// It runs in the foreground by default. On Windows it can also install and run
// as a Windows service via the -service flag (install|uninstall|start|stop|
// status|run); when launched by the Service Control Manager it detects that and
// runs under the SCM automatically.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/EasyTidy/MeowFlixEmby/internal/app"
	"github.com/EasyTidy/MeowFlixEmby/internal/config"
)

// Build metadata, injected via -ldflags "-X main.version=... -X main.commit=... -X main.buildDate=...".
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	cfgPath := flag.String("config", "meowflix.yaml", "path to config file")
	svcAction := flag.String("service", "", "Windows service control: install|uninstall|start|stop|status|run")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("meowflix %s (commit %s, built %s)\n", version, commit, buildDate)
		return
	}

	// Resolve to an absolute config path so a service (whose working directory
	// is C:\Windows\System32) still finds it.
	absCfg := *cfgPath
	if p, err := filepath.Abs(*cfgPath); err == nil {
		absCfg = p
	}

	// On Windows, handle service control / SCM-managed execution. On other
	// platforms this returns handled=false unless a -service action was given.
	if handled, err := runService(absCfg, *svcAction); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "service:", err)
			os.Exit(1)
		}
		return
	}

	if err := runConsole(absCfg); err != nil {
		slog.Error("fatal", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

// runConsole runs the daemon in the foreground until interrupted.
func runConsole(cfgPath string) error {
	cfg, log, err := loadAndLogger(cfgPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.New(cfg, log, filepath.Dir(cfgPath)).Run(ctx)
}

// loadAndLogger loads config and builds the logger, shared by console and
// service execution paths.
func loadAndLogger(cfgPath string) (*config.Config, *slog.Logger, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, newLogger(cfg.Log), nil
}

// newLogger builds a structured logger honouring the configured level and,
// when set, writing to the configured file (falling back to stderr on error).
func newLogger(c config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(c.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var w io.Writer = os.Stderr
	if c.File != "" {
		if f, err := os.OpenFile(c.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			w = f
		}
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
