package http

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/service"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestReverseWizard(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	transit, _ := r.st.NodeByID(ctx, r.nodeID)
	transit.PublicAddr = "198.51.100.20"
	if err := r.st.UpdateNode(ctx, transit); err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := r.c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, code, want, b)
		}
		return b
	}
	newNode := func(name string) int64 {
		b := call("POST", "/api/admin/nodes", map[string]any{"Name": name, "PublicAddr": "203.0.113.30"}, 200)
		return int64(mustJSON[map[string]any](t, b)["id"].(float64))
	}
	exit := newNode("exit")
	a2 := newNode("transit-two")
	path := "/api/admin/nodes/" + itoa(exit) + "/reverse-connections"
	row := func(id int64) map[string]any {
		return map[string]any{"transit_id": id, "name": "Reverse test", "port": 18445, "tunnel_port": 38921, "server_name": "www.example.com", "enabled": true}
	}
	// The second transit is NAT-only: both local listeners must map to the
	// advertised public ports, including the endpoint sent to B.
	g := mustJSON[struct {
		Ingress store.Ingress `json:"ingress"`
	}](t, call("POST", "/api/admin/nodes/"+itoa(a2)+"/ingresses", map[string]any{
		"Name": "NAT", "Kind": "nat", "EntryHost": "192.0.2.10", "RequireIngress": true,
		"PortMappings": []spec.PortMapping{{LocalFrom: 20000, LocalTo: 20009, PublicFrom: 30000}},
	}, 200)).Ingress
	natRow := row(a2)
	natRow["ingress_id"], natRow["port"], natRow["tunnel_port"] = g.ID, 20000, 20001
	draft := map[string]any{"expected": map[string]int64{}, "links": []any{row(r.nodeID), natRow}}
	// One atomic wizard writes both transits, two listeners and one entry each.
	call("PUT", path, draft, 200)
	list := mustJSON[[]map[string]any](t, call("GET", path, nil, 200))
	if len(list) != 2 {
		t.Fatal("missing links")
	}
	expected := map[string]int64{}
	for _, l := range list {
		id, ok := l["id"].(string)
		if !ok || id == "" {
			t.Fatal("link identity missing")
		}
		expected[id] = int64(l["version"].(float64))
		if l["state"] != "upgrade" {
			t.Fatal("old agent should require upgrade")
		}
	}
	call("PUT", path, draft, 409)
	for _, role := range []string{"operator", "support"} {
		u, err := admin.NewUser(role+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		staff := &client{t: t, srv: r.srv}
		if code, _, _ := staff.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil); code != 200 {
			t.Fatal("staff login", code)
		}
		for _, method := range []string{"GET", "PUT"} {
			if code, _, _ := staff.do(method, path, draft, nil); code != 403 {
				t.Fatal("reverse role boundary", role, method, code)
			}
		}
	}
	build := &service.AgentState{Store: r.st}
	n, _ := r.st.NodeByID(ctx, r.nodeID)
	st, err := build.Build(ctx, n, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, ib := range st.Node.Inbounds {
		if ib.Reverse != nil {
			t.Fatal("old agent got managed inbound")
		}
	}
	caps := spec.CapabilitiesForCore("xray")
	for _, id := range []int64{r.nodeID, a2, exit} {
		if err := r.st.TouchNode(ctx, id, "dev", "", spec.SystemStatus{}, map[string]agentproto.CoreStatus{"xray": {Capabilities: &caps}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	st, err = build.Build(ctx, n, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var control spec.Inbound
	for _, ib := range st.Node.Inbounds {
		if ib.Reverse != nil && ib.Reverse.Receiver {
			control = ib
		}
	}
	if control.Reverse == nil || len(control.EffectiveUsers([]spec.User{{Name: "customer"}})) != 0 {
		t.Fatal("control accepted customer list")
	}
	nb, _ := r.st.NodeByID(ctx, exit)
	bst, err := build.Build(ctx, nb, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(bst.Node.ReverseClients) != 2 {
		t.Fatal("missing active reverse clients")
	}
	natClient := false
	for _, c := range bst.Node.ReverseClients {
		if c.Host == "192.0.2.10" {
			natClient = c.Port == 30001
		}
		if c.TLS.Reality.PrivateKey != "" {
			t.Fatal("server private key sent to exit")
		}
	}
	if !natClient {
		t.Fatal("NAT tunnel public endpoint not resolved")
	}
	for _, l := range list {
		if int64(l["transit_id"].(float64)) == a2 && (l["host"] != "192.0.2.10" || l["public_port"] != float64(30000)) {
			t.Fatal("NAT subscription endpoint not resolved")
		}
	}
	// Routing overrides must not replace the fail-closed dynamic outbound.
	for _, nid := range []int64{exit, r.nodeID} {
		call("PUT", "/api/admin/nodes/"+itoa(nid)+"/overrides", map[string]string{"xray": `{"routing":null}`}, 409)
		call("PUT", "/api/admin/nodes/"+itoa(nid)+"/routing", map[string]any{"Outbounds": []any{map[string]any{"tag": "reverse-collision", "protocol": "freedom"}}}, 409)
	}
	links, _ := r.st.ReverseLinks(ctx, exit)
	credentials := map[string]string{}
	for _, l := range links {
		c, _ := r.st.InboundByID(ctx, l.ReceiverInboundID)
		credentials[l.ID] = c.Settings.Reverse.UUID
	}
	// Force a failure after the first resource update inside the transaction.
	first := links[0]
	u, _ := r.st.InboundByID(ctx, first.UserInboundID)
	c, _ := r.st.InboundByID(ctx, first.ReceiverInboundID)
	e, _ := r.st.EntryByID(ctx, first.EntryID)
	originalPort := u.Port
	u.Port++
	c.NodeID = 999999 // Foreign-key violation on the second write.
	if err := r.st.SaveReverseLinks(ctx, exit, expected, []store.ReverseWrite{{Link: first, User: *u, Receiver: *c, Entry: *e}}); err == nil {
		t.Fatal("expected transactional failure")
	}
	rolledBack, _ := r.st.InboundByID(ctx, first.UserInboundID)
	if rolledBack.Port != originalPort {
		t.Fatal("partial transaction persisted")
	}
	for _, l := range links {
		call("DELETE", "/api/admin/inbounds/"+itoa(l.UserInboundID), nil, 409)
		call("DELETE", "/api/admin/entries/"+itoa(l.EntryID), nil, 409)
		call("POST", "/api/admin/entries", map[string]any{"Name": "bad", "InboundID": l.ReceiverInboundID, "DisplayHost": "example.com", "DisplayPort": 443}, 400)
	}
	// A failed per-link edit must leave every resource and credential intact.
	list[0]["port"] = list[0]["tunnel_port"]
	call("PUT", path, map[string]any{"expected": expected, "links": list}, 409)
	list = mustJSON[[]map[string]any](t, call("GET", path, nil, 200))
	// Rename/edit one connection, remove the other, retain stable credentials.
	list[0]["name"] = "Renamed reverse"
	call("PUT", path, map[string]any{"expected": expected, "links": list[:1]}, 200)
	remaining, _ := r.st.ReverseLinks(ctx, exit)
	if len(remaining) != 1 {
		t.Fatal("remove transit failed")
	}
	ib, _ := r.st.InboundByID(ctx, remaining[0].ReceiverInboundID)
	if ib.Settings.Reverse.UUID != credentials[remaining[0].ID] {
		t.Fatal("identity lost")
	}
	raw, _ := json.Marshal(mustJSON[[]map[string]any](t, call("GET", path, nil, 200)))
	if strings.Contains(string(raw), ib.Settings.Reverse.UUID) {
		t.Fatal("tunnel credential exposed by wizard")
	}
	// Deleting B also removes the owned listeners on A, leaving its ordinary inbound.
	call("DELETE", "/api/admin/nodes/"+itoa(exit), nil, 200)
	for _, id := range []int64{r.nodeID, a2} {
		ibs, _ := r.st.AllInboundsByNode(ctx, id)
		for _, ib := range ibs {
			if ib.Settings.Reverse != nil {
				t.Fatal("orphan managed inbound")
			}
		}
	}
	ordinary, _ := r.st.AllInboundsByNode(ctx, r.nodeID)
	if len(ordinary) != 1 {
		t.Fatal("ordinary inbound lost")
	}
}
