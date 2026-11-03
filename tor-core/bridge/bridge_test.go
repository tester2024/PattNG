package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestParseLinesAndTransportOf(t *testing.T) {
	input := `
# comment line
Bridge obfs4 192.0.2.1:443 AAAA cert=abc iat-mode=0
bridge snowflake 192.0.2.3:80 BBBB url=https://example.com
conjure 23.128.36.60:80 CCCC url=https://example.org transport=prefix
198.51.100.10:9001 DDDD
Bridge obfs4 192.0.2.1:443 AAAA cert=abc iat-mode=0
`
	lines := ParseLines(input)
	if len(lines) != 4 {
		t.Fatalf("expected 4 deduplicated lines, got %d: %v", len(lines), lines)
	}

	wantTransports := []string{"obfs4", "snowflake", "conjure", ""}
	for i, want := range wantTransports {
		if got := TransportOf(lines[i]); got != want {
			t.Fatalf("line %d (%q): expected transport %q, got %q", i, lines[i], want, got)
		}
	}
}

func TestFilterByToggles(t *testing.T) {
	lines := []string{
		"obfs4 192.0.2.1:443 AAAA cert=abc iat-mode=0",
		"snowflake 192.0.2.3:80 BBBB url=https://example.com",
		"conjure 23.128.36.60:80 CCCC url=https://example.org",
		"198.51.100.10:9001 DDDD",
	}

	onlySnowflakeAndConjure := FilterByToggles(lines, Toggles{
		Snowflake: true,
		Lyrebird:  false,
		Conjure:   true,
	})
	if len(onlySnowflakeAndConjure) != 3 {
		t.Fatalf("expected 3 lines (snowflake, conjure, vanilla), got %v", onlySnowflakeAndConjure)
	}
	if slices.ContainsFunc(onlySnowflakeAndConjure, func(s string) bool { return strings.HasPrefix(s, "obfs4") }) {
		t.Fatal("expected obfs4 to be filtered out when Lyrebird is false")
	}
}

func TestFetcherResolveWithMockMoat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/settings":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": []map[string]any{
					{
						"bridges": map[string]any{
							"type": "obfs4",
							"bridge_strings": []string{
								"obfs4 198.51.100.1:443 1111 cert=test iat-mode=0",
							},
						},
					},
				},
			})
		case "/builtin":
			_ = json.NewEncoder(w).Encode(map[string]string{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	fetcher := &Fetcher{
		BaseURL:    srv.URL,
		CacheDir:   t.TempDir(),
		HTTPClient: srv.Client(),
	}

	toggles := Toggles{Snowflake: true, Lyrebird: true, Conjure: true}
	custom := []string{"Bridge conjure 198.51.100.99:80 9999 url=https://custom.example"}

	plan := fetcher.Resolve(t.Context(), custom, true, toggles)
	if plan.Source != "custom+moat" {
		t.Fatalf("expected source custom+moat, got %s", plan.Source)
	}
	if len(plan.Lines) < 3 {
		t.Fatalf("expected custom + moat + snowflake fallback bridges, got %v", plan.Lines)
	}
	if !strings.HasPrefix(plan.Lines[0], "conjure 198.51.100.99:80") {
		t.Fatalf("expected custom bridge first, got %s", plan.Lines[0])
	}
}
