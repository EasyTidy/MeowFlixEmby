// Package app wires the pieces together and owns the process lifecycle.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/EasyTidy/MeowFlixEmby/internal/config"
)

// App holds the assembled application dependencies.
type App struct {
	cfg *config.Config
	log *slog.Logger
}

// New assembles an App from validated configuration.
func New(cfg *config.Config, log *slog.Logger) *App {
	return &App{cfg: cfg, log: log}
}

// Run starts the daemon and blocks until ctx is cancelled.
//
// M0: lifecycle skeleton only. Later milestones inject the mediaserver.Server,
// remote.Session, resolver.Resolver, player.Registry and playback.Controller.
func (a *App) Run(ctx context.Context) error {
	a.log.Info("MeowFlixEmby starting",
		slog.String("server_type", string(a.cfg.Server.Type)),
		slog.String("device_name", a.cfg.Server.DeviceName),
	)

	// TODO(M2-M5): authenticate, announce capabilities, open remote session,
	// consume commands, resolve + play + report. Skeleton just waits for shutdown.
	<-ctx.Done()
	a.log.Info("MeowFlixEmby shutting down")
	if err := ctx.Err(); err != nil && err != context.Canceled {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}
