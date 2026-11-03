package engine

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/patterniha/PattNG/tor-core/config"
)

func TestWriteTorrcAndArtiInvocation(t *testing.T) {
	dir := t.TempDir()
	bridges := []string{
		"obfs4 192.0.2.1:443 AAAA cert=xyz iat-mode=0",
		"conjure 23.128.36.60:80 BBBB url=https://example.org transport=prefix",
	}
	ptLines := []string{
		"ClientTransportPlugin obfs4,snowflake exec /app/lib/liblyrebird.so",
		"ClientTransportPlugin conjure exec /app/lib/libconjure.so",
	}

	torrcPath, err := WriteTorrc(dir, "127.0.0.1:10829", "socks5://127.0.0.1:10832", bridges, ptLines)
	if err != nil {
		t.Fatalf("WriteTorrc failed: %v", err)
	}

	data, err := os.ReadFile(torrcPath)
	if err != nil {
		t.Fatalf("read torrc failed: %v", err)
	}
	text := string(data)

	for _, want := range []string{
		"SocksPort 127.0.0.1:10829",
		"Socks5Proxy 127.0.0.1:10832",
		"UseBridges 1",
		"ClientTransportPlugin conjure exec /app/lib/libconjure.so",
		"Bridge conjure 23.128.36.60:80",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("torrc missing %q:\n%s", want, text)
		}
	}

	cfg := config.Default()
	cfg.DataDir = dir
	cfg.ListenAddr = "127.0.0.1:10829"
	cfg.UpstreamProxy = "socks5://127.0.0.1:10832"

	bridgeFile, err := WriteBridgeFile(dir, bridges)
	if err != nil {
		t.Fatalf("WriteBridgeFile failed: %v", err)
	}

	args, env := BuildArtiInvocation(cfg, bridgeFile, "obfs4=/lib/liblyrebird.so;conjure=/lib/libconjure.so")
	if !slices.Contains(args, "--tor-only") || !slices.Contains(args, "--tor-bridges") {
		t.Fatalf("unexpected arti args: %v", args)
	}
	if !slices.Contains(env, "AETHER_TOR_BRIDGE_FILE="+bridgeFile) {
		t.Fatalf("missing bridge file env in %v", env)
	}
}

func TestEnginePrepare(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.AutoFetch = false
	cfg.Snowflake = true
	cfg.Lyrebird = false
	cfg.Conjure = true
	cfg.Bridges = []string{
		"obfs4 192.0.2.1:443 AAAA cert=xyz iat-mode=0",
		"conjure 23.128.36.60:80 BBBB url=https://example.org transport=prefix",
	}

	eng := New(cfg)
	plan, bridgeFile, err := eng.Prepare(t.Context())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(plan.Lines) != 1 || !strings.HasPrefix(plan.Lines[0], "conjure ") {
		t.Fatalf("expected only conjure bridge after filtering, got %v", plan.Lines)
	}
	if bridgeFile == "" {
		t.Fatal("expected bridgeFile to be created")
	}
}
