package http

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestWireGuardSetupAPI(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	base := "/api/admin/settings/nodes/" + itoa(r.nodeID)
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := r.c.do(method, base+path, body, nil)
		if code != want {
			t.Fatalf("%s: %d %s", path, code, b)
		}
		return b
	}
	report := func(rep *agentproto.WireGuardReport) {
		t.Helper()
		if err := r.st.SetWireGuardReport(ctx, r.nodeID, auth.SHA256Hex(r.agent.token), rep); err != nil {
			t.Fatal(err)
		}
	}
	report(&agentproto.WireGuardReport{Available: true})
	old := mustJSON[store.WireGuardReadiness](t, call("GET", "/wireguard-readiness", nil, 200))
	if old.Supported || !old.Ready {
		t.Fatal(old)
	}
	call("POST", "/wireguard-setup", map[string]string{"action": "install"}, 409)
	rep := &agentproto.WireGuardReport{Setup: &spec.WireGuardSetup{Installable: true, Packages: []string{"wireguard-tools"}, Checks: []spec.PrivateAccessCheck{{ID: "tool_wg", OK: false}}}}
	report(rep)
	state := mustJSON[store.WireGuardReadiness](t, call("GET", "/wireguard-readiness?transit=true", nil, 200))
	if state.Ready || !state.Supported || !state.Installable {
		t.Fatal(state)
	}
	call("POST", "/wireguard-setup", map[string]string{"action": "shell"}, 400)
	first := mustJSON[map[string]string](t, call("POST", "/wireguard-setup", map[string]string{"action": "install"}, 200))
	second := mustJSON[map[string]string](t, call("POST", "/wireguard-setup", map[string]string{"action": "install"}, 200))
	if first["id"] != second["id"] {
		t.Fatal("duplicate job")
	}
	job, err := r.st.NodeJob(ctx, r.nodeID, first["id"])
	if err != nil {
		t.Fatal(err)
	}
	var p spec.WireGuardSetupRequest
	if json.Unmarshal(job.Params, &p) != nil || p.Action != "install" || p.ExpiresAt <= time.Now().Unix() || p.ExpiresAt > time.Now().Unix()+spec.WireGuardSetupTTL {
		t.Fatal(job)
	}
	if err := r.st.CompleteNodeJob(ctx, r.nodeID, job.ID, json.RawMessage(`{"error_code":"install_failed"}`), ""); err != nil {
		t.Fatal(err)
	}
	state = mustJSON[store.WireGuardReadiness](t, call("GET", "/wireguard-readiness", nil, 200))
	if state.Job == nil || state.Job.DoneAt == nil || state.Ready {
		t.Fatal(state)
	}
	rep.Available = true
	rep.Setup.Installable = false
	rep.Setup.Packages = []string{}
	rep.Setup.Checks[0].OK = true
	report(rep)
	state = mustJSON[store.WireGuardReadiness](t, call("GET", "/wireguard-readiness", nil, 200))
	if !state.Ready || state.Installable {
		t.Fatal(state)
	}
	call("POST", "/wireguard-setup", map[string]string{"action": "install"}, 409)
	call("POST", "/wireguard-setup", map[string]string{"action": "check"}, 200)
	report(nil)
	call("POST", "/wireguard-setup", map[string]string{"action": "check"}, 409)
}

func TestWireGuardSetupPermissions(t *testing.T) {
	r := newRig(t)
	for _, role := range []string{"operator", "support"} {
		u, err := admin.NewUser(role+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.st.CreateStaff(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		if code, _, _ := c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil); code != 200 {
			t.Fatal(code)
		}
		base := "/api/admin/settings/nodes/" + itoa(r.nodeID)
		if code, _, _ := c.do("GET", base+"/wireguard-readiness", nil, nil); code != 403 {
			t.Fatal(role, code)
		}
		if code, _, _ := c.do("POST", base+"/wireguard-setup", map[string]string{"action": "install"}, nil); code != 403 {
			t.Fatal(role, code)
		}
	}
}
