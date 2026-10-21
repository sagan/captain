package service

import (
	"encoding/json"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/bosun/pkg/subscription"
	"gopkg.in/yaml.v3"
)

func chainFixture() []PublishedLine {
	return []PublishedLine{
		{EntryID: 1, Line: subscription.Line{Name: "Front renamed", Host: "front.example.com", Port: 443, UUID: "user", Inbound: spec.Inbound{Protocol: spec.VLESS}}},
		{EntryID: 2, DialerEntryID: 1, Line: subscription.Line{Name: "Home via front", Host: "exit.example.com", Port: 8388, Password: "secret", Inbound: spec.Inbound{Protocol: spec.Shadowsocks, Cipher: "aes-128-gcm"}, Extra: map[string]any{"tfo": true, "dialer-proxy": "stale-name", "detour": "stale-name"}}},
	}
}

func TestClientChainRendersCurrentNameAndKeepsLandingEndpoint(t *testing.T) {
	for _, format := range []string{"clash", "stash", "singbox"} {
		t.Run(format, func(t *testing.T) {
			input := chainFixture()
			lines, err := ClientLines(input, format)
			if err != nil || len(lines) != 2 {
				t.Fatalf("lines=%+v err=%v", lines, err)
			}
			body, err := subscription.Pick(format, "").Render(lines, subscription.Account{})
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if format == "singbox" {
				err = json.Unmarshal(body, &doc)
			} else {
				err = yaml.Unmarshal(body, &doc)
			}
			if err != nil {
				t.Fatal(err)
			}
			collection, name, port, dialer := "proxies", "name", "port", "dialer-proxy"
			if format == "singbox" {
				collection, name, port, dialer = "outbounds", "tag", "server_port", "detour"
			}
			found := false
			for _, raw := range doc[collection].([]any) {
				p := raw.(map[string]any)
				if p[name] != "Home via front" {
					continue
				}
				found = true
				if p[dialer] != "Front renamed" || p["server"] != "exit.example.com" || p["tfo"] != true {
					t.Fatalf("proxy=%v", p)
				}
				if p[port] != 8388 && p[port] != float64(8388) {
					t.Fatalf("port=%v", p[port])
				}
			}
			if !found {
				t.Fatal("chain missing from rendered document")
			}
			if input[1].Extra["dialer-proxy"] != "stale-name" {
				t.Fatal("mutated stored extra fields")
			}
		})
	}
}

func TestClientChainFailsClosedForMissingAmbiguousAndUnsupportedFront(t *testing.T) {
	for _, tc := range []struct {
		name, format string
		change       func([]PublishedLine) []PublishedLine
	}{
		{"missing", "clash", func(v []PublishedLine) []PublishedLine { return v[1:] }},
		{"duplicate", "clash", func(v []PublishedLine) []PublishedLine { return append(v, PublishedLine{Line: v[0].Line}) }},
		{"cycle", "clash", func(v []PublishedLine) []PublishedLine { v[0].DialerEntryID = 2; return v }},
		{"raw-name", "clash", func(v []PublishedLine) []PublishedLine { v[0].Extra = map[string]any{"name": "different"}; return v }},
		{"raw-dialer", "clash", func(v []PublishedLine) []PublishedLine {
			v[0].Extra = map[string]any{"dialer-proxy": "unmanaged"}
			return v
		}},
		{"unsupported", "singbox", func(v []PublishedLine) []PublishedLine { v[0].Inbound.Protocol = spec.Mieru; return v }},
		{"unsupported-name-collision", "singbox", func(v []PublishedLine) []PublishedLine {
			v[0].Inbound.Protocol = spec.Mieru
			other := v[0].Line
			other.Inbound.Protocol = spec.VLESS
			return append(v, PublishedLine{Line: other})
		}},
		{"unsupported-format", "surge", func(v []PublishedLine) []PublishedLine { return v }},
		{"standalone-uri", "uri", func(v []PublishedLine) []PublishedLine { return v }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := ClientLines(tc.change(chainFixture()), tc.format)
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range lines {
				if l.Name == "Home via front" {
					t.Fatal("dependent node leaked as direct or invalid profile")
				}
			}
		})
	}
}

func TestClientChainTransitiveDependency(t *testing.T) {
	lines := chainFixture()
	lines = append(lines, PublishedLine{EntryID: 3, DialerEntryID: 2, Line: subscription.Line{Name: "Third", Host: "third.example.com", Port: 443, Inbound: spec.Inbound{Protocol: spec.VLESS}}})
	got, err := ClientLines(lines, "clash")
	if err != nil || len(got) != 3 || got[2].Extra["dialer-proxy"] != "Home via front" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	got, err = ClientLines(lines[1:], "clash")
	if err != nil || len(got) != 0 {
		t.Fatalf("dependent descendants leaked: %v %v", got, err)
	}
}
