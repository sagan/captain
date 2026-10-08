package http

import (
	"context"
	"github.com/zeptop-dev/captain/internal/service"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/http/admin"
)

func TestInboundPrivateAccessAPI(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	call := func(c *client, method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, code, want, b)
		}
		return b
	}
	create := "/api/admin/nodes/" + itoa(r.nodeID) + "/inbounds"
	body := map[string]any{"Tag": "private", "Protocol": "vless", "Port": 18444, "Core": "xray", "Settings": map[string]any{"private_access": map[string]any{"mode": "internal"}}}
	call(r.c, "POST", create, body, 400) // no capability report
	caps := spec.CapabilitiesForCore("xray")
	caps.PrivateAccess = false
	report := func() {
		call(r.agent, "POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"xray": {Capabilities: &caps}}}, 200)
	}
	report()
	call(r.c, "POST", create, body, 400)
	caps.PrivateAccess = true
	report()
	ib := mustJSON[domain.Inbound](t, call(r.c, "POST", create, body, 200))
	path := "/api/admin/inbounds/" + itoa(ib.ID)
	staff, err := admin.NewUser("operator-private@example.com", "password123", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.st.CreateStaff(ctx, staff); err != nil {
		t.Fatal(err)
	}
	op := &client{t: t, srv: r.srv}
	call(op, "POST", "/api/admin/login", map[string]string{"Email": staff.Email, "Password": "password123"}, 200)
	call(op, "PATCH", path, map[string]any{"Settings": map[string]any{"private_access": map[string]any{"mode": "off"}}}, 403)
	call(op, "PATCH", path, map[string]any{"Settings": map[string]any{"private_access": map[string]any{"mode": "custom", "rules": []any{map[string]any{"cidr": "10.10.0.2"}}}}}, 403)
	call(op, "POST", create, body, 403)
	// Omitted/null policy from an older client must preserve authorization.
	for _, settings := range []any{map[string]any{"no_sniff": true}, map[string]any{"private_access": nil}} {
		call(r.c, "PATCH", path, map[string]any{"Settings": settings}, 200)
		stored, err := r.st.InboundByID(ctx, ib.ID)
		if err != nil || !stored.Settings.PrivateAccess.Enabled() {
			t.Fatal("old client removed policy", err)
		}
	}
	for _, address := range []string{"127.0.0.1", "169.254.169.254", "::/0", "example.com"} {
		call(r.c, "PATCH", path, map[string]any{"Settings": map[string]any{"private_access": map[string]any{"mode": "custom", "rules": []any{map[string]any{"cidr": address}}}}}, 400)
	}
	call(r.c, "PUT", "/api/admin/nodes/"+itoa(r.nodeID)+"/overrides", map[string]string{"xray": `{"routing":{"rules":[]}}`}, 400)
	call(r.c, "PATCH", path, map[string]any{"Settings": map[string]any{"private_access": map[string]any{"mode": "off"}}}, 200)
	stored, _ := r.st.InboundByID(ctx, ib.ID)
	if stored.Settings.PrivateAccess.Enabled() {
		t.Fatal("explicit disable ignored")
	}
}

func TestReversePrivateAccessPropagationAndDowngrade(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := r.c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s: got %d want %d: %s", path, code, want, b)
		}
		return b
	}
	a, _ := r.st.NodeByID(ctx, r.nodeID)
	a.PublicAddr = "198.51.100.20"
	if err := r.st.UpdateNode(ctx, a); err != nil {
		t.Fatal(err)
	}
	created := mustJSON[map[string]any](t, call("POST", "/api/admin/nodes", map[string]any{"Name": "private-exit", "PublicAddr": "203.0.113.30"}, 200))
	exit := int64(created["id"].(float64))
	path := "/api/admin/nodes/" + itoa(exit) + "/reverse-connections"
	row := map[string]any{"transit_id": r.nodeID, "name": "Private reverse", "port": 18445, "tunnel_port": 38921, "server_name": "example.com", "enabled": true, "settings": map[string]any{"private_access": map[string]any{"mode": "internal"}}}
	draft := map[string]any{"expected": map[string]int64{}, "links": []any{row}}
	caps := spec.CapabilitiesForCore("xray")
	report := func(id int64, private bool) {
		t.Helper()
		c := caps
		c.PrivateAccess = private
		if err := r.st.TouchNode(ctx, id, "dev", "", spec.SystemStatus{}, map[string]agentproto.CoreStatus{"xray": {Capabilities: &c}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	report(r.nodeID, true)
	report(exit, false)
	call("PUT", path, draft, 400)
	report(exit, true)
	call("PUT", path, draft, 200)
	build := &service.AgentState{Store: r.st}
	b, _ := r.st.NodeByID(ctx, exit)
	state, err := build.Build(ctx, b, time.Now())
	if err != nil || len(state.Node.ReverseClients) != 1 || !state.Node.ReverseClients[0].PrivateAccess.Enabled() {
		t.Fatalf("exit policy lost: %v", err)
	}
	state, err = build.Build(ctx, a, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ib := range state.Node.Inbounds {
		if ib.Reverse == nil {
			continue
		}
		if ib.Reverse.Receiver && ib.PrivateAccess.Enabled() {
			t.Fatal("receiver acquired user permission")
		}
		if !ib.Reverse.Receiver {
			found = true
			if !ib.PrivateAccess.Enabled() {
				t.Fatal("transit policy lost")
			}
		}
	}
	if !found {
		t.Fatal("managed transit disappeared")
	}
	report(exit, false)
	state, err = build.Build(ctx, b, time.Now())
	if err != nil || len(state.Node.ReverseClients) != 0 {
		t.Fatal("downgraded exit received private client", err)
	}
	report(r.nodeID, false)
	state, err = build.Build(ctx, a, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, ib := range state.Node.Inbounds {
		if ib.PrivateAccess.Enabled() {
			t.Fatal("downgraded transit received private inbound")
		}
	}
}
