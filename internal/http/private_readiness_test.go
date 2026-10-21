package http

import (
	"context"
	"github.com/zeptop-dev/captain/internal/store"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestPrivateReadinessReportAndJobGate(t *testing.T) {
	r := newRig(t)
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	job := map[string]any{"kind": spec.PrivateAccessCheckKind, "params": map[string]any{"ignored": "not forwarded"}}
	if code, _, _ := r.c.do("POST", base+"/jobs", job, nil); code != 409 {
		t.Fatalf("old agent accepted job: %d", code)
	}
	caps := spec.CapabilitiesForCore("singbox")
	caps.PrivateAccess = false
	caps.PrivateAccessReadiness = &spec.PrivateAccessReadiness{CheckedAt: time.Now().Unix(), Checks: []spec.PrivateAccessCheck{{ID: "user", OK: false}}}
	report := func() {
		t.Helper()
		if code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"singbox": {Capabilities: &caps}}}, nil); code != 200 {
			t.Fatalf("report %d %s", code, b)
		}
	}
	report()
	_, b, _ := r.c.do("GET", base+"/core-options?protocol=vless&private_access=true", nil, nil)
	p := mustJSON[spec.CoreOptions](t, b)
	if p.PrivateAccessReadiness == nil || p.PrivateAccessReadiness.Checks[0].ID != "user" || p.AutoCore != "" {
		t.Fatalf("lost readiness: %s", b)
	}
	if code, b, _ := r.c.do("POST", base+"/jobs", job, nil); code != 200 {
		t.Fatalf("supported job rejected %d %s", code, b)
	}
	_, first, _ := r.c.do("POST", base+"/jobs", job, nil)
	_, second, _ := r.c.do("POST", base+"/jobs", job, nil)
	if mustJSON[map[string]string](t, first)["id"] != mustJSON[map[string]string](t, second)["id"] {
		t.Fatal("duplicate check was queued")
	}
	caps.PrivateAccessReadiness.CheckedAt = time.Now().Add(-5 * time.Minute).Unix()
	caps.PrivateAccess = true
	report()
	_, b, _ = r.c.do("GET", base+"/core-options?protocol=vless&private_access=true", nil, nil)
	p = mustJSON[spec.CoreOptions](t, b)
	if p.PrivateAccessReadiness != nil || p.AutoCore != "" {
		t.Fatal("heartbeat refreshed stale readiness")
	}
	if code, _, _ := r.c.do("POST", base+"/jobs", job, nil); code != 409 {
		t.Fatalf("stale agent accepted job %d", code)
	}
}

func TestCoreOnlyPrivateAccessAPI(t *testing.T) {
	r := newRig(t)
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	caps := spec.CapabilitiesForCore("singbox")
	caps.PrivateAccess = false
	caps.PrivateAccessReadiness = &spec.PrivateAccessReadiness{CheckedAt: time.Now().Unix(), Checks: []spec.PrivateAccessCheck{{ID: "guard", OK: false}}}
	report := func() {
		t.Helper()
		code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"singbox": {Capabilities: &caps}}}, nil)
		if code != 200 {
			t.Fatalf("report %d %s", code, b)
		}
	}
	report()
	body := map[string]any{"Tag": "core-only", "Protocol": "vless", "Core": "singbox", "Port": 18445, "Settings": map[string]any{"private_access": map[string]any{"mode": "internal"}}}
	if code, _, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 400 {
		t.Fatal("dual bypassed disabled guard", code)
	}
	body["Settings"] = map[string]any{"private_access": map[string]any{"mode": "internal", "enforcement": "core_only"}}
	if code, b, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 200 {
		t.Fatalf("explicit core-only rejected: %d %s", code, b)
	}
	_, b, _ := r.c.do("GET", base+"/core-options?protocol=vless&private_access=true&private_enforcement=core_only", nil, nil)
	if p := mustJSON[spec.CoreOptions](t, b); p.AutoCore != "singbox" {
		t.Fatal("probe lost enforcement")
	}
	caps.CoreOnlyPrivateAccess = false
	report()
	body["Tag"] = "legacy-refused"
	body["Port"] = 18446
	if code, _, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 400 {
		t.Fatal("legacy capability accepted core-only", code)
	}
}

func TestAutomaticPrivateAccessAPIRequiresConsent(t *testing.T) {
	r := newRig(t)
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	c := spec.CapabilitiesForCore("singbox")
	c.PrivateAccess = false
	c.CoreOnlyPrivateAccess = true
	c.PrivateAccessPolicy = &spec.PrivateAccessPolicyCapabilities{Scoped: true, IndependentDNS: true}
	c.PrivateAccessReadiness = &spec.PrivateAccessReadiness{CheckedAt: time.Now().Unix(), EffectiveMode: "core_only", Checks: []spec.PrivateAccessCheck{{ID: "guard", OK: false}, {ID: "user", OK: true}}}
	report := func() {
		t.Helper()
		code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"singbox": {Capabilities: &c}}}, nil)
		if code != 200 {
			t.Fatalf("%d %s", code, b)
		}
	}
	report()
	policy := map[string]any{"mode": "internal", "enforcement": "auto"}
	body := map[string]any{"Tag": "automatic", "Protocol": "vless", "Core": "singbox", "Port": 18448, "Settings": map[string]any{"private_access": policy}}
	if code, _, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 400 {
		t.Fatal("missing consent accepted", code)
	}
	policy["allow_core_only"] = true
	if code, b, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 200 {
		t.Fatalf("consent rejected: %d %s", code, b)
	}
	_, stateJSON, _ := r.agent.do("GET", "/api/agent/state", nil, nil)
	state := mustJSON[agentproto.State](t, stateJSON)
	found := false
	for _, ib := range state.Node.Inbounds {
		if ib.Tag == "automatic" {
			found = true
			if ib.PrivateAccess.Enforcement != "auto" || !ib.PrivateAccess.AllowCoreOnly {
				t.Fatal("delivery replaced stored automatic intent", ib.PrivateAccess)
			}
		}
	}
	if !found {
		t.Fatal("supported automatic inbound was not delivered")
	}
	c.PrivateAccess = true
	c.CoreOnlyPrivateAccess = false
	c.PrivateAccessReadiness.EffectiveMode = "dual"
	c.PrivateAccessReadiness.Checks[0].OK = true
	report()
	policy["allow_core_only"] = false
	body["Tag"] = "automatic-dual"
	body["Port"] = 18449
	if code, b, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 200 {
		t.Fatalf("dual unnecessarily requires consent: %d %s", code, b)
	}
	c.PrivateAccessPolicy = nil
	report()
	_, stateJSON, _ = r.agent.do("GET", "/api/agent/state", nil, nil)
	state = mustJSON[agentproto.State](t, stateJSON)
	for _, ib := range state.Node.Inbounds {
		if ib.Tag == "automatic" || ib.Tag == "automatic-dual" {
			t.Fatal("automatic policy delivered after capability downgrade")
		}
	}
	body["Tag"] = "old-refused"
	body["Port"] = 18450
	if code, _, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 400 {
		t.Fatal("old node accepted auto", code)
	}
}

func TestCoreOnlyPrivateAccessSplitRoutingAPI(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	base := "/api/admin/nodes/" + itoa(r.nodeID)
	routing := &store.NodeRouting{Outbounds: []spec.Outbound{{Tag: "warp-one", WARP: &spec.WARP{FromNode: true}}, {Tag: "warp-two", WARP: &spec.WARP{FromNode: true}}}, Routes: []spec.RouteRule{{Match: []string{"domain:example.com"}, Action: "outbound", Value: "warp-two"}}, DefaultOutbound: "warp-one"}
	if err := r.st.SetNodeRouting(ctx, r.nodeID, routing); err != nil {
		t.Fatal(err)
	}
	c := spec.CapabilitiesForCore("singbox")
	c.PrivateAccess = false
	c.PrivateAccessPolicy = &spec.PrivateAccessPolicyCapabilities{Scoped: true, IndependentDNS: true}
	c.PrivateAccessReadiness = &spec.PrivateAccessReadiness{CheckedAt: time.Now().Unix(), EffectiveMode: "core_only"}
	report := func() {
		t.Helper()
		if code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{Cores: map[string]agentproto.CoreStatus{"singbox": {Capabilities: &c}}}, nil); code != 200 {
			t.Fatalf("%d %s", code, b)
		}
	}
	report()
	body := map[string]any{"Tag": "split", "Protocol": "vless", "Core": "singbox", "Port": 18460, "Settings": map[string]any{"private_access": map[string]any{"mode": "internal", "enforcement": "auto", "allow_core_only": true}}}
	if code, _, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 400 {
		t.Fatal("old node accepted expanded routing", code)
	}
	c.PrivateAccessPolicy.SplitRouting = true
	report()
	if code, b, _ := r.c.do("POST", base+"/inbounds", body, nil); code != 200 {
		t.Fatalf("new capability rejected: %d %s", code, b)
	}
	delivered := func() bool {
		t.Helper()
		code, b, _ := r.agent.do("GET", "/api/agent/state", nil, nil)
		if code != 200 {
			t.Fatalf("%d %s", code, b)
		}
		st := mustJSON[agentproto.State](t, b)
		for _, ib := range st.Node.Inbounds {
			if ib.Tag == "split" {
				return true
			}
		}
		return false
	}
	if !delivered() {
		t.Fatal("new node not delivered")
	}
	if code, b, _ := r.c.do("PUT", base+"/routing", routing, nil); code != 200 {
		t.Fatalf("routing editor rejected supported combination: %d %s", code, b)
	}
	c.PrivateAccessPolicy.SplitRouting = false
	report()
	if code, _, _ := r.c.do("PUT", base+"/routing", routing, nil); code != 400 {
		t.Fatal("routing editor bypassed old-node gate", code)
	}
	if delivered() {
		t.Fatal("downgrade received unsupported topology")
	}
	c.PrivateAccessPolicy.SplitRouting = true
	c.CoreOnlyPrivateAccess = false
	c.PrivateAccess = true
	c.PrivateAccessReadiness.EffectiveMode = "dual"
	report()
	if delivered() {
		t.Fatal("guard re-enable silently weakened isolation")
	}
}
