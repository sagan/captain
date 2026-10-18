package http

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeAddressAPICompatibilityAndPairReset(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	path := "/api/admin/settings/nodes/" + itoa(r.nodeID) + "/addresses"
	call := func(c *client, method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s code %d want %d: %s", method, path, code, want, b)
		}
		return b
	}
	call(r.c, "PUT", path, map[string]any{"auto_ipv4": true}, 409)
	// An old node without the additive field still reports normally.
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{}, 200)
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{PublicAddresses: &agentproto.PublicAddresses{SampledAt: time.Now().Unix(), IPv4: "192.0.2.10"}}, 200)
	call(r.c, "PUT", path, map[string]any{"auto_ipv4": true}, 200)
	call(r.c, "PUT", path, map[string]any{}, 200)
	status := mustJSON[store.NodeAddresses](t, call(r.c, "GET", "/api/admin/nodes/"+itoa(r.nodeID)+"/addresses", nil, 200))
	if !status.AutoIPv4 || status.AutoIPv6 || status.Observed.IPv4.Address != "192.0.2.10" {
		t.Fatalf("policy/observation %+v", status)
	}
	// Node credentials cannot alter the administrative follow policy.
	call(r.agent, "PUT", path, map[string]any{"auto_ipv4": false}, 401)
	// The production repair handler owns Topology: this also detects accidental
	// nested locking in the store reset routine.
	call(r.c, "POST", "/api/admin/nodes/"+itoa(r.nodeID)+"/repair", nil, 200)
	out, err := r.st.NodeAddresses(ctx, r.nodeID)
	if err != nil || out.Observed.ReceivedAt != 0 {
		t.Fatalf("stale observation after repair: %+v %v", out, err)
	}
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{}, 401)
}

func TestCreateNodeAddressPolicies(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name         string
		body         map[string]any
		auto4, auto6 bool
		want         int
	}{
		{"automatic", map[string]any{"Name": "auto", "AutoIPv4": true, "AutoIPv6": true}, true, true, 200},
		{"fixed-v4", map[string]any{"Name": "nat", "PublicAddr": "198.51.100.20", "AutoIPv4": false, "AutoIPv6": true}, false, true, 200},
		{"legacy", map[string]any{"Name": "legacy"}, false, false, 200},
		{"ambiguous", map[string]any{"Name": "bad", "PublicAddr": "198.51.100.20", "AutoIPv4": true}, false, false, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, b, _ := r.c.do("POST", "/api/admin/nodes", tc.body, nil)
			if code != tc.want {
				t.Fatalf("code %d: %s", code, b)
			}
			if code != 200 {
				return
			}
			n := mustJSON[struct{ ID int64 }](t, b)
			got, err := r.st.NodeAddresses(ctx, n.ID)
			if err != nil || got.AutoIPv4 != tc.auto4 || got.AutoIPv6 != tc.auto6 {
				t.Fatalf("policy %+v: %v", got, err)
			}
			if err := r.st.FillPublicAddr(ctx, n.ID, "192.0.2.10"); err != nil {
				t.Fatal(err)
			}
			node, err := r.st.NodeByID(ctx, n.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.auto4 && node.PublicAddr != "" {
				t.Fatal("pair source bypassed automatic confirmation")
			}
			if tc.name == "fixed-v4" && node.PublicAddr != "198.51.100.20" {
				t.Fatal("fixed entry changed")
			}
			if tc.name == "legacy" && node.PublicAddr != "192.0.2.10" {
				t.Fatal("legacy pair fallback changed")
			}
		})
	}
}
