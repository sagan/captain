package http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestProbeAppearanceAndGPUPrivacy(t *testing.T) {
	for _, preset := range []string{"terminal", "glass"} {
		t.Run(preset, func(t *testing.T) {
			r := newRig(t)
			path := "/api/admin/settings/probe"
			settings := map[string]any{"enabled": true, "appearance": map[string]string{"preset": preset, "scheme": "light"}}
			if code, b, _ := r.c.do("PUT", path, settings, nil); code != 200 {
				t.Fatal(code, string(b))
			}
			// Old clients cannot reset the chosen appearance by omitting its new field.
			if code, b, _ := r.c.do("PUT", path, map[string]any{"enabled": true}, nil); code != 200 {
				t.Fatal(code, string(b))
			}
			_, b, _ := r.c.do("GET", path, nil, nil)
			if !strings.Contains(string(b), `"preset":"`+preset+`"`) || !strings.Contains(string(b), `"scheme":"light"`) {
				t.Fatal(string(b))
			}
			for _, bad := range []map[string]string{{"preset": "url(https://example.com)", "scheme": "dark"}, {"preset": "paper", "scheme": "script"}} {
				settings["appearance"] = bad
				if code, _, _ := r.c.do("PUT", path, settings, nil); code != 400 {
					t.Fatal("unvalidated appearance", code)
				}
			}
			nodePath := "/api/admin/nodes/" + itoa(r.nodeID) + "/probe"
			if code, b, _ := r.c.do("PUT", nodePath, map[string]any{"Resources": spec.ResourceOptions{GPU: true}}, nil); code != 200 {
				t.Fatal(code, string(b))
			}
			_, b, _ = r.agent.do("GET", "/api/agent/state", nil, nil)
			state := mustJSON[agentproto.State](t, b)
			if state.Probe == nil || state.Probe.Resources == nil || !state.Probe.Resources.GPU {
				t.Fatal("GPU option not delivered", string(b))
			}
			value := 0.0
			at := time.Now().UnixMilli()
			h := spec.SystemStatus{Resources: &spec.Resources{Epoch: "host", Sequence: 1, At: at, GPU: &spec.GPUStatus{Epoch: "gpu", Sequence: 1, At: at, State: "ok", Devices: []spec.GPUResource{{ID: "nvidia:GPU-private", Name: "Private GPU", Utilization: &value}}}}}
			if code, b, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: h}, nil); code != 204 {
				t.Fatal("GPU beat rejected", code, string(b))
			}
			anon := &client{t: t, srv: r.srv}
			code, b, _ := anon.do("GET", "/api/probe", nil, nil)
			if code != 200 || !strings.Contains(string(b), `"preset":"`+preset+`"`) || strings.Contains(string(b), "GPU-private") || strings.Contains(string(b), `"resources"`) {
				t.Fatal("public appearance/privacy", code, string(b))
			}
			_, b, _ = r.c.do("GET", nodePath, nil, nil)
			if !strings.Contains(string(b), `"gpu":true`) || !strings.Contains(string(b), "GPU-private") {
				t.Fatal("private GPU setting or snapshot lost", string(b))
			}
		})
	}
}

func TestProbeResolvesInheritedSchemeWithoutSiteEndpoint(t *testing.T) {
	r := newRig(t)
	if err := r.st.SetSetting(context.Background(), "site", map[string]any{"theme": map[string]string{"site_scheme": "auto"}}); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{"enabled": true, "path": "/status", "appearance": map[string]string{"preset": "glass", "scheme": "inherit"}}
	if code, b, _ := r.c.do("PUT", "/api/admin/settings/probe", settings, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	anon := &client{t: t, srv: r.srv}
	code, b, _ := anon.do("GET", "/api/probe", nil, nil)
	if code != 200 || mustJSON[map[string]any](t, b)["appearance_scheme"] != "auto" {
		t.Fatal(code, string(b))
	}
}
