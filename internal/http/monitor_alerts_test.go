package http

import (
	"context"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
	"strings"
	"testing"
	"time"
)

func TestMonitorLifecyclePermissionsAndPublicPrivacy(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	at := time.Now()
	notice, err := r.st.CheckIncident(ctx, r.nodeID, store.IncidentCheck{Kind: "cpu", Active: true, Value: 95, Threshold: 80}, at)
	if err != nil || notice == nil {
		t.Fatal(err)
	}
	payload := map[string]any{"node_id": r.nodeID, "kind": "maintenance", "starts_at": 0, "ends_at": at.Add(time.Hour).Unix(), "note": "private operations note"}
	code, body, _ := r.c.do("POST", "/api/admin/settings/monitor-windows", payload, nil)
	if code != 200 {
		t.Fatalf("create %d %s", code, body)
	}
	window := mustJSON[store.MonitorWindow](t, body)
	incidentPath := "/api/admin/monitoring/incidents/" + itoa(notice.ID) + "/ack"
	availabilityPath := "/api/admin/nodes/" + itoa(r.nodeID) + "/availability?range=24h"
	for _, role := range []string{"support", "operator"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err = r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		want := 200
		if role == "support" {
			want = 403
		}
		for _, path := range []string{"/api/admin/monitoring/incidents", "/api/admin/monitoring/windows", availabilityPath} {
			if code, b, _ := c.do("GET", path, nil, nil); code != want {
				t.Fatalf("%s %s: %d %s", role, path, code, b)
			}
		}
		if code, b, _ := c.do("POST", incidentPath, nil, nil); code != want {
			t.Fatalf("ack role %s %d %s", role, code, b)
		}
		if code, _, _ := c.do("POST", "/api/admin/settings/monitor-windows", payload, nil); code != 403 {
			t.Fatal("operator/support could mute fleet", code)
		}
		if code, _, _ := c.do("DELETE", "/api/admin/settings/monitor-windows/"+itoa(window.ID), nil, nil); code != 403 {
			t.Fatal("operator/support canceled window", code)
		}
	}
	anon := &client{t: t, srv: r.srv}
	for _, path := range []string{"/api/admin/monitoring/incidents", "/api/admin/monitoring/windows", availabilityPath} {
		if code, _, _ := anon.do("GET", path, nil, nil); code != 401 {
			t.Fatal("anonymous management", code)
		}
	}
	if code, _, _ := r.c.do("GET", "/api/admin/monitoring/incidents?node_id=oops", nil, nil); code != 400 {
		t.Fatal("bad filter", code)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/nodes/999999/availability", nil, nil); code != 404 {
		t.Fatal("missing node", code)
	}
	if code, _, _ := r.c.do("GET", availabilityPath+"0", nil, nil); code != 400 {
		t.Fatal("unbounded range", code)
	}
	payload["starts_at"] = at.Add(-time.Hour).Unix()
	if code, _, _ := r.c.do("POST", "/api/admin/settings/monitor-windows", payload, nil); code != 400 {
		t.Fatal("backdated maintenance", code)
	}
	settings := map[string]any{"enabled": true, "page_enabled": true, "public_sections": []string{"history", "availability"}}
	r.c.do("PUT", "/api/admin/settings/probe", settings, nil)
	publicPath := "/api/probe/nodes/" + itoa(r.nodeID) + "/availability?range=24h"
	code, body, _ = anon.do("GET", publicPath, nil, nil)
	if code != 200 || strings.Contains(string(body), "private operations") || strings.Contains(string(body), "acknowledged_by") {
		t.Fatalf("public availability %d %s", code, body)
	}
	for _, sections := range [][]string{{"history"}, {"availability"}, {}} {
		settings["public_sections"] = sections
		r.c.do("PUT", "/api/admin/settings/probe", settings, nil)
		if code, _, _ := anon.do("GET", publicPath, nil, nil); code != 404 {
			t.Fatal("hidden public availability", code)
		}
	}
	settings["public_sections"] = []string{"history", "availability"}
	r.c.do("PUT", "/api/admin/settings/probe", settings, nil)
	if err = r.st.UpdateNodeProbe(ctx, r.nodeID, true, store.NodeProbeInfo{}, 0, 1, "sum"); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := anon.do("GET", publicPath, nil, nil); code != 404 {
		t.Fatal("hidden node availability", code)
	}
	if code, _, _ := r.c.do("DELETE", "/api/admin/settings/monitor-windows/"+itoa(window.ID), nil, nil); code != 200 {
		t.Fatal("cancel", code)
	}
	rows, _ := r.st.MonitorWindows(ctx, at.Add(-time.Minute), at.Add(2*time.Hour))
	if len(rows) != 1 || rows[0].CanceledAt == nil {
		t.Fatal("cancel deleted history", rows)
	}
	// Disabling collection is not a recovery and immediately closes open events.
	r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": false}, nil)
	incidents, _ := r.st.MonitorIncidents(ctx, r.nodeID, 0, "resolved")
	if len(incidents) != 1 || incidents[0].Resolution != "disabled" {
		t.Fatal("disabled incidents", incidents)
	}
}
