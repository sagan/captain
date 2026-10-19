package http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeRegionAPIChoicesAndPublicPrivacy(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/probe"
	call := func(c *client, method, path string, in any, want int) []byte {
		t.Helper()
		status, b, _ := c.do(method, path, in, nil)
		if status != want {
			t.Fatalf("%s %s %d %s", method, path, status, b)
		}
		return b
	}
	read := func() *store.NodeProbe {
		return mustJSON[struct {
			Probe *store.NodeProbe `json:"probe"`
		}](t, call(r.c, "GET", path, nil, 200)).Probe
	}
	out := mustJSON[struct {
		Countries []string `json:"region_countries"`
	}](t, call(r.c, "GET", path, nil, 200))
	if len(out.Countries) < 240 || !store.ValidRegion("JP") || store.ValidRegion("ZZ") {
		t.Fatal(len(out.Countries))
	}
	call(r.agent, "POST", "/api/agent/report", agentproto.Report{PublicAddresses: &agentproto.PublicAddresses{SampledAt: time.Now().Unix(), IPv4: "198.51.100.20"}}, 200)
	state, err := r.st.NodeRegionState(ctx, r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	err = r.st.SaveRegionDetection(ctx, state, store.RegionDetection{IPv4: store.RegionResult{IP: "198.51.100.20", Country: "JP", Status: "ok", CheckedAt: time.Now().Unix()}})
	if err != nil {
		t.Fatal(err)
	}
	if p := read(); p.Info.Region != "JP" || p.RegionMode != "auto" {
		t.Fatal(p)
	}
	// Successful cached detection is shared by authorized staff; never by public or nodes.
	detect := "/api/admin/nodes/" + itoa(r.nodeID) + "/region-detect"
	call(r.c, "POST", detect, nil, 200)
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err = r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		call(c, "POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, 200)
		want := 200
		if role == "support" {
			want = 403
		}
		call(c, "POST", detect, nil, want)
	}
	anon := &client{t: t, srv: r.srv}
	call(anon, "POST", detect, nil, 401)
	call(r.agent, "POST", detect, nil, 401)
	for _, bad := range []map[string]any{{"RegionMode": "bad"}, {"RegionMode": "manual", "Info": map[string]string{"region": "XX"}}, {"RegionMode": "manual", "Info": map[string]string{"region": "<script>"}}} {
		call(r.c, "PUT", path, bad, 400)
	}
	call(r.c, "PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status", "show_ip": false}, 200)
	publicRegion := func(want string) {
		t.Helper()
		raw := call(anon, "GET", "/api/probe", nil, 200)
		data := mustJSON[struct {
			Nodes []struct {
				Info store.NodeProbeInfo `json:"info"`
			} `json:"nodes"`
		}](t, raw)
		if len(data.Nodes) != 1 || data.Nodes[0].Info.Region != want {
			t.Fatalf("public %s", raw)
		}
		for _, secret := range []string{"198.51.100.20", "country.is", "region_detection", "region_mode"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("private region metadata leaked: %s", secret)
			}
		}
	}
	publicRegion("JP")
	call(r.c, "PUT", path, map[string]any{"RegionMode": "off"}, 200)
	publicRegion("")
	call(r.c, "PUT", path, map[string]any{"RegionMode": "manual", "Info": map[string]string{"region": "de"}}, 200)
	publicRegion("DE")
	call(r.c, "PUT", path, map[string]any{"RegionMode": "auto"}, 200)
	publicRegion("JP")
	// Old clients preserve the mode when echoing the current region.
	call(r.c, "PUT", path, map[string]any{"Info": map[string]string{"region": "JP"}}, 200)
	if read().RegionMode != "auto" {
		t.Fatal("old client cleared mode")
	}
	call(r.c, "PUT", path, map[string]any{"Info": map[string]string{"region": "FR"}}, 200)
	if read().RegionMode != "manual" {
		t.Fatal("old manual edit lost")
	}
}
