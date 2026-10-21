package http

import (
	"context"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestManagedLinesAPIAndAgentDelivery(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	call := func(c *client, method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: want %d got %d %s", method, path, want, code, b)
		}
		return b
	}
	call(r.c, "PATCH", "/api/admin/nodes/"+itoa(r.nodeID), map[string]any{"Name": "Landing", "PublicAddr": "203.0.113.30"}, 200)
	ibs, _ := r.st.AllInboundsByNode(ctx, r.nodeID)
	ib := ibs[0]
	var relays []int64
	var agents []*client
	for i, ip := range []string{"192.0.2.10", "198.51.100.20"} {
		n := mustJSON[map[string]any](t, call(r.c, "POST", "/api/admin/nodes", map[string]any{"Name": "Relay " + itoa(int64(i)), "PublicAddr": ip}, 200))
		id := int64(n["id"].(float64))
		relays = append(relays, id)
		a := &client{t: t, srv: r.srv}
		p := mustJSON[agentproto.PairResponse](t, call(a, "POST", "/api/agent/pair", agentproto.PairRequest{Code: n["pair_code"].(string), Version: "v0.75.0"}, 200))
		a.token = p.Token
		agents = append(agents, a)
	}
	opts := mustJSON[map[string]string](t, call(r.c, "GET", "/api/admin/lines/options?inbound_id="+itoa(ib.ID), nil, 200))
	if opts["protocol"] != "tcp" {
		t.Fatal(opts)
	}
	input := store.ForwardLine{Name: "Two hops", Protocol: "tcp", Backend: "relay", TargetInboundID: ib.ID, Groups: []store.LineGroup{{Members: []store.LineMember{{NodeID: relays[0], Port: 18443}}}, {Members: []store.LineMember{{NodeID: relays[1], Port: 19443}}}}}
	l := mustJSON[store.ForwardLine](t, call(r.c, "POST", "/api/admin/lines", input, 200))
	path := "/api/admin/lines/" + itoa(l.ID)
	if len(l.Endpoints) != 1 || l.Endpoints[0].Host != "192.0.2.10" || len(l.Status) != 2 || l.Status[0].Health != "unknown" {
		t.Fatal("bad view", l)
	}
	for i, a := range agents {
		st := mustJSON[agentproto.State](t, call(a, "GET", "/api/agent/state", nil, 200))
		if len(st.Forwards) != 1 {
			t.Fatalf("member %d not delivered: %+v", i, st.Forwards)
		}
		want := "198.51.100.20:19443"
		if i == 1 {
			want = "203.0.113.30:443"
		}
		if st.Forwards[0].Target != want {
			t.Fatal("wrong execution path", st.Forwards)
		}
	}
	e := mustJSON[domain.Entry](t, call(r.c, "POST", "/api/admin/entries", map[string]any{"Name": "Via line", "LineID": l.ID}, 200))
	ep := "/api/admin/entries/" + itoa(e.ID)
	if e.InboundID != ib.ID || e.DisplayHost != "192.0.2.10" || e.LineMemberID == "" {
		t.Fatal("wrong subscription endpoint", e)
	}
	// Old clients omit managed fields and forwards. Neither loses ownership.
	call(r.c, "PATCH", ep, map[string]any{"Name": "Renamed", "InboundID": ib.ID, "DisplayHost": "192.0.2.10", "DisplayPort": 18443}, 200)
	call(r.c, "PUT", "/api/admin/nodes/"+itoa(relays[0])+"/forwards", map[string]any{"Forwards": []store.NodeForward{}}, 200)
	fs, _ := r.st.NodeForwards(ctx, relays[0])
	if len(fs) != 1 || fs[0].LineID != l.ID {
		t.Fatal("old caller dropped managed rules")
	}
	fs[0].Target = "example.com:443"
	call(r.c, "PUT", "/api/admin/nodes/"+itoa(relays[0])+"/forwards", map[string]any{"Forwards": fs}, 409)
	call(r.c, "PUT", "/api/admin/nodes/"+itoa(relays[0])+"/forwards", map[string]any{"Forwards": []store.NodeForward{{Forward: spec.Forward{Tag: "collision", Port: 18443, Protocol: "tcp", Target: "example.com:443"}}}}, 400)
	stale := l
	l.Groups[0].Members[0].Port = 18444
	l = mustJSON[store.ForwardLine](t, call(r.c, "PUT", path, l, 200))
	call(r.c, "PUT", path, stale, 409)
	e, _ = func() (domain.Entry, error) { x, err := r.st.EntryByID(ctx, e.ID); return *x, err }()
	if e.DisplayPort != 18444 {
		t.Fatal("entry did not follow port edit", e)
	}
	call(r.c, "DELETE", path+"?version="+itoa(l.Version), nil, 409)
	call(r.c, "DELETE", "/api/admin/nodes/"+itoa(relays[0]), nil, 409)
	call(r.c, "DELETE", "/api/admin/inbounds/"+itoa(ib.ID), nil, 409)
	call(r.c, "POST", "/api/admin/nodes/"+itoa(relays[0])+"/removal", map[string]any{"mode": "standalone"}, 409)
	u, _ := admin.NewUser("operator@example.com", "password123", "operator")
	if err := r.st.CreateStaff(ctx, u); err != nil {
		t.Fatal(err)
	}
	op := &client{t: t, srv: r.srv}
	call(op, "POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, 200)
	call(op, "GET", "/api/admin/lines", nil, 403)
	call(op, "POST", "/api/admin/lines", input, 403)
	call(op, "PATCH", ep, map[string]any{"Name": "Operator rename", "Tags": []string{"tag"}}, 200)
	call(op, "PATCH", ep, map[string]any{"LineID": l.ID + 1}, 403)
	call(op, "PATCH", ep, map[string]any{"DisplayPort": 19444}, 403)
	call(op, "PATCH", ep, map[string]any{"ClientExtra": map[string]any{"server": "example.com"}}, 403)
	call(r.c, "DELETE", ep, nil, 200)
	call(r.c, "DELETE", path+"?version="+itoa(l.Version-1), nil, 409)
	call(r.c, "DELETE", path+"?version="+itoa(l.Version), nil, 200)
	st := mustJSON[agentproto.State](t, call(agents[0], "GET", "/api/agent/state", nil, 200))
	if len(st.Forwards) != 0 {
		t.Fatal("deleted line remained deployed", st.Forwards)
	}
}
