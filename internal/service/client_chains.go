package service

import (
	"encoding/json"
	"maps"

	"github.com/zeptop-dev/bosun/pkg/subscription"
	"github.com/zeptop-dev/captain/internal/store"
	"gopkg.in/yaml.v3"
)

// PublishedLine is internal presentation metadata. It is never merged into a
// client configuration or sent to an agent.
type PublishedLine struct {
	subscription.Line
	EntryID       int64
	DialerEntryID int64
}

// AvailablePublications is used for the portal's format-neutral catalogue.
// A client chain has no standalone share URI, but remains visible here when
// the user can access every required node.
func AvailablePublications(lines []PublishedLine) []PublishedLine {
	byID := map[int64]PublishedLine{}
	for _, l := range lines {
		if l.EntryID != 0 {
			byID[l.EntryID] = l
		}
	}
	out := make([]PublishedLine, 0, len(lines))
	for _, l := range lines {
		current, seen, valid := l, map[int64]bool{}, true
		for depth := 1; current.DialerEntryID != 0; depth++ {
			if depth >= store.MaxClientChainDepth || seen[current.EntryID] {
				valid = false
				break
			}
			seen[current.EntryID] = true
			next, ok := byID[current.DialerEntryID]
			if !ok {
				valid = false
				break
			}
			current = next
		}
		if valid {
			out = append(out, l)
		}
	}
	return out
}

// ClientLines resolves references only within the already authorized lines.
// Missing, hidden, unsupported or ambiguous dependencies remove the dependent
// node; they never turn it into an ordinary direct node.
func ClientLines(lines []PublishedLine, format string) ([]subscription.Line, error) {
	supportedFormat := format == "clash" || format == "stash" || format == "singbox"
	hasChain := false
	for _, l := range lines {
		hasChain = hasChain || l.DialerEntryID != 0
	}
	plain := make([]subscription.Line, 0, len(lines))
	for _, l := range lines {
		if supportedFormat || l.DialerEntryID == 0 {
			plain = append(plain, l.Line)
		}
	}
	if !supportedFormat || !hasChain {
		return plain, nil
	}

	// Use the existing renderer as the protocol/transport capability source.
	// This avoids maintaining a second protocol matrix in Captain. The built-in
	// template also reveals collisions with selector and ShadowTLS carrier names.
	data, err := subscription.Pick(format, "").Render(plain, subscription.Account{})
	if err != nil {
		return nil, err
	}
	counts, err := renderedClientNames(format, data)
	if err != nil {
		return nil, err
	}
	capabilities := map[int64]bool{}
	checked := map[int64]bool{}
	var probeErr error
	supports := func(l PublishedLine) bool {
		if checked[l.EntryID] {
			return capabilities[l.EntryID]
		}
		checked[l.EntryID] = true
		probe := l.Line
		probe.Name, probe.Extra = "captain-chain-capability", nil
		data, err := subscription.Pick(format, "").Render([]subscription.Line{probe}, subscription.Account{})
		if err != nil {
			probeErr = err
			return false
		}
		names, err := renderedClientNames(format, data)
		if err != nil {
			probeErr = err
			return false
		}
		capabilities[l.EntryID] = names[probe.Name] == 1
		return capabilities[l.EntryID]
	}
	byID := map[int64]PublishedLine{}
	for _, l := range lines {
		if l.EntryID != 0 {
			byID[l.EntryID] = l
		}
	}
	var valid func(PublishedLine, map[int64]bool, int) bool
	valid = func(l PublishedLine, seen map[int64]bool, depth int) bool {
		if depth > store.MaxClientChainDepth || seen[l.EntryID] || counts[l.Name] != 1 || !supports(l) {
			return false
		}
		// A raw name/tag override cannot silently redirect a managed reference.
		key := "name"
		if format == "singbox" {
			key = "tag"
		}
		if name, exists := l.Extra[key]; exists && name != l.Name {
			return false
		}
		if l.DialerEntryID == 0 {
			// Unmanaged dialers can point outside the authorized dependency set.
			return l.Extra["dialer-proxy"] == nil && l.Extra["detour"] == nil
		}
		if format == "singbox" && l.Inbound.ShadowTLS != nil {
			// The current shared renderer owns the ShadowTLS carrier detour.
			// Until it can chain that carrier, omit rather than overwrite it.
			return false
		}
		front, ok := byID[l.DialerEntryID]
		if !ok {
			return false
		}
		seen[l.EntryID] = true
		return valid(front, seen, depth+1)
	}
	out := make([]subscription.Line, 0, len(lines))
	for _, l := range lines {
		if l.DialerEntryID == 0 {
			out = append(out, l.Line)
			continue
		}
		if !valid(l, map[int64]bool{}, 1) {
			continue
		}
		line := l.Line
		line.Extra = maps.Clone(line.Extra)
		if line.Extra == nil {
			line.Extra = map[string]any{}
		}
		delete(line.Extra, "dialer-proxy")
		delete(line.Extra, "detour")
		// Stash treats interface-name and dialer-proxy as mutually exclusive;
		// the explicit managed chain owns how this destination is dialed.
		if format == "stash" {
			delete(line.Extra, "interface-name")
		}
		key := "dialer-proxy"
		if format == "singbox" {
			key = "detour"
		}
		line.Extra[key] = byID[l.DialerEntryID].Name
		out = append(out, line)
	}
	if probeErr != nil {
		return nil, probeErr
	}
	return out, nil
}

func renderedClientNames(format string, data []byte) (map[string]int, error) {
	counts := map[string]int{}
	if format == "singbox" {
		var doc struct {
			Outbounds []map[string]any `json:"outbounds"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		for _, p := range doc.Outbounds {
			if name, ok := p["tag"].(string); ok {
				counts[name]++
			}
		}
	} else {
		var doc struct {
			Proxies []map[string]any `yaml:"proxies"`
			Groups  []map[string]any `yaml:"proxy-groups"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		for _, p := range append(doc.Proxies, doc.Groups...) {
			if name, ok := p["name"].(string); ok {
				counts[name]++
			}
		}
	}
	return counts, nil
}
