package http

import (
	"context"
	"encoding/json"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
	"strings"
	"testing"
)

func TestHostManagementDeliveryPermissionsAndHistory(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/settings/nodes/" + itoa(r.nodeID) + "/host"
	call := func(c *client, method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: %d %s", method, path, code, b)
		}
		return b
	}
	inspect := spec.HostRequest{Action: "inspect"}
	call(r.c, "POST", path, inspect, 409)
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{Version: spec.HostManagementMinimumVersion}, 200)
	call(r.agent, "GET", path, nil, 401)
	for _, role := range []string{"operator", "support"} {
		u, e := admin.NewUser(role+"@example.com", "password123", role)
		if e != nil {
			t.Fatal(e)
		}
		if e = r.st.CreateStaff(context.Background(), u); e != nil {
			t.Fatal(e)
		}
		c := &client{t: t, srv: r.srv}
		call(c, "POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, 200)
		call(c, "GET", path, nil, 403)
		call(c, "POST", path, inspect, 403)
	}
	call(r.c, "POST", path, spec.HostRequest{Action: "shell"}, 400)
	id := mustJSON[map[string]string](t, call(r.c, "POST", path, inspect, 200))["id"]
	call(r.c, "POST", path, inspect, 409)
	state := mustJSON[agentproto.State](t, call(r.agent, "GET", "/api/agent/state", nil, 200))
	found := false
	for _, j := range state.Jobs {
		if j.ID == id && j.Kind == spec.HostManagementKind {
			found = true
			var p spec.HostJobParams
			if json.Unmarshal(j.Params, &p) != nil || p.ExpiresAt == 0 {
				t.Fatal("missing expiry")
			}
		}
	}
	if !found {
		t.Fatal("task not delivered")
	}
	raw := json.RawMessage(`{"outcome":"ok","state":{"checked_at":1,"environment":"restricted","revision":"test","items":[],"firewall":{"kind":"ufw","state":"synced","checked_at":1,"rules":[{"port":443,"protocol":"tcp","sources":["inbound:test"],"state":"external"}]}}}`)
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{Version: spec.HostManagementMinimumVersion, Jobs: []agentproto.JobResult{{ID: id, Kind: spec.HostManagementKind, Result: raw}}}, 200)
	data := mustJSON[struct {
		State             *spec.HostSnapshot
		History           []store.NodeJob
		Supported, Online bool
	}](t, call(r.c, "GET", path, nil, 200))
	if data.State == nil || len(data.History) != 1 || data.History[0].DoneAt == nil || !data.Supported || !data.Online {
		t.Fatal(data)
	}
	if data.State.Firewall == nil || len(data.State.Firewall.Rules) != 1 || data.State.Firewall.Rules[0].Sources[0] != "inbound:test" {
		t.Fatal("firewall details lost in host snapshot")
	}
	// Generic cleanup must not erase host observations or operation history.
	if e := r.st.CreateNodeJob(context.Background(), "other", r.nodeID, "reality_scan", nil); e != nil {
		t.Fatal(e)
	}
	req := spec.HostRequest{Action: "apply", Section: "admin", Revision: strings.Repeat("a", 64), Values: map[string]string{"user": "operator", "public_key": "example-public-key"}}
	call(r.c, "POST", path, req, 200)
	body := call(r.c, "GET", path, nil, 200)
	if strings.Contains(string(body), "example-public-key") {
		t.Fatal("key leaked in history")
	}
}
