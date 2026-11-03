package bridge

import (
	"os"
	"slices"
	"strings"
)

var lyrebirdTransports = []string{
	"obfs4",
	"webtunnel",
	"meek_lite",
	"meek",
	"obfs3",
	"obfs2",
	"scramblesuit",
}

// Toggles controls which pluggable transport families are active.
type Toggles struct {
	Snowflake bool
	Lyrebird  bool
	Conjure   bool
}

// Allows reports whether the given transport protocol is permitted by the active toggles.
// An empty transport string represents a vanilla bridge and is always allowed.
func (t Toggles) Allows(transport string) bool {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "":
		return true
	case "snowflake":
		return t.Snowflake
	case "conjure":
		return t.Conjure
	default:
		return t.Lyrebird && slices.Contains(lyrebirdTransports, strings.ToLower(strings.TrimSpace(transport)))
	}
}

// ParseLines normalizes raw bridge text (newline or semicolon separated, with optional
// "Bridge " prefix and '#' comments) into deduplicated bridge lines.
func ParseLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, ";", "\n")
	var out []string

	for line := range strings.SplitSeq(normalized, "\n") {
		cleaned := cleanBridgeLine(line)
		if cleaned == "" || slices.Contains(out, cleaned) {
			continue
		}
		out = append(out, cleaned)
	}

	return out
}

// ReadFile reads and parses bridge lines from a file path.
func ReadFile(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseLines(string(data)), nil
}

// TransportOf extracts the pluggable transport name from a bridge line, or returns ""
// when the line describes a plain IP:port bridge.
func TransportOf(line string) string {
	cleaned := cleanBridgeLine(line)
	if cleaned == "" {
		return ""
	}

	first, _, _ := strings.Cut(cleaned, " ")
	if first == "" {
		return ""
	}

	ch := first[0]
	if (ch >= '0' && ch <= '9') || ch == '[' || strings.Contains(first, "=") {
		return ""
	}

	return strings.ToLower(first)
}

// FilterByToggles retains only bridge lines whose transport is enabled in toggles,
// deduplicating while preserving order.
func FilterByToggles(lines []string, toggles Toggles) []string {
	var kept []string

	for _, raw := range lines {
		for _, line := range ParseLines(raw) {
			if !toggles.Allows(TransportOf(line)) {
				continue
			}
			if !slices.Contains(kept, line) {
				kept = append(kept, line)
			}
		}
	}

	return kept
}

func cleanBridgeLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	if rest, ok := strings.CutPrefix(trimmed, "Bridge "); ok {
		trimmed = strings.TrimSpace(rest)
	} else if rest, ok := strings.CutPrefix(trimmed, "bridge "); ok {
		trimmed = strings.TrimSpace(rest)
	}

	return strings.Join(strings.Fields(trimmed), " ")
}
