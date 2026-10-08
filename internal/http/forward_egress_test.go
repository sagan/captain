package http

import (
	"context"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
	"reflect"
	"testing"
)

func TestForwardIPv6APIAndHealthRoundTrip(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/forwards"
	for _, backend := range []string{"", "nft", "realm"} {
		f := store.NodeForward{Forward: spec.Forward{Tag: "v6", Protocol: "udp", Port: 1081, Target: "[2001:db8::10]:1081", Backend: backend}}
		if backend == "realm" {
			f.Balance = "roundrobin"
		}
		if backend != "nft" {
			f.Targets = []spec.ForwardTarget{{Target: "[2001:db8::20]:1081"}}
		}
		code, b, _ := r.c.do("PUT", path, map[string]any{"Forwards": []store.NodeForward{f}}, nil)
		if code != 200 {
			t.Fatalf("IPv6 %s: %d %s", backend, code, b)
		}
		got, err := r.st.NodeForwards(context.Background(), r.nodeID)
		if err != nil || got[0].Target != f.Target {
			t.Fatal("IPv6 target lost", got, err)
		}
		f.Listen = "192.0.2.10"
		if backend == "nft" {
			if code, _, _ := r.c.do("PUT", path, map[string]any{"Forwards": []store.NodeForward{f}}, nil); code != 400 {
				t.Fatal("cross-family nft accepted", code)
			}
		}
	}
	report := agentproto.Report{Forwards: []agentproto.ForwardStatus{{Tag: "v6", Up: true, Health: "unknown", ProbeProtocol: "none", Targets: []agentproto.ForwardTargetStatus{{Target: "[2001:db8::10]:1081", Up: true, Health: "unknown", ProbeProtocol: "none"}}}}}
	if code, b, _ := r.agent.do("POST", "/api/agent/report", report, nil); code != 200 && code != 204 {
		t.Fatalf("report: %d %s", code, b)
	}
	_, b, _ := r.c.do("GET", path, nil, nil)
	got := mustJSON[struct {
		Status map[string]store.ForwardStatus `json:"status"`
	}](t, b).Status["v6"]
	if got.Health != "unknown" || got.ProbeProtocol != "none" || len(got.Targets) != 1 || got.Targets[0].Health != "unknown" {
		t.Fatal("health metadata lost", got)
	}
	// A subsequent old-agent report must clear metadata, not preserve a stale success.
	if err := r.st.UpsertForwardStatus(context.Background(), r.nodeID, agentproto.ForwardStatus{Tag: "v6", Up: false}); err != nil {
		t.Fatal(err)
	}
	statuses, _ := r.st.ForwardStatuses(context.Background(), r.nodeID)
	if statuses["v6"].Health != "" {
		t.Fatal("old report retained new metadata")
	}
}

func TestNodeEgressPolicyPermissionsCompatibilityAndState(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	path := "/api/admin/settings/nodes/" + itoa(r.nodeID) + "/egress"
	list := []spec.EgressUpstream{{CIDR: "10.10.0.2", Protocol: "tcp", Port: 1080}}
	body := map[string]any{"upstreams": list}
	if code, _, _ := r.c.do("PUT", path, body, nil); code != 409 {
		t.Fatal("old node accepted unsupported policy", code)
	}
	if err := r.st.TouchNode(ctx, r.nodeID, "v0.62.0", "", spec.SystemStatus{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if code, b, _ := r.c.do("PUT", path, body, nil); code != 200 {
		t.Fatalf("save: %d %s", code, b)
	}
	for _, bad := range []any{map[string]any{}, map[string]any{"upstreams": nil}, map[string]any{"upstreams": []spec.EgressUpstream{{CIDR: "example.com", Protocol: "tcp", Port: 1080}}}} {
		if code, _, _ := r.c.do("PUT", path, bad, nil); code != 400 {
			t.Fatal("bad/omitted policy accepted", code)
		}
	}
	if code, b, _ := r.c.do("PUT", "/api/admin/nodes/"+itoa(r.nodeID)+"/routing", map[string]any{}, nil); code != 200 {
		t.Fatalf("legacy routing save: %d %s", code, b)
	}
	_, b, _ := r.agent.do("GET", "/api/agent/state", nil, nil)
	state := mustJSON[agentproto.State](t, b)
	if !reflect.DeepEqual(state.Node.EgressUpstreams, list) || state.Node.AllowPrivateDest || len(state.Node.PrivateDestAllow) != 0 {
		t.Fatal("wrong downlink policy", state.Node.EgressUpstreams)
	}
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err := r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		for _, method := range []string{"GET", "PUT"} {
			if code, _, _ := c.do(method, path, body, nil); code != 403 {
				t.Fatal("unauthorized policy access", role, method, code)
			}
		}
	}
	if code, _, _ := r.c.do("PUT", path, map[string]any{"upstreams": []spec.EgressUpstream{}}, nil); code != 200 {
		t.Fatal("clear failed", code)
	}
	got, err := r.st.NodeEgressUpstreams(ctx, r.nodeID)
	if err != nil || len(got) != 0 {
		t.Fatal("policy not cleared", got, err)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/settings/nodes/999999/egress", nil, nil); code != 404 {
		t.Fatal("missing node", code)
	}
}
