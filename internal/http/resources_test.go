package http

import (
	"strings"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestResourcesPrivateAndPageIndependent(t *testing.T) {
	r := newRig(t)
	nodePath := "/api/admin/nodes/" + itoa(r.nodeID) + "/probe"
	put := func(path string, v any) {
		t.Helper()
		if code, b, _ := r.c.do("PUT", path, v, nil); code != 200 {
			t.Fatalf("PUT %s: %d %s", path, code, b)
		}
	}
	put("/api/admin/settings/probe", map[string]any{"enabled": true, "page_enabled": false, "path": "/status", "visibility": "public"})
	put(nodePath, map[string]any{"Resources": spec.ResourceOptions{IncludeInterfaces: []string{"eth0"}, ExcludeInterfaces: []string{"docker0"}}})
	_, b, _ := r.agent.do("GET", "/api/agent/state", nil, nil)
	state := mustJSON[agentproto.State](t, b)
	if state.Probe == nil || !state.Probe.Enabled || state.Probe.Resources == nil || len(state.Probe.Resources.IncludeInterfaces) != 1 {
		t.Fatalf("collection was disabled: %s", b)
	}
	h := spec.SystemStatus{Valid: &spec.MetricValidity{Memory: true}, MemTotal: 100, MemUsed: 10, Resources: &spec.Resources{Epoch: "test", Sequence: 1, At: 1, Filesystems: []spec.FilesystemResource{{Mount: "/private-volume"}}, Processes: []spec.ProcessResource{{Name: "private-process", PID: 42}}}}
	if code, b, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: h}, nil); code != 204 {
		t.Fatalf("beat: %d %s", code, b)
	}
	code, b, _ := r.c.do("GET", nodePath, nil, nil)
	if code != 200 || !strings.Contains(string(b), "/private-volume") {
		t.Fatalf("admin details missing: %d %s", code, b)
	}
	anon := &client{t: t, srv: r.srv}
	if code, _, _ := anon.do("GET", "/api/probe", nil, nil); code != 404 {
		t.Fatalf("disabled page API: %d", code)
	}
	// Legacy clients updating settings must preserve the separate page switch.
	put("/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status"})
	if code, _, _ := anon.do("GET", "/api/probe", nil, nil); code != 404 {
		t.Fatal("legacy settings re-enabled the page")
	}
	// Same applies to per-node updates which omit resource selection.
	put(nodePath, map[string]any{"Hidden": false})
	_, b, _ = r.c.do("GET", nodePath, nil, nil)
	if !strings.Contains(string(b), `"include_interfaces":["eth0"]`) {
		t.Fatal("selection lost in legacy update")
	}
	put("/api/admin/settings/probe", map[string]any{"enabled": true, "page_enabled": true, "path": "/status", "visibility": "public"})
	code, b, _ = anon.do("GET", "/api/probe", nil, nil)
	if code != 200 || strings.Contains(string(b), "private-volume") || strings.Contains(string(b), "private-process") || strings.Contains(string(b), `"resources"`) {
		t.Fatalf("public detail leak: %d %s", code, b)
	}
	if !strings.Contains(string(b), `"memory":true`) {
		t.Fatal("public summary lost validity flags")
	}
	if code, _, _ := anon.do("GET", nodePath, nil, nil); code != 401 {
		t.Fatalf("anonymous resource access: %d", code)
	}
	if code, _, _ := r.c.do("PUT", nodePath, map[string]any{"Resources": spec.ResourceOptions{IncludeInterfaces: []string{"eth0;\nsecret"}}}, nil); code != 400 {
		t.Fatalf("bad NIC accepted: %d", code)
	}
}
