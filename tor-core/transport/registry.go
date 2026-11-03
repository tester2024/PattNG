package transport

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/patterniha/PattNG/tor-core/bridge"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/meeklite"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/obfs2"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/obfs3"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/obfs4"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/scramblesuit"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/snowflake"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/webtunnel"
)

const conjureMethodName = "conjure"

// Registry manages the enabled pluggable transports for a Tor session.
type Registry struct {
	toggles    bridge.Toggles
	transports map[string]base.Transport
	order      []string
}

// NewRegistry constructs a Registry containing only the transports enabled by toggles.
func NewRegistry(toggles bridge.Toggles) *Registry {
	reg := &Registry{
		toggles:    toggles,
		transports: make(map[string]base.Transport),
	}

	if toggles.Lyrebird {
		for _, t := range []base.Transport{
			new(obfs4.Transport),
			webtunnel.Transport,
			new(meeklite.Transport),
			new(obfs3.Transport),
			new(obfs2.Transport),
			new(scramblesuit.Transport),
		} {
			reg.register(t)
		}
	}

	if toggles.Snowflake {
		reg.register(new(snowflake.Transport))
	}

	if toggles.Conjure {
		reg.order = append(reg.order, conjureMethodName)
	}

	return reg
}

func (r *Registry) register(t base.Transport) {
	name := t.Name()
	if _, exists := r.transports[name]; !exists {
		r.transports[name] = t
		r.order = append(r.order, name)
	}
}

// Methods returns the ordered list of enabled pluggable transport names.
func (r *Registry) Methods() []string {
	return slices.Clone(r.order)
}

// Get looks up an enabled in-process transport by name.
func (r *Registry) Get(name string) base.Transport {
	return r.transports[strings.ToLower(strings.TrimSpace(name))]
}

// FormatAetherPT formats the protocol=path semicolon-separated value consumed by AETHER_TOR_PT.
func (r *Registry) FormatAetherPT(lyrebirdBin, conjureBin, selfBin string) string {
	lyrebirdExec := cmp.Or(strings.TrimSpace(lyrebirdBin), strings.TrimSpace(selfBin))
	conjureExec := cmp.Or(strings.TrimSpace(conjureBin), strings.TrimSpace(selfBin))

	var entries []string
	for _, method := range r.order {
		bin := lyrebirdExec
		if method == conjureMethodName {
			bin = conjureExec
		}
		if bin != "" {
			entries = append(entries, fmt.Sprintf("%s=%s", method, bin))
		}
	}

	return strings.Join(entries, ";")
}

// TorrcTransportLines builds ClientTransportPlugin torrc directives for the enabled transports.
func (r *Registry) TorrcTransportLines(lyrebirdBin, conjureBin, selfBin string) []string {
	lyrebirdExec := cmp.Or(strings.TrimSpace(lyrebirdBin), strings.TrimSpace(selfBin))
	conjureExec := cmp.Or(strings.TrimSpace(conjureBin), strings.TrimSpace(selfBin))

	var lyrebirdMethods []string
	var hasConjure bool

	for _, method := range r.order {
		if method == conjureMethodName {
			hasConjure = true
		} else {
			lyrebirdMethods = append(lyrebirdMethods, method)
		}
	}

	var lines []string
	if len(lyrebirdMethods) > 0 && lyrebirdExec != "" {
		if lyrebirdExec == conjureExec && hasConjure {
			allMethods := append(slices.Clone(lyrebirdMethods), conjureMethodName)
			return []string{fmt.Sprintf("ClientTransportPlugin %s exec %s", strings.Join(allMethods, ","), lyrebirdExec)}
		}
		lines = append(lines, fmt.Sprintf("ClientTransportPlugin %s exec %s", strings.Join(lyrebirdMethods, ","), lyrebirdExec))
	}

	if hasConjure && conjureExec != "" {
		lines = append(lines, fmt.Sprintf("ClientTransportPlugin %s exec %s", conjureMethodName, conjureExec))
	}

	return lines
}
