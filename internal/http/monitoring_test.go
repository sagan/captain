package http

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestMonitoringPermissionsHistoryAndGroups(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/nodes/" + itoa(r.nodeID)
	anon := &client{t: t, srv: r.srv}
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err := r.st.CreateStaff(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		for _, p := range []string{"/api/admin/monitoring", path + "/resource-history?range=1h", path + "/network-quality?range=1h"} {
			want := 200
			if role == "support" {
				want = 403
			}
			if code, b, _ := c.do("GET", p, nil, nil); code != want {
				t.Fatalf("%s %d %s", role, code, b)
			}
			if code, _, _ := anon.do("GET", p, nil, nil); code != 401 {
				t.Fatal("anonymous monitoring", code)
			}
		}
		want := 200
		if role == "support" {
			want = 403
		}
		if code, _, _ := c.do("PUT", path+"/monitor-group", map[string]string{"group": "Edge"}, nil); code != want {
			t.Fatal("group role", code)
		}
	}
	if code, _, _ := r.c.do("PUT", path+"/monitor-group", map[string]string{"group": "bad\nname"}, nil); code != 400 {
		t.Fatal("invalid group", code)
	}
	if code, _, _ := r.c.do("GET", path+"/resource-history?range=100y", nil, nil); code != 400 {
		t.Fatal("unbounded history", code)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/nodes/99999/resource-history", nil, nil); code != 404 {
		t.Fatal("missing node", code)
	}
	// Off collection still permits the fleet workspace and minute report snapshot.
	h := spec.SystemStatus{CPUPercent: 33, Resources: &spec.Resources{Epoch: "one", Sequence: 1, Processes: []spec.ProcessResource{{Name: "private-process"}}}}
	if err := r.st.TouchNode(context.Background(), r.nodeID, "", "", h, nil, nil); err != nil {
		t.Fatal(err)
	}
	code, b, _ := r.c.do("GET", "/api/admin/monitoring", nil, nil)
	nodes := mustJSON[[]store.MonitorNode](t, b)
	if code != 200 || len(nodes) != 1 || nodes[0].Group != "Edge" || nodes[0].Host.CPUPercent != 33 || nodes[0].SampledAt == 0 || strings.Contains(string(b), "private-process") {
		t.Fatalf("workspace %d %s", code, b)
	}
	if err := r.st.RecordBeat(context.Background(), r.nodeID, h, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, b, _ = r.c.do("GET", path+"/resource-history?series="+url.QueryEscape("host::cpu"), nil, nil)
	hist := mustJSON[store.ResourceHistory](t, b)
	if len(hist.Points) != 60 || *hist.Points[59].Peak != 33 {
		t.Fatalf("history %s", b)
	}
}

func TestPublicSectionsMaskSnapshotAndHistory(t *testing.T) {
	r := newRig(t)
	anon := &client{t: t, srv: r.srv}
	settings := map[string]any{"enabled": true, "path": "/status", "public_sections": []string{"history"}, "layout": "compact"}
	if code, _, _ := r.c.do("PUT", "/api/admin/settings/probe", settings, nil); code != 200 {
		t.Fatal(code)
	}
	host := spec.SystemStatus{CPUPercent: 73, MemTotal: 100, MemUsed: 41, DiskTotal: 100, DiskUsed: 52, NetUp: 123, NetDown: 456, Load1: 3, TCP: 88, Info: &spec.HostInfo{OS: "private-os"}, Pings: []spec.PingResult{{Name: "private-target", LatencyMs: 1}}, Resources: &spec.Resources{Epoch: "r", Sequence: 1, Processes: []spec.ProcessResource{{Name: "private-process"}}}}
	if code, b, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: host}, nil); code != 204 {
		t.Fatalf("beat %d %s", code, b)
	}
	for _, p := range []string{"/api/probe", "/api/probe/nodes/" + itoa(r.nodeID) + "/history?range=1h", "/api/probe/nodes/" + itoa(r.nodeID) + "/history?range=24h"} {
		code, b, _ := anon.do("GET", p, nil, nil)
		if code != 200 {
			t.Fatalf("%s %d %s", p, code, b)
		}
		for _, secret := range []string{"private-os", "private-target", "private-process"} {
			if strings.Contains(string(b), secret) {
				t.Fatalf("hidden data %s: %s", p, b)
			}
		}
		// Check decoded numbers: searching for ":41" also matches a perfectly
		// public last_seen timestamp whose seconds/minutes happen to be 41.
		var decoded any
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		var check func(any)
		check = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for _, v := range x {
					check(v)
				}
			case []any:
				for _, v := range x {
					check(v)
				}
			case float64:
				switch x {
				case 73, 41, 52, 123, 456, 88:
					t.Fatalf("hidden numeric data %s: %s", p, b)
				}
			}
		}
		check(decoded)
	}
	// Masking must not mutate private live values.
	_, b, _ := r.c.do("GET", "/api/admin/nodes/"+itoa(r.nodeID)+"/probe", nil, nil)
	if !strings.Contains(string(b), "private-process") || !strings.Contains(string(b), `"cpu_percent":73`) {
		t.Fatalf("private values mutated %s", b)
	}
	// Old clients cannot accidentally re-enable hidden sections or reset layout.
	r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status"}, nil)
	_, b, _ = r.c.do("GET", "/api/admin/settings/probe", nil, nil)
	if !strings.Contains(string(b), `"public_sections":["history"]`) || !strings.Contains(string(b), `"layout":"compact"`) {
		t.Fatalf("compatibility %s", b)
	}
	if code, _, _ := anon.do("GET", "/api/probe/nodes/"+itoa(r.nodeID)+"/pings", nil, nil); code != 404 {
		t.Fatal("hidden latency", code)
	}
	settings["public_sections"] = []string{}
	r.c.do("PUT", "/api/admin/settings/probe", settings, nil)
	if code, _, _ := anon.do("GET", "/api/probe/nodes/"+itoa(r.nodeID)+"/history", nil, nil); code != 404 {
		t.Fatal("hidden history", code)
	}
}
