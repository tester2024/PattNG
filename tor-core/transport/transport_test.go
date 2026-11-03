package transport

import (
	"slices"
	"strings"
	"testing"

	"github.com/patterniha/PattNG/tor-core/bridge"
)

func TestRegistryToggles(t *testing.T) {
	all := NewRegistry(bridge.Toggles{Snowflake: true, Lyrebird: true, Conjure: true})
	methods := all.Methods()

	for _, want := range []string{"obfs4", "webtunnel", "meek_lite", "snowflake", "conjure"} {
		if !slices.Contains(methods, want) {
			t.Fatalf("expected %s in methods %v", want, methods)
		}
	}

	onlyConjure := NewRegistry(bridge.Toggles{Snowflake: false, Lyrebird: false, Conjure: true})
	if !slices.Equal(onlyConjure.Methods(), []string{"conjure"}) {
		t.Fatalf("expected only conjure, got %v", onlyConjure.Methods())
	}

	ptEnv := all.FormatAetherPT("/app/lib/liblyrebird.so", "/app/lib/libconjure.so", "/app/lib/libtor.so")
	if !strings.Contains(ptEnv, "obfs4=/app/lib/liblyrebird.so") {
		t.Fatalf("missing obfs4 mapping in %s", ptEnv)
	}
	if !strings.Contains(ptEnv, "snowflake=/app/lib/liblyrebird.so") {
		t.Fatalf("missing snowflake mapping in %s", ptEnv)
	}
	if !strings.Contains(ptEnv, "conjure=/app/lib/libconjure.so") {
		t.Fatalf("missing conjure mapping in %s", ptEnv)
	}
}
