// Package app wires the pieces together and owns the process lifecycle.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/EasyTidy/MeowFlixEmby/internal/config"
	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver"
	"github.com/EasyTidy/MeowFlixEmby/pkg/mediaserver/emby"
	"github.com/EasyTidy/MeowFlixEmby/pkg/openlist"
	"github.com/EasyTidy/MeowFlixEmby/pkg/playback"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player/generic"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player/mpc"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player/mpv"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player/potplayer"
	"github.com/EasyTidy/MeowFlixEmby/pkg/player/vlc"
	"github.com/EasyTidy/MeowFlixEmby/pkg/resolver"
)

// App holds the assembled application dependencies.
type App struct {
	cfg      *config.Config
	log      *slog.Logger
	stateDir string
}

// New assembles an App from validated configuration. stateDir is where small
// persistent state (e.g. the device id) is stored; pass the config file's dir.
func New(cfg *config.Config, log *slog.Logger, stateDir string) *App {
	if stateDir == "" {
		stateDir = "."
	}
	return &App{cfg: cfg, log: log, stateDir: filepath.Clean(stateDir)}
}

// defaultCapabilities is what we advertise so the web client lists us under
// "Play On". SupportedCommands lists only what later milestones will implement.
func defaultCapabilities() mediaserver.Capabilities {
	return mediaserver.Capabilities{
		PlayableMediaTypes: []string{"Video", "Audio"},
		SupportedCommands: []string{
			"PlayPause", "Pause", "Unpause", "Stop", "Seek",
			"NextTrack", "PreviousTrack",
			"SetSubtitleStreamIndex", "SetAudioStreamIndex",
			"DisplayMessage", "SetVolume", "Mute", "Unmute",
		},
		SupportsMediaControl:         true,
		SupportsPersistentIdentifier: true,
	}
}

// flavorFor maps the configured server type to the emby client flavor.
func flavorFor(t config.ServerType) (emby.Flavor, error) {
	switch t {
	case config.ServerEmby:
		return emby.FlavorEmby, nil
	case config.ServerJellyfin:
		return emby.FlavorJellyfin, nil
	default:
		return 0, fmt.Errorf("server type %q not yet supported for remote sessions", t)
	}
}

// Run starts the daemon and blocks until ctx is cancelled.
//
// M3: authenticate, announce capabilities and open the remote WebSocket session
// so the client appears under "Play On", then consume inbound commands. Later
// milestones (M4/M5) replace the log-only consumer with resolve + play + report.
func (a *App) Run(ctx context.Context) error {
	a.log.Info("MeowFlixEmby starting",
		slog.String("server_type", string(a.cfg.Server.Type)),
		slog.String("device_name", a.cfg.Server.DeviceName),
	)

	flavor, err := flavorFor(a.cfg.Server.Type)
	if err != nil {
		return err
	}
	deviceID := stableDeviceID(a.stateDir)

	client := emby.New(emby.Options{
		Address:       a.cfg.Server.Address,
		Username:      a.cfg.Server.Username,
		Password:      a.cfg.Server.Password,
		APIKey:        a.cfg.Server.APIKey,
		DeviceName:    a.cfg.Server.DeviceName,
		DeviceID:      deviceID,
		Flavor:        flavor,
		SkipTLSVerify: a.cfg.Server.SkipTLSVerify,
	})

	if err := client.Authenticate(ctx); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	a.log.Info("authenticated", slog.String("device_id", deviceID))

	// Assemble the playback controller (resolver + player registry + reporter).
	controller := playback.New(playback.Options{
		Server:      client,
		Resolver:    resolver.New(),
		Players:     a.buildPlayers(),
		Openlist:    a.buildOpenlist(),
		ResolverCfg: a.resolverConfig(),
		DeviceID:    deviceID,
		Logger:      a.log,
	})

	session := client.NewSession(a.log, defaultCapabilities())

	// Consume commands through the controller until the session ends.
	go controller.Consume(ctx, session.Commands())

	err = session.Run(ctx)
	a.log.Info("MeowFlixEmby shutting down")
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// buildPlayers constructs the player registry from configuration. mpv is the
// first-class driver (full remote control); vlc and potplayer are also
// registered for by_path / default selection.
func (a *App) buildPlayers() *player.MapRegistry {
	fsArgs := a.fullscreenArgs()
	players := []player.Player{
		mpv.New(mpv.Options{ExePath: a.cfg.Players.Exe["mpv"], ExtraArgs: fsArgs["mpv"]}),
		vlc.New(vlc.Options{ExePath: a.cfg.Players.Exe["vlc"], ExtraArgs: fsArgs["vlc"]}),
		potplayer.New(potplayer.Options{ExePath: a.cfg.Players.Exe["potplayer"], ExtraArgs: fsArgs["potplayer"]}),
		mpc.New(mpc.Options{ExePath: a.cfg.Players.Exe["mpc-hc"], ExtraArgs: fsArgs["mpc-hc"]}),
	}
	// A generic launch-only driver for any other configured executable.
	if exe := a.cfg.Players.Exe["generic"]; exe != "" {
		players = append(players, generic.New(generic.Options{ExePath: exe, ExtraArgs: fsArgs["generic"]}))
	}
	byPath := make([]player.PathRule, 0, len(a.cfg.Players.ByPath))
	for _, r := range a.cfg.Players.ByPath {
		byPath = append(byPath, player.PathRule{Player: r.Player, Match: r.Match})
	}
	return player.NewRegistry(players, a.cfg.Players.Default, byPath)
}

// fullscreenArgs returns the fullscreen CLI argument for each player type
// when players.fullscreen is enabled, or nil otherwise.
func (a *App) fullscreenArgs() map[string][]string {
	if !a.cfg.Players.Fullscreen {
		return nil
	}
	return map[string][]string{
		"mpv":       {"--fs"},
		"vlc":       {"-f"},
		"mpc-hc":    {"/fullscreen"},
		"potplayer": {"/fullscreen"},
	}
}

// buildOpenlist constructs the openlist client when configured, else nil.
func (a *App) buildOpenlist() *openlist.Client {
	if a.cfg.Openlist.Host == "" {
		return nil
	}
	a.log.Info("openlist enabled", slog.String("host", a.cfg.Openlist.Host))
	return openlist.New(openlist.Options{
		Host:          a.cfg.Openlist.Host,
		Token:         a.cfg.Openlist.Token,
		SkipTLSVerify: a.cfg.Server.SkipTLSVerify,
	})
}

// resolverConfig maps the app config into the resolver's decoupled config.
func (a *App) resolverConfig() resolver.Config {
	maps := make([]resolver.PathMap, 0, len(a.cfg.Playback.PathMaps))
	for _, m := range a.cfg.Playback.PathMaps {
		maps = append(maps, resolver.PathMap{Src: m.Src, Dst: m.Dst})
	}
	olMaps := make([]resolver.PathMap, 0, len(a.cfg.Openlist.PathMaps))
	for _, m := range a.cfg.Openlist.PathMaps {
		olMaps = append(olMaps, resolver.PathMap{Src: m.Src, Dst: m.Dst})
	}
	return resolver.Config{
		PathMaps:          maps,
		ForceDiskPrefixes: a.cfg.Playback.ForceDiskPrefixes,
		DirectURLHosts:    a.cfg.Playback.DirectURLHosts,
		PathCheck:         a.cfg.Playback.PathCheck,
		VersionPrefer:     a.cfg.Version.Prefer,
		SubtitlePriority:  a.cfg.Subtitle.Priority,
		OpenlistEnabled:   a.cfg.Openlist.Host != "",
		OpenlistPathMaps:  olMaps,
	}
}
