// Package config loads and validates MeowFlixEmby runtime configuration.
//
// Configuration is YAML-first with environment-variable overrides for a small
// set of sensitive fields, so credentials need not be committed to disk.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerType identifies the media server flavour.
type ServerType string

const (
	ServerEmby     ServerType = "emby"
	ServerJellyfin ServerType = "jellyfin"
	ServerPlex     ServerType = "plex"
)

// Config is the root configuration document.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Playback PlaybackConfig `yaml:"playback"`
	Players  PlayersConfig  `yaml:"players"`
	Subtitle SubtitleConfig `yaml:"subtitle"`
	Version  VersionConfig  `yaml:"version"`
	Log      LogConfig      `yaml:"log"`
}

// ServerConfig describes how to reach and authenticate against the media server.
type ServerConfig struct {
	Type       ServerType `yaml:"type"`
	Address    string     `yaml:"address"`
	Username   string     `yaml:"username"`
	Password   string     `yaml:"password"`
	APIKey     string     `yaml:"api_key"`
	DeviceName string     `yaml:"device_name"`
	// SkipTLSVerify disables server certificate verification (self-signed). Off by default.
	SkipTLSVerify bool `yaml:"skip_tls_verify"`
}

// PathMap maps a server-side path prefix to a local (mounted) path prefix.
type PathMap struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

// PlaybackConfig controls how a media source is turned into a playable target.
type PlaybackConfig struct {
	PathMaps          []PathMap `yaml:"path_maps"`
	ForceDiskPrefixes []string  `yaml:"force_disk_prefixes"`
	DirectURLHosts    []string  `yaml:"direct_url_hosts"`
	PathCheck         bool      `yaml:"path_check"`
	OneInstance       bool      `yaml:"one_instance"`
}

// PlayerByPath selects a specific player when the media path matches keywords.
type PlayerByPath struct {
	Player string   `yaml:"player"`
	Match  []string `yaml:"match"`
}

// PlayersConfig configures which player to launch and where the binaries live.
type PlayersConfig struct {
	Default string            `yaml:"default"`
	ByPath  []PlayerByPath    `yaml:"by_path"`
	Exe     map[string]string `yaml:"exe"`
}

// SubtitleConfig holds subtitle selection preferences (keyword priority).
type SubtitleConfig struct {
	Priority []string `yaml:"priority"`
}

// VersionConfig holds multi-version selection preferences (keyword priority).
type VersionConfig struct {
	Prefer []string `yaml:"prefer"`
}

// LogConfig controls logging output.
type LogConfig struct {
	Level         string `yaml:"level"`
	File          string `yaml:"file"`
	MaskSensitive bool   `yaml:"mask_sensitive"`
}

// Load reads, parses and validates a config file at path, then applies
// environment overrides. It returns a usable Config or a descriptive error.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	cfg.applyEnvOverrides()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return cfg, nil
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	host, _ := os.Hostname()
	if host == "" {
		host = "MeowFlix"
	}
	return &Config{
		Server: ServerConfig{
			Type:       ServerEmby,
			DeviceName: "MeowFlix (" + host + ")",
		},
		Playback: PlaybackConfig{PathCheck: false, OneInstance: false},
		Players:  PlayersConfig{Default: "mpv", Exe: map[string]string{}},
		Log:      LogConfig{Level: "info", MaskSensitive: true},
	}
}

// applyEnvOverrides lets sensitive fields be supplied via environment variables
// (MEOWFLIX_SERVER_ADDRESS, MEOWFLIX_USERNAME, MEOWFLIX_PASSWORD, MEOWFLIX_API_KEY).
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("MEOWFLIX_SERVER_ADDRESS"); v != "" {
		c.Server.Address = v
	}
	if v := os.Getenv("MEOWFLIX_USERNAME"); v != "" {
		c.Server.Username = v
	}
	if v := os.Getenv("MEOWFLIX_PASSWORD"); v != "" {
		c.Server.Password = v
	}
	if v := os.Getenv("MEOWFLIX_API_KEY"); v != "" {
		c.Server.APIKey = v
	}
}

// Validate checks required fields and internal consistency.
func (c *Config) Validate() error {
	switch c.Server.Type {
	case ServerEmby, ServerJellyfin, ServerPlex:
	default:
		return fmt.Errorf("server.type must be emby|jellyfin|plex, got %q", c.Server.Type)
	}
	if strings.TrimSpace(c.Server.Address) == "" {
		return fmt.Errorf("server.address is required")
	}
	if !strings.HasPrefix(c.Server.Address, "http://") && !strings.HasPrefix(c.Server.Address, "https://") {
		return fmt.Errorf("server.address must start with http:// or https://")
	}
	if c.Server.APIKey == "" && (c.Server.Username == "" || c.Server.Password == "") {
		return fmt.Errorf("provide server.api_key, or both server.username and server.password")
	}
	if c.Players.Default == "" {
		return fmt.Errorf("players.default is required")
	}
	for i, pm := range c.Playback.PathMaps {
		if pm.Src == "" || pm.Dst == "" {
			return fmt.Errorf("playback.path_maps[%d] needs both src and dst", i)
		}
	}
	return nil
}
