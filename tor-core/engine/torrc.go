package engine

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/patterniha/PattNG/tor-core/config"
)

const (
	bridgeFileName = "tor-bridges.conf"
	torrcFileName  = "torrc.conf"
)

// WriteBridgeFile writes the resolved bridge lines to a file inside dir.
func WriteBridgeFile(dir string, bridges []string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	var b strings.Builder
	for _, line := range bridges {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			b.WriteString(trimmed)
			b.WriteByte('\n')
		}
	}

	path := filepath.Join(dir, bridgeFileName)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}

	return path, nil
}

// WriteTorrc generates a torrc file for github.com/cretz/bine/tor.
func WriteTorrc(dir, listenAddr, upstreamProxy string, bridges, ptLines []string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("SocksPort %s", strings.TrimSpace(listenAddr)))

	if proxyHost := socksHostPort(upstreamProxy); proxyHost != "" {
		lines = append(lines, fmt.Sprintf("Socks5Proxy %s", proxyHost))
	}

	if len(bridges) > 0 {
		lines = append(lines, "UseBridges 1")
		lines = append(lines, ptLines...)
		for _, br := range bridges {
			if trimmed := strings.TrimSpace(br); trimmed != "" {
				lines = append(lines, fmt.Sprintf("Bridge %s", trimmed))
			}
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	path := filepath.Join(dir, torrcFileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}

	return path, nil
}

// BuildArtiInvocation constructs the arguments and environment additions for running Arti (--tor-only).
func BuildArtiInvocation(cfg config.Config, bridgeFile, ptSpec string) ([]string, []string) {
	args := []string{
		"--bind", cfg.ListenAddr,
		"--tor-only",
		"--log-level", cfg.LogLevel,
	}

	if len(cfg.Bridges) > 0 || cfg.AutoFetch || bridgeFile != "" {
		args = append(args, "--tor-bridges")
	} else {
		args = append(args, "--no-tor-bridges")
	}

	if upstream := strings.TrimSpace(cfg.UpstreamProxy); upstream != "" {
		args = append(args, "--upstream", upstream)
	}

	var env []string
	if strings.TrimSpace(cfg.DataDir) != "" {
		env = append(env, "AETHER_TOR_DIR="+filepath.Join(cfg.DataDir, "tor-state"))
	}
	if strings.TrimSpace(bridgeFile) != "" {
		env = append(env, "AETHER_TOR_BRIDGE_FILE="+bridgeFile)
	}
	if strings.TrimSpace(ptSpec) != "" {
		env = append(env, "AETHER_TOR_PT="+ptSpec)
	}

	return args, env
}

func socksHostPort(rawProxy string) string {
	trimmed := strings.TrimSpace(rawProxy)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return u.Host
}
