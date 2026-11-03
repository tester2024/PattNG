package engine

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cretz/bine/tor"
	"github.com/patterniha/PattNG/tor-core/bridge"
	"github.com/patterniha/PattNG/tor-core/config"
	"github.com/patterniha/PattNG/tor-core/transport"
)

var ErrNoTorRuntime = errors.New("no tor or arti runtime binary found")

// Engine coordinates bridge resolution, pluggable transport wiring, and the Tor core process.
type Engine struct {
	cfg      config.Config
	registry *transport.Registry
	fetcher  *bridge.Fetcher
}

// New creates a new Engine for the given Config.
func New(cfg config.Config) *Engine {
	toggles := bridge.Toggles{
		Snowflake: cfg.Snowflake,
		Lyrebird:  cfg.Lyrebird,
		Conjure:   cfg.Conjure,
	}

	return &Engine{
		cfg:      cfg,
		registry: transport.NewRegistry(toggles),
		fetcher: &bridge.Fetcher{
			BaseURL:       cfg.MoatURL,
			CacheDir:      cfg.DataDir,
			UpstreamProxy: cfg.UpstreamProxy,
			Timeout:       cfg.FetchTimeout,
		},
	}
}

// Prepare resolves custom and auto-fetched bridges and writes the bridge configuration file.
func (e *Engine) Prepare(ctx context.Context) (bridge.Plan, string, error) {
	toggles := bridge.Toggles{
		Snowflake: e.cfg.Snowflake,
		Lyrebird:  e.cfg.Lyrebird,
		Conjure:   e.cfg.Conjure,
	}

	custom := slices.Clone(e.cfg.Bridges)
	if fileLines, err := bridge.ReadFile(e.cfg.BridgeFile); err == nil && len(fileLines) > 0 {
		custom = append(custom, fileLines...)
	}

	plan := e.fetcher.Resolve(ctx, custom, e.cfg.AutoFetch, toggles)
	var bridgeFile string
	if len(plan.Lines) > 0 {
		var err error
		bridgeFile, err = WriteBridgeFile(e.cfg.DataDir, plan.Lines)
		if err != nil {
			return bridge.Plan{}, "", err
		}
	}

	return plan, bridgeFile, nil
}

// Run resolves bridges and runs the Tor session until ctx is canceled or the underlying process exits.
func (e *Engine) Run(ctx context.Context) error {
	plan, bridgeFile, err := e.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("prepare bridges: %w", err)
	}

	log.Printf("[+] tor-core prepared %d bridge(s) from %s (snowflake=%v, lyrebird=%v, conjure=%v)",
		len(plan.Lines), plan.Source, e.cfg.Snowflake, e.cfg.Lyrebird, e.cfg.Conjure)

	selfBin, _ := os.Executable()
	lyrebirdBin := resolveSiblingBinary(e.cfg.LyrebirdBinary, selfBin, "liblyrebird.so", "lyrebird")
	conjureBin := resolveSiblingBinary(e.cfg.ConjureBinary, selfBin, "libconjure.so", "conjure-client")

	if torBin := resolveSiblingBinary(e.cfg.TorBinary, selfBin, "tor"); torBin != "" {
		return e.runBine(ctx, torBin, lyrebirdBin, conjureBin, selfBin, plan.Lines)
	}

	artiBin := resolveSiblingBinary(e.cfg.ArtiBinary, selfBin, "libaether.so", "aether")
	if artiBin == "" {
		return ErrNoTorRuntime
	}

	ptSpec := e.registry.FormatAetherPT(lyrebirdBin, conjureBin, selfBin)
	args, extraEnv := BuildArtiInvocation(e.cfg, bridgeFile, ptSpec)

	cmd := exec.CommandContext(ctx, artiBin, args...)
	cmd.Dir = e.cfg.DataDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)

	return cmd.Run()
}

func (e *Engine) runBine(ctx context.Context, torBin, lyrebirdBin, conjureBin, selfBin string, bridges []string) error {
	ptLines := e.registry.TorrcTransportLines(lyrebirdBin, conjureBin, selfBin)
	torrcPath, err := WriteTorrc(e.cfg.DataDir, e.cfg.ListenAddr, e.cfg.UpstreamProxy, bridges, ptLines)
	if err != nil {
		return err
	}

	instance, err := tor.Start(ctx, &tor.StartConf{
		ExePath:         torBin,
		DataDir:         filepath.Join(e.cfg.DataDir, "bine-data"),
		TorrcFile:       torrcPath,
		EnableNetwork:   true,
		NoAutoSocksPort: true,
		DebugWriter:     os.Stderr,
	})
	if err != nil {
		return err
	}
	defer instance.Close()

	log.Printf("[+] tor is ready; %s leaves through tor", e.cfg.ListenAddr)
	<-ctx.Done()
	return nil
}

func resolveSiblingBinary(configured, selfBin string, names ...string) string {
	if trimmed := strings.TrimSpace(configured); trimmed != "" {
		if isExecutable(trimmed) {
			return trimmed
		}
	}

	if selfBin != "" {
		dir := filepath.Dir(selfBin)
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if isExecutable(candidate) {
				return candidate
			}
		}
	}

	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return cmp.Or(path, name)
		}
	}

	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
