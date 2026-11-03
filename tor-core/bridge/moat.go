package bridge

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	cacheFileName = "tor-bridges-cache.json"
	cacheMaxAge   = 6 * time.Hour
)

var transportOrder = []string{
	"obfs4",
	"webtunnel",
	"snowflake",
	"conjure",
	"meek_lite",
	"meek",
}

// Fetcher retrieves bridge lines from the Tor Moat circumvention API with local caching.
type Fetcher struct {
	BaseURL       string
	CacheDir      string
	UpstreamProxy string
	Timeout       time.Duration
	HTTPClient    *http.Client
}

// Plan holds the resolved bridge lines and their origin description.
type Plan struct {
	Lines  []string
	Source string
}

type cachedBridges struct {
	FetchedAt int64    `json:"fetched_at"`
	Lines     []string `json:"lines"`
}

type moatReply struct {
	Settings []struct {
		Bridges struct {
			Type    string   `json:"type"`
			Strings []string `json:"bridge_strings"`
		} `json:"bridges"`
	} `json:"settings"`
}

// Resolve combines custom bridges and (when autoFetch is true) Moat-fetched or built-in bridges,
// filtering all lines through the active transport toggles.
func (f *Fetcher) Resolve(ctx context.Context, custom []string, autoFetch bool, toggles Toggles) Plan {
	filteredCustom := FilterByToggles(custom, toggles)
	if !autoFetch {
		if len(filteredCustom) > 0 {
			return Plan{Lines: filteredCustom, Source: "custom"}
		}
		return Plan{Lines: BuiltinFor(toggles), Source: "builtin"}
	}

	fetched, source := f.Fetch(ctx, toggles)
	combined := FilterByToggles(slices.Concat(filteredCustom, fetched), toggles)
	if len(filteredCustom) > 0 && len(fetched) > 0 {
		source = "custom+" + source
	} else if len(filteredCustom) > 0 {
		source = "custom"
	}

	return Plan{
		Lines:  combined,
		Source: source,
	}
}

// Fetch queries the cache or Moat API and filters the resulting bridges by toggles.
func (f *Fetcher) Fetch(ctx context.Context, toggles Toggles) ([]string, string) {
	cached := f.readCache()
	if cached != nil && time.Since(time.Unix(cached.FetchedAt, 0)) < cacheMaxAge {
		if filtered := FilterByToggles(cached.Lines, toggles); len(filtered) > 0 {
			return ensureRequestedTransports(filtered, toggles), "cache"
		}
	}

	client, err := f.httpClient()
	if err != nil {
		return f.fallback(cached, toggles)
	}

	var raw []string
	if settings := f.fetchSettings(ctx, client, toggles); len(settings) > 0 {
		raw = append(raw, settings...)
	}
	if builtin := f.fetchBuiltin(ctx, client); len(builtin) > 0 {
		raw = append(raw, builtin...)
	}

	deduped := ParseLines(strings.Join(raw, "\n"))
	if len(deduped) == 0 {
		return f.fallback(cached, toggles)
	}

	f.writeCache(deduped)
	filtered := FilterByToggles(deduped, toggles)
	if len(filtered) == 0 {
		return f.fallback(cached, toggles)
	}

	return ensureRequestedTransports(filtered, toggles), "moat"
}

func ensureRequestedTransports(lines []string, toggles Toggles) []string {
	out := slices.Clone(lines)
	for _, b := range BuiltinFor(toggles) {
		pt := TransportOf(b)
		hasTransport := slices.ContainsFunc(out, func(existing string) bool {
			return TransportOf(existing) == pt
		})
		if !hasTransport {
			out = append(out, b)
		}
	}
	return out
}

func (f *Fetcher) fallback(cached *cachedBridges, toggles Toggles) ([]string, string) {
	if cached != nil {
		if stale := FilterByToggles(cached.Lines, toggles); len(stale) > 0 {
			return ensureRequestedTransports(stale, toggles), "stale-cache"
		}
	}
	return BuiltinFor(toggles), "builtin"
}

func (f *Fetcher) fetchSettings(ctx context.Context, client *http.Client, toggles Toggles) []string {
	var transports []string
	if toggles.Lyrebird {
		transports = append(transports, "obfs4", "webtunnel", "meek_lite")
	}
	if toggles.Snowflake {
		transports = append(transports, "snowflake")
	}
	if toggles.Conjure {
		transports = append(transports, "conjure")
	}

	payload, err := json.Marshal(map[string]any{"transports": transports})
	if err != nil {
		return nil
	}

	endpoint := strings.TrimRight(f.baseURL(), "/") + "/settings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/vnd.api+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var reply moatReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return nil
	}

	var lines []string
	for _, s := range reply.Settings {
		lines = append(lines, s.Bridges.Strings...)
	}
	return lines
}

func (f *Fetcher) fetchBuiltin(ctx context.Context, client *http.Client) []string {
	endpoint := strings.TrimRight(f.baseURL(), "/") + "/builtin"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var grouped map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&grouped); err != nil {
		return nil
	}

	var ordered []string
	for _, kind := range transportOrder {
		if found, ok := grouped[kind]; ok {
			ordered = append(ordered, found...)
			delete(grouped, kind)
		}
	}
	for _, remaining := range grouped {
		ordered = append(ordered, remaining...)
	}

	return ordered
}

func (f *Fetcher) httpClient() (*http.Client, error) {
	if f.HTTPClient != nil {
		return f.HTTPClient, nil
	}

	timeout := cmp.Or(f.Timeout, 25*time.Second)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil

	if rawProxy := strings.TrimSpace(f.UpstreamProxy); rawProxy != "" {
		parsed, err := url.Parse(rawProxy)
		if err != nil {
			return nil, err
		}

		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return nil, err
		}

		if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = ctxDialer.DialContext
		} else {
			transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

func (f *Fetcher) baseURL() string {
	return cmp.Or(strings.TrimSpace(f.BaseURL), "https://bridges.torproject.org/moat/circumvention")
}

func (f *Fetcher) readCache() *cachedBridges {
	if strings.TrimSpace(f.CacheDir) == "" {
		return nil
	}

	raw, err := os.ReadFile(filepath.Join(f.CacheDir, cacheFileName))
	if err != nil {
		return nil
	}

	var cached cachedBridges
	if err := json.Unmarshal(raw, &cached); err != nil || len(cached.Lines) == 0 {
		return nil
	}

	return &cached
}

func (f *Fetcher) writeCache(lines []string) {
	if strings.TrimSpace(f.CacheDir) == "" || len(lines) == 0 {
		return
	}

	if err := os.MkdirAll(f.CacheDir, 0o700); err != nil {
		return
	}

	body, err := json.MarshalIndent(cachedBridges{
		FetchedAt: time.Now().Unix(),
		Lines:     lines,
	}, "", "  ")
	if err != nil {
		return
	}

	target := filepath.Join(f.CacheDir, cacheFileName)
	tmp := target + ".new"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, target)
}
