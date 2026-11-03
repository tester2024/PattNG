package config

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	EnvListenAddr     = "TOR_CORE_BIND"
	EnvDataDir        = "TOR_CORE_DATA_DIR"
	EnvUpstreamProxy  = "TOR_CORE_UPSTREAM"
	EnvLogLevel       = "TOR_CORE_LOG_LEVEL"
	EnvMoatURL        = "TOR_CORE_MOAT_URL"
	EnvTorBinary      = "TOR_CORE_TOR_BIN"
	EnvArtiBinary     = "TOR_CORE_ARTI_BIN"
	EnvLyrebirdBinary = "TOR_CORE_LYREBIRD_BIN"
	EnvConjureBinary  = "TOR_CORE_CONJURE_BIN"
)

var (
	ErrEmptyListenAddr   = errors.New("listen address is required")
	ErrInvalidListenAddr = errors.New("listen address must be a valid ip:port")
	ErrInvalidLogLevel   = errors.New("log level must be debug, info, warn, or error")
	ErrNoTransports      = errors.New("at least one transport must be enabled when bridges require a pluggable transport")
)

var validLogLevels = []string{"debug", "info", "warn", "error"}

// Config holds the runtime settings for the Tor core process.
type Config struct {
	ListenAddr     string
	DataDir        string
	UpstreamProxy  string
	LogLevel       string
	Snowflake      bool
	Lyrebird       bool
	Conjure        bool
	AutoFetch      bool
	Bridges        []string
	BridgeFile     string
	MoatURL        string
	FetchTimeout   time.Duration
	TorBinary      string
	ArtiBinary     string
	LyrebirdBinary string
	ConjureBinary  string
}

// Default returns a Config populated from environment overrides and sensible defaults.
func Default() Config {
	return Config{
		ListenAddr:     cmp.Or(os.Getenv(EnvListenAddr), "127.0.0.1:10829"),
		DataDir:        cmp.Or(os.Getenv(EnvDataDir), os.TempDir()),
		UpstreamProxy:  os.Getenv(EnvUpstreamProxy),
		LogLevel:       cmp.Or(os.Getenv(EnvLogLevel), "info"),
		Snowflake:      true,
		Lyrebird:       true,
		Conjure:        true,
		AutoFetch:      true,
		MoatURL:        cmp.Or(os.Getenv(EnvMoatURL), "https://bridges.torproject.org/moat/circumvention"),
		FetchTimeout:   25 * time.Second,
		TorBinary:      os.Getenv(EnvTorBinary),
		ArtiBinary:     os.Getenv(EnvArtiBinary),
		LyrebirdBinary: os.Getenv(EnvLyrebirdBinary),
		ConjureBinary:  os.Getenv(EnvConjureBinary),
	}
}

// ParseFlags parses command-line arguments into a validated Config.
func ParseFlags(args []string) (Config, error) {
	cfg := Default()
	var bridges stringSliceFlag

	fs := flag.NewFlagSet("tor-core", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddr, "bind", cfg.ListenAddr, "SOCKS5 listen address (ip:port)")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directory for Tor state and bridge cache")
	fs.StringVar(&cfg.UpstreamProxy, "upstream", cfg.UpstreamProxy, "optional upstream SOCKS5 proxy URL")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level (debug, info, warn, error)")
	fs.BoolVar(&cfg.Snowflake, "snowflake", cfg.Snowflake, "enable Snowflake pluggable transport")
	fs.BoolVar(&cfg.Lyrebird, "lyrebird", cfg.Lyrebird, "enable Lyrebird (obfs4/webtunnel/meek) pluggable transport")
	fs.BoolVar(&cfg.Conjure, "conjure", cfg.Conjure, "enable Conjure pluggable transport")
	fs.BoolVar(&cfg.AutoFetch, "auto-fetch", cfg.AutoFetch, "automatically fetch bridges from Tor Moat API")
	fs.Var(&bridges, "bridge", "custom bridge line (repeatable)")
	fs.StringVar(&cfg.BridgeFile, "bridge-file", cfg.BridgeFile, "file containing custom bridge lines")
	fs.StringVar(&cfg.MoatURL, "moat-url", cfg.MoatURL, "Tor Moat circumvention API base URL")
	fs.DurationVar(&cfg.FetchTimeout, "fetch-timeout", cfg.FetchTimeout, "bridge auto-fetch HTTP timeout")
	fs.StringVar(&cfg.TorBinary, "tor-bin", cfg.TorBinary, "path to tor executable")
	fs.StringVar(&cfg.ArtiBinary, "arti-bin", cfg.ArtiBinary, "path to arti-capable executable")
	fs.StringVar(&cfg.LyrebirdBinary, "lyrebird-bin", cfg.LyrebirdBinary, "path to lyrebird executable")
	fs.StringVar(&cfg.ConjureBinary, "conjure-bin", cfg.ConjureBinary, "path to conjure-client executable")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Bridges = slices.Clone(bridges)
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks that the configuration holds valid addresses and log levels.
func (c Config) Validate() error {
	addr := strings.TrimSpace(c.ListenAddr)
	if addr == "" {
		return ErrEmptyListenAddr
	}

	ap, err := netip.ParseAddrPort(addr)
	if err != nil || ap.Port() == 0 {
		return fmt.Errorf("%w: %s", ErrInvalidListenAddr, c.ListenAddr)
	}

	level := strings.ToLower(strings.TrimSpace(c.LogLevel))
	if !slices.Contains(validLogLevels, level) {
		return fmt.Errorf("%w: %s", ErrInvalidLogLevel, c.LogLevel)
	}

	return nil
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ";")
}

func (s *stringSliceFlag) Set(val string) error {
	for part := range strings.SplitSeq(val, "\n") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			*s = append(*s, trimmed)
		}
	}
	return nil
}
