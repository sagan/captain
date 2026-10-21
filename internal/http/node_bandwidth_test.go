package http

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeBandwidthPrivacyAndLegacyUpdates(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/probe"
	save := func(info any, want int) {
		t.Helper()
		code, body, _ := r.c.do("PUT", path, map[string]any{"Info": info, "Mode": "sum", "ResetDay": 1}, nil)
		if code != want {
			t.Fatalf("save: %d %s", code, body)
		}
	}
	settings := func(sections []string) {
		t.Helper()
		code, body, _ := r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "visibility": "public", "public_sections": sections}, nil)
		if code != 200 {
			t.Fatalf("settings: %d %s", code, body)
		}
	}
	read := func() *store.NodeBandwidth {
		t.Helper()
		p, err := r.st.NodeProbe(context.Background(), r.nodeID)
		if err != nil {
			t.Fatal(err)
		}
		return p.Info.Bandwidth
	}
	public := func(want bool) {
		t.Helper()
		anon := &client{t: t, srv: r.srv}
		code, body, _ := anon.do("GET", "/api/probe", nil, nil)
		if code != 200 {
			t.Fatalf("public: %d %s", code, body)
		}
		var out struct {
			Nodes []struct {
				ID   int64               `json:"id"`
				Info store.NodeProbeInfo `json:"info"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		for _, n := range out.Nodes {
			if n.ID == r.nodeID {
				if (n.Info.Bandwidth != nil) != want {
					t.Fatalf("public bandwidth: %+v", n.Info)
				}
				if want && (n.Info.Bandwidth.InboundMbps != 1500 || n.Info.Bandwidth.OutboundMbps != 100.5) {
					t.Fatal("rates changed")
				}
				return
			}
		}
		t.Fatal("missing public node")
	}
	settings([]string{"info"})
	b := &store.NodeBandwidth{InboundMbps: 1500, OutboundMbps: 100.5}
	save(map[string]any{"bandwidth": b}, 200)
	if got := read(); got == nil || *got != *b {
		t.Fatal("bandwidth not persisted")
	}
	public(false)
	// Older clients, including explicit null, preserve the private record.
	save(map[string]any{"note": "legacy"}, 200)
	save(map[string]any{"bandwidth": nil}, 200)
	if got := read(); got == nil || *got != *b {
		t.Fatal("legacy update erased bandwidth")
	}
	for _, invalid := range []float64{-1, 1000001} {
		save(map[string]any{"bandwidth": map[string]any{"inbound_mbps": invalid}}, 400)
	}
	if got := read(); got == nil || *got != *b {
		t.Fatal("invalid update changed record")
	}
	b.Public = true
	save(map[string]any{"bandwidth": b}, 200)
	public(true)
	settings([]string{"traffic"})
	public(false)
	settings([]string{"info"})
	// Hiding it removes the actual public payload, not just a UI label.
	b.Public = false
	save(map[string]any{"bandwidth": b}, 200)
	public(false)
	save(map[string]any{"bandwidth": map[string]any{}}, 200)
	if read() != nil {
		t.Fatal("explicit clear retained bandwidth")
	}
	public(false)
}
