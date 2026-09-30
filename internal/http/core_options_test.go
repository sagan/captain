package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestCoreOptionsAndInboundValidation(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/core-options?protocol=vless"
	anon := &client{t: t, srv: r.srv}
	if code, _, _ := anon.do("GET", path, nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", code)
	}
	code, b, _ := r.c.do("GET", path, nil, nil)
	if code != 200 || mustJSON[spec.CoreOptions](t, b).InventoryKnown {
		t.Fatalf("unreported node: %d %s", code, b)
	}
	// Old reports cannot turn Running=false into a disabled option.
	code, b, _ = r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"xray": {Running: false}}}, nil)
	if code != 200 {
		t.Fatalf("legacy report: %d %s", code, b)
	}
	_, b, _ = r.c.do("GET", path, nil, nil)
	if mustJSON[spec.CoreOptions](t, b).InventoryKnown {
		t.Fatal("legacy report became authoritative")
	}
	sb, xr := spec.CapabilitiesForCore("singbox"), spec.CapabilitiesForCore("xray")
	code, b, _ = r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{
		"singbox": {Capabilities: &sb, Priority: 1},
		"xray":    {Capabilities: &xr, Priority: 0, Running: true, Inbounds: []string{"in"}},
	}}, nil)
	if code != 200 {
		t.Fatalf("capability report: %d %s", code, b)
	}
	_, b, _ = r.c.do("GET", path, nil, nil)
	p := mustJSON[spec.CoreOptions](t, b)
	if !p.InventoryKnown || p.AutoCore != "xray" || p.Options[0].Available == nil || !*p.Options[0].Available {
		t.Fatalf("stopped enabled core/priority: %s", b)
	}
	// The report round-trip retains successfully applied inbound assignments.
	_, b, _ = r.c.do("GET", "/api/admin/nodes/"+itoa(r.nodeID), nil, nil)
	d := mustJSON[struct {
		Status struct {
			Cores map[string]agentproto.CoreStatus `json:"cores"`
		} `json:"status"`
	}](t, b)
	if len(d.Status.Cores["xray"].Inbounds) != 1 {
		t.Fatalf("lost assignment: %s", b)
	}
	create := "/api/admin/nodes/" + itoa(r.nodeID) + "/inbounds"
	for _, tc := range []struct {
		core, protocol string
		settings       map[string]any
		want           int
	}{
		{"singbox", "vless", map[string]any{"transport": map[string]any{"type": "xhttp"}}, 400},
		{"mita", "mieru", map[string]any{}, 400},    // supported but disabled on this node
		{"singbox", "vless", map[string]any{}, 200}, // idle is available
	} {
		code, b, _ = r.c.do("POST", create, map[string]any{"Tag": "new-" + tc.core, "Protocol": tc.protocol, "Core": tc.core, "Port": 24443, "Settings": tc.settings}, nil)
		if code != tc.want {
			t.Fatalf("%s/%s: %d %s", tc.core, tc.protocol, code, b)
		}
		if code == 200 {
			created := mustJSON[map[string]any](t, b)
			id := int64(created["ID"].(float64))
			code, b, _ = r.c.do("PATCH", "/api/admin/inbounds/"+itoa(id), map[string]any{"Settings": map[string]any{"transport": map[string]any{"type": "xhttp"}}}, nil)
			if code != 400 {
				t.Fatalf("incompatible edit: %d %s", code, b)
			}
			stored, err := r.st.InboundByID(context.Background(), id)
			if err != nil || stored.Core != "singbox" || stored.Spec().TransportType() != "tcp" {
				t.Fatalf("rejected edit changed inbound: %+v %v", stored, err)
			}
		}
	}
	_, b, _ = r.c.do("GET", "/api/admin/nodes/"+itoa(r.nodeID)+"/core-options?protocol=snell&snell_obfs=tls", nil, nil)
	p = mustJSON[spec.CoreOptions](t, b)
	if p.AutoCore != "" || !p.Options[4].Compatible || *p.Options[4].Available {
		t.Fatalf("disabled compatible Snell: %s", b)
	}
	// Offline inventories no longer grey out a core that might have been
	// enabled while disconnected. The node revalidates on its next apply.
	_, err := r.st.DB().ExecContext(context.Background(), "UPDATE nodes SET last_seen_at = ? WHERE id = ?", time.Now().Add(-10*time.Minute), r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	_, b, _ = r.c.do("GET", path, nil, nil)
	if p := mustJSON[spec.CoreOptions](t, b); p.InventoryKnown || p.AutoCore != "" {
		t.Fatalf("offline preview claimed availability: %s", b)
	}

}
