package http

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/service"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestWireGuardOptionsCompatibilityAndForwarding(t *testing.T) {
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
	transit, err := r.st.NodeByID(ctx, r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	transit.PublicAddr = "198.51.100.20"
	if err := r.st.UpdateNode(ctx, transit); err != nil {
		t.Fatal(err)
	}
	created := mustJSON[map[string]any](t, call(r.c, "POST", "/api/admin/nodes", map[string]any{"Name": "WG options exit", "PublicAddr": "203.0.113.30"}, 200))
	exitID := int64(created["id"].(float64))
	if err := r.st.ResetPairCode(ctx, exitID, "wg-options-pair", time.Minute); err != nil {
		t.Fatal(err)
	}
	const exitToken = "wg-options-exit-token"
	if _, err := r.st.RedeemPairCode(ctx, "wg-options-pair", auth.SHA256Hex(exitToken), "test", "v0.74.0", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	exitAgent := &client{t: t, srv: r.srv, token: exitToken}
	cores := map[string]agentproto.CoreStatus{"xray": {Capabilities: &spec.CoreCapabilities{WireGuardReverse: true, PrivateAccess: true, Protocols: []spec.Protocol{spec.VLESS}}}}
	for _, id := range []int64{r.nodeID, exitID} {
		if err := r.st.TouchNode(ctx, id, "v0.74.0", "", spec.SystemStatus{}, cores, nil); err != nil {
			t.Fatal(err)
		}
	}
	report := func(id int64, rep *agentproto.WireGuardReport) {
		t.Helper()
		token := exitToken
		if id == r.nodeID {
			token = r.agent.token
		}
		if err := r.st.SetWireGuardReport(ctx, id, auth.SHA256Hex(token), rep); err != nil {
			t.Fatal(err)
		}
	}
	old := &agentproto.WireGuardReport{Available: true}
	report(r.nodeID, old)
	report(exitID, old)
	path := "/api/admin/settings/nodes/" + itoa(exitID) + "/wireguard-connections"
	input := map[string]any{"transit_id": r.nodeID, "name": "WG options", "port": 18443, "tunnel_port": 51820, "endpoint": "198.51.100.20:51820", "mtu": 1380, "server_name": "www.example.com", "enabled": true}
	for _, fields := range []map[string]any{{"domain_preference": "ipv4"}, {"auto_forwarding": true}} {
		payload := maps.Clone(input)
		maps.Copy(payload, fields)
		call(r.c, "POST", path, payload, 409)
	}
	response := mustJSON[map[string]any](t, call(r.c, "POST", path, input, 200))
	id := response["id"].(string)
	readLink := func() store.WireGuardLink {
		t.Helper()
		links, err := r.st.WireGuardLinks(ctx, exitID)
		if err != nil || len(links) != 1 {
			t.Fatalf("read link: %d links, %v", len(links), err)
		}
		return links[0]
	}
	link := readLink()
	if link.DomainPreference != "" || link.AutoForwarding {
		t.Fatal("legacy client creation changed routing or adopted firewall management")
	}
	update := func(fields map[string]any, want int) {
		t.Helper()
		payload := maps.Clone(input)
		payload["version"] = link.Version
		maps.Copy(payload, fields)
		call(r.c, "PUT", path+"/"+id, payload, want)
		link = readLink()
	}
	for _, fields := range []map[string]any{{"domain_preference": "ipv4"}, {"domain_preference": "ipv6"}, {"auto_forwarding": true}} {
		update(fields, 409)
	}
	update(map[string]any{"domain_preference": "automatic"}, 400)
	newA := &agentproto.WireGuardReport{Available: true, Applied: true, DomainPreferenceSupported: true, Links: []agentproto.WireGuardLinkStatus{{Slot: link.Slot, Connected: true}}}
	newB := &agentproto.WireGuardReport{Available: true, Applied: true, ForwardingSupported: true, Forwarding: &agentproto.WireGuardForwardingStatus{State: "synced", Backend: "ufw", CheckedAt: time.Now().Unix()}}
	report(r.nodeID, newA)
	update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 409)
	report(r.nodeID, old)
	report(exitID, newB)
	update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 409)
	report(r.nodeID, newA)
	update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 200)
	for _, fields := range []map[string]any{{}, {"domain_preference": nil, "auto_forwarding": nil}} {
		update(fields, 200)
		if link.DomainPreference != "ipv4" || !link.AutoForwarding {
			t.Fatal("omitted or null options erased saved intent")
		}
	}
	version := link.Version
	update(map[string]any{"domain_preference": "unknown", "auto_forwarding": false}, 400)
	if link.Version != version || link.DomainPreference != "ipv4" || !link.AutoForwarding {
		t.Fatal("invalid preference partially changed the connection")
	}
	// An administrator can return to the legacy manual behavior after a downgrade.
	report(r.nodeID, old)
	report(exitID, old)
	update(map[string]any{"domain_preference": "", "auto_forwarding": false}, 200)
	if link.DomainPreference != "" || link.AutoForwarding {
		t.Fatal("explicit empty/false options did not clear saved intent")
	}
	report(r.nodeID, newA)
	report(exitID, newB)
	update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 200)
	ib, err := r.st.InboundByID(ctx, link.UserInboundID)
	if err != nil {
		t.Fatal(err)
	}
	state := &service.AgentState{Store: r.st}
	assertDelivery := func(want bool) {
		t.Helper()
		for _, nodeID := range []int64{r.nodeID, exitID} {
			n, err := r.st.NodeByID(ctx, nodeID)
			if err != nil {
				t.Fatal(err)
			}
			st, err := state.Build(ctx, n, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if (len(st.Node.WireGuardLinks) == 1) != want || len(st.Node.WireGuardLinks) > 1 {
				t.Fatalf("node %d: got %d transport links, delivery wanted=%v", nodeID, len(st.Node.WireGuardLinks), want)
			}
			if want {
				c := st.Node.WireGuardLinks[0]
				if nodeID == r.nodeID && (c.DomainPreference != "ipv4" || c.AutoForwarding || c.Exit) {
					t.Fatal("A did not receive its routing option only")
				}
				if nodeID == exitID && (!c.AutoForwarding || c.DomainPreference != "" || !c.Exit) {
					t.Fatal("B did not receive its forwarding option only")
				}
			}
			found, ordinary := false, false
			for _, candidate := range st.Node.Inbounds {
				found = found || candidate.Tag == ib.Tag || candidate.WireGuardSlot == link.Slot
				ordinary = ordinary || candidate.Tag == "in"
			}
			if nodeID == r.nodeID && (found != want || !ordinary) {
				t.Fatalf("A: managed inbound=%v want %v; ordinary inbound=%v", found, want, ordinary)
			}
			if nodeID == exitID && found {
				t.Fatal("B received subscriber inbound")
			}
		}
	}
	assertDelivery(true)
	for _, nodeID := range []int64{r.nodeID, exitID} {
		report(nodeID, old)
		assertDelivery(false)
		if nodeID == r.nodeID {
			report(nodeID, newA)
		} else {
			report(nodeID, newB)
		}
		assertDelivery(true)
		if _, err := r.st.DB().ExecContext(ctx, `UPDATE nodes SET wireguard_reported_at=? WHERE id=?`, time.Now().Add(-5*time.Minute).Unix(), nodeID); err != nil {
			t.Fatal(err)
		}
		assertDelivery(false)
		update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 409)
		if nodeID == r.nodeID {
			report(nodeID, newA)
		} else {
			report(nodeID, newB)
		}
	}
	// Obtain exactly the server's desired revisions so connected is a real
	// baseline, rather than a pending state that would make failure tests vacuous.
	update(map[string]any{"domain_preference": "ipv4", "auto_forwarding": true}, 200)
	for _, endpoint := range []struct {
		id int64
		c  *client
	}{{r.nodeID, r.agent}, {exitID, exitAgent}} {
		st := mustJSON[agentproto.State](t, call(endpoint.c, "GET", "/api/agent/state", nil, 200))
		if _, err := r.st.DB().ExecContext(ctx, `UPDATE nodes SET applied_revision=? WHERE id=?`, st.Revision, endpoint.id); err != nil {
			t.Fatal(err)
		}
	}
	type connectionView struct {
		State      string                                `json:"state"`
		Forwarding *agentproto.WireGuardForwardingStatus `json:"forwarding"`
	}
	view := func() connectionView {
		t.Helper()
		rows := mustJSON[[]connectionView](t, call(r.c, "GET", path, nil, 200))
		if len(rows) != 1 {
			t.Fatalf("got %d connection views", len(rows))
		}
		return rows[0]
	}
	if v := view(); v.State != "connected" || v.Forwarding == nil || v.Forwarding.State != "synced" {
		t.Fatalf("connected baseline missing: %+v", v)
	}
	for _, failure := range []string{"error", "unsupported"} {
		newB.Forwarding.State = failure
		report(exitID, newB)
		if v := view(); v.State != "blocked" || v.Forwarding == nil || v.Forwarding.State != failure {
			t.Fatalf("%s forwarding presented as ready: %+v", failure, v)
		}
	}
	newB.Forwarding.State = "synced"
	for _, checkedAt := range []int64{0, time.Now().Add(-5 * time.Minute).Unix(), time.Now().Add(time.Minute).Unix()} {
		newB.Forwarding.CheckedAt = checkedAt
		report(exitID, newB)
		if v := view(); v.Forwarding != nil {
			t.Fatalf("invalid check time %d survived a fresh heartbeat: %+v", checkedAt, v.Forwarding)
		}
	}
	newB.Forwarding.CheckedAt = time.Now().Unix()
	report(exitID, newB)
	if _, err := r.st.DB().ExecContext(ctx, `UPDATE nodes SET wireguard_reported_at=? WHERE id=?`, time.Now().Add(-5*time.Minute).Unix(), exitID); err != nil {
		t.Fatal(err)
	}
	if v := view(); v.State == "connected" || (v.Forwarding != nil && v.Forwarding.State == "synced") {
		t.Fatalf("stale B report presented as ready: %+v", v)
	}
	// A failure on an automatically managed link must not be attributed to
	// a connection whose existing firewall rules are managed by the operator.
	report(exitID, newB)
	update(map[string]any{"auto_forwarding": false}, 200)
	newB.Forwarding.State = "error"
	report(exitID, newB)
	if v := view(); v.State == "blocked" || v.Forwarding == nil || v.Forwarding.State != "manual" {
		t.Fatalf("node-wide automatic failure affected a manual connection: %+v", v)
	}
}
