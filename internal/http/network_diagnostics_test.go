package http

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNetworkDiagnosticsEndToEndAndPermissions(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	req := map[string]any{"kind": spec.NetworkDiagnosticKind, "params": spec.DiagnosticRequest{Type: "tcp", Target: "example.com:443"}}
	r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: "v0.55.0"}, nil)
	if code, _, _ := r.c.do("POST", base+"/jobs", req, nil); code != 409 {
		t.Fatal("old node accepted", code)
	}
	r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: "v0.56.0"}, nil)
	if code, _, _ := r.c.do("POST", base+"/jobs", map[string]any{"kind": spec.NetworkDiagnosticKind, "params": spec.DiagnosticRequest{Type: "mtr", Target: "--help"}}, nil); code != 400 {
		t.Fatal("invalid target", code)
	}
	_, _, hdr := r.agent.do("GET", "/api/agent/state", nil, nil)
	code, b, _ := r.c.do("POST", base+"/jobs", req, nil)
	if code != 200 {
		t.Fatalf("create %d %s", code, b)
	}
	id := mustJSON[map[string]string](t, b)["id"]
	if code, _, _ := r.c.do("POST", base+"/jobs", req, nil); code != 409 {
		t.Fatal("duplicate accepted", code)
	}
	code, b, _ = r.agent.do("GET", "/api/agent/state", nil, map[string]string{"If-None-Match": hdr.Get("ETag")})
	state := mustJSON[agentproto.State](t, b)
	if code != 200 || len(state.Jobs) != 1 || state.Jobs[0].ID != id {
		t.Fatal("not delivered", code, string(b))
	}
	var params spec.DiagnosticParams
	if err := json.Unmarshal(state.Jobs[0].Params, &params); err != nil || params.ExpiresAt <= time.Now().Unix() || params.ExpiresAt > time.Now().Unix()+120 {
		t.Fatal(params, err)
	}
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err := r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		if code, _, _ := c.do("POST", base+"/jobs", req, nil); code != 403 {
			t.Fatal(role, "could run", code)
		}
		for _, path := range []string{base + "/network-diagnostics", base + "/jobs/" + id} {
			if code, _, _ := c.do("GET", path, nil, nil); code != 403 {
				t.Fatal(role, "could read", code)
			}
		}
	}
	anon := &client{t: t, srv: r.srv}
	if code, _, _ := anon.do("GET", base+"/network-diagnostics", nil, nil); code != 401 {
		t.Fatal("anonymous read", code)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/nodes/999999/jobs/"+id, nil, nil); code != 404 {
		t.Fatal("cross-node read", code)
	}
	// A failed database write must make the node retry its result.
	if _, err := r.st.DB().Exec(`CREATE TRIGGER reject_diag BEFORE UPDATE ON node_jobs BEGIN SELECT RAISE(FAIL,'test write failure'); END`); err != nil {
		t.Fatal(err)
	}
	report := agentproto.Report{Version: "v0.56.0", Jobs: []agentproto.JobResult{{ID: id, Kind: spec.NetworkDiagnosticKind, Result: json.RawMessage(`{"type":"tcp","outcome":"ok"}`)}}}
	if code, _, _ := r.agent.do("POST", "/api/agent/report", report, nil); code != 500 {
		t.Fatal("lost result acknowledged", code)
	}
	if _, err := r.st.DB().Exec(`DROP TRIGGER reject_diag`); err != nil {
		t.Fatal(err)
	}
	if code, b, _ := r.agent.do("POST", "/api/agent/report", report, nil); code != 200 && code != 204 {
		t.Fatalf("report %d %s", code, b)
	}
	_, b, _ = r.c.do("GET", base+"/network-diagnostics", nil, nil)
	jobs := mustJSON[[]store.NodeJob](t, b)
	if len(jobs) != 1 || jobs[0].DoneAt == nil || string(jobs[0].Result) != string(report.Jobs[0].Result) {
		t.Fatal(jobs)
	}
	// Replayed reports cannot replace the first completed result.
	report.Jobs[0].Result = json.RawMessage(`{"outcome":"failed"}`)
	r.agent.do("POST", "/api/agent/report", report, nil)
	j, _ := r.st.NodeJob(ctx, r.nodeID, id)
	if string(j.Result) != string(jobs[0].Result) {
		t.Fatal("result overwritten", j)
	}
}
