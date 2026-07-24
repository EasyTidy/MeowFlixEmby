// Command meowflix is the standalone entry point for the MeowFlixEmby daemon.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/EasyTidy/MeowFlixEmby/internal/app"
	"github.com/EasyTidy/MeowFlixEmby/internal/config"
)

func main() {
	cfgPath := flag.String("config", "meowflix.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		// Logger not yet configured; use a plain default.
		slog.Error("load config", slog.String("err", err.Error()))
		os.Exit(1)
	}

	log := newLogger(cfg.Log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Error("fatal", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

// newLogger builds a structured logger honouring the configured level.
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
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
