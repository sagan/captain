package http

import (
	"context"
	"encoding/json"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
	"testing"
)

func TestReturnRouteAPIAndDelivery(t *testing.T) {
	r := newRig(t)
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	call := func(c *client, method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s %d: %s", method, path, code, b)
		}
		return b
	}
	req := map[string]any{"kind": spec.NetworkDiagnosticKind, "params": spec.DiagnosticRequest{Type: "return_route", Route: &spec.ReturnRouteRequest{City: "beijing", Family: "ipv4"}}}
	settings := "/api/admin/settings/nodes/" + itoa(r.nodeID) + "/return-routes"
	p := store.RouteSchedule{Enabled: true, IntervalHours: 6, City: "beijing", Family: "ipv4"}
	call(r.c, "POST", base+"/jobs", req, 409)
	call(r.c, "PUT", settings, p, 409)
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{Version: spec.ReturnRouteMinimumVersion}, 200)
	call(r.c, "PUT", settings, p, 200)
	call(r.agent, "GET", base+"/return-routes", nil, 401)
	call(r.agent, "PUT", settings, p, 401)
	b := call(r.c, "POST", base+"/jobs", req, 200)
	id := mustJSON[map[string]string](t, b)["id"]
	call(r.c, "POST", base+"/jobs", req, 409)
	state := mustJSON[agentproto.State](t, call(r.agent, "GET", "/api/agent/state", nil, 200))
	found := false
	for _, j := range state.Jobs {
		if j.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("job not delivered")
	}
	data := mustJSON[struct {
		Schedule store.RouteSchedule
		History  []store.NodeJob
	}](t, call(r.c, "GET", base+"/return-routes", nil, 200))
	if !data.Schedule.Enabled || len(data.History) != 1 {
		t.Fatal(data)
	}
	for _, role := range []string{"operator", "support"} {
		u, err := admin.NewUser(role+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.st.CreateStaff(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		call(c, "POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, 200)
		call(c, "GET", base+"/return-routes", nil, 403)
		call(c, "PUT", settings, p, 403)
		call(c, "POST", base+"/jobs", req, 403)
	}
	result := json.RawMessage(`{"type":"return_route","outcome":"incomplete","route":{"city":"beijing","family":"ipv4","paths":[]}}`)
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{Version: spec.ReturnRouteMinimumVersion, Jobs: []agentproto.JobResult{{ID: id, Kind: spec.NetworkDiagnosticKind, Result: result}}}, 200)
	done := mustJSON[struct{ History []store.NodeJob }](t, call(r.c, "GET", base+"/return-routes", nil, 200))
	if len(done.History) != 1 || done.History[0].DoneAt == nil || string(done.History[0].Result) != string(result) {
		t.Fatal("result not persisted", done)
	}
	p.IntervalHours = 1
	call(r.c, "PUT", settings, p, 400)
}

func TestReturnRouteMonitorNodeDelivery(t *testing.T) {
	r := newRig(t)
	call := func(c *client, method, path string, body any) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != 200 {
			t.Fatalf("%s %s %d: %s", method, path, code, b)
		}
		return b
	}
	node := mustJSON[struct {
		ID       int64
		PairCode string `json:"pair_code"`
	}](t, call(r.c, "POST", "/api/admin/nodes", map[string]any{"Name": "route-monitor", "Mode": "monitor"}))
	c := &client{t: t, srv: r.srv}
	pair := mustJSON[agentproto.PairResponse](t, call(c, "POST", "/api/agent/pair", agentproto.PairRequest{Code: node.PairCode, NodeModes: true, Version: spec.ReturnRouteMinimumVersion}))
	c.token = pair.Token
	call(c, "POST", "/api/agent/report", agentproto.Report{Version: spec.ReturnRouteMinimumVersion, NodeMode: spec.NodeMonitor, NodeModeRevision: 1})
	job := mustJSON[map[string]string](t, call(r.c, "POST", "/api/admin/nodes/"+itoa(node.ID)+"/jobs", map[string]any{"kind": spec.NetworkDiagnosticKind, "params": spec.DiagnosticRequest{Type: "return_route", Route: &spec.ReturnRouteRequest{City: "shanghai", Family: "ipv6"}}}))
	st := mustJSON[agentproto.State](t, call(c, "GET", "/api/agent/state", nil))
	if st.Node.Mode != spec.NodeMonitor || len(st.Users) != 0 || spec.ValidateMonitorState(&st.Node, st.Users, st.Forwards) != nil {
		t.Fatal("monitor guard bypass")
	}
	if len(st.Jobs) != 1 || st.Jobs[0].ID != job["id"] {
		t.Fatal("monitor diagnostic not delivered")
	}
}
