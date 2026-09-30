package http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNetworkQualityAPIAndPublicSections(t *testing.T) {
	r := newRig(t)
	anon := &client{t: t, srv: r.srv}
	if code, b, _ := r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "page_enabled": true, "path": "/status", "public_sections": []string{"latency", "history"}}, nil); code != 200 {
		t.Fatalf("settings %d %s", code, b)
	}
	at := time.Now()
	dns, connect := 1.0, 2.0
	m := spec.ProbeMeasurement{Sequence: 1, At: at.Unix(), LatencyMs: -1, Outcome: "http_status", HTTPStatus: 503, DurationMs: 30, Timings: &spec.ProbeTimings{DNS: &dns, Connect: &connect}}
	q := &spec.PingQuality{ProbeMeasurement: m, Epoch: "run", Type: "http", IntervalSeconds: 30, Window: spec.ProbeWindow{Attempts: 1, Failed: 1}, Recent: []spec.ProbeMeasurement{m}}
	host := spec.SystemStatus{Resources: &spec.Resources{Epoch: "run", Sequence: 1, At: at.UnixMilli()}, Pings: []spec.PingResult{{TaskID: 1, Name: "web", At: m.At, LatencyMs: -1, Loss: 100, Quality: q}}}
	for i := 1; i <= 2; i++ {
		host.Resources.Sequence = uint64(i)
		if code, b, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: host}, nil); code != 204 {
			t.Fatalf("beat %d %s", code, b)
		}
	}
	privatePath := "/api/admin/nodes/" + itoa(r.nodeID) + "/network-quality?range=1h"
	code, b, _ := r.c.do("GET", privatePath, nil, nil)
	got := mustJSON[struct {
		Current []spec.PingResult `json:"current"`
		Points  []store.PingPoint `json:"points"`
	}](t, b)
	if code != 200 || len(got.Current) != 1 || len(got.Points) != 1 || got.Points[0].Samples != 1 || got.Points[0].Lost != 1 || got.Points[0].Quality.Outcomes["http_status"] != 1 || len(got.Current[0].Quality.Recent) != 0 {
		t.Fatalf("quality API %d %s", code, b)
	}
	// A newer minute host report must not erase separately sampled network data.
	if err := r.st.TouchNode(context.Background(), r.nodeID, "", "", spec.SystemStatus{CPUPercent: 55}, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, b, _ = r.c.do("GET", privatePath, nil, nil)
	if !strings.Contains(string(b), `"http_status":503`) {
		t.Fatalf("minute report erased network measurement %s", b)
	}
	_, b, _ = anon.do("GET", "/api/probe", nil, nil)
	public := mustJSON[struct {
		Nodes []struct {
			Host spec.SystemStatus `json:"host"`
		} `json:"nodes"`
	}](t, b)
	if !strings.Contains(string(b), `"http_status":503`) || len(public.Nodes) != 1 || len(public.Nodes[0].Host.Pings) != 1 || len(public.Nodes[0].Host.Pings[0].Quality.Recent) != 0 {
		t.Fatalf("public result or private transport queue %s", b)
	}
	r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status", "public_sections": []string{"latency"}}, nil)
	_, b, _ = anon.do("GET", "/api/probe", nil, nil)
	if !strings.Contains(string(b), `"http_status":503`) || strings.Contains(string(b), `"attempts":1`) || strings.Contains(string(b), `"loss":100`) {
		t.Fatalf("hidden history leaked rolling statistics %s", b)
	}
	r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status", "public_sections": []string{}}, nil)
	_, b, _ = anon.do("GET", "/api/probe", nil, nil)
	if strings.Contains(string(b), "http_status") {
		t.Fatalf("hidden latency leaked %s", b)
	}
	if code, _, _ := r.c.do("POST", "/api/admin/ping-tasks", map[string]any{"name": "invalid", "type": "http", "target": "file:///private"}, nil); code != 400 {
		t.Fatal("invalid task accepted", code)
	}
}

func TestNetworkBeatAcknowledgesOnlyCommittedMeasurements(t *testing.T) {
	r := newRig(t)
	if code, body, _ := r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true}, nil); code != 200 {
		t.Fatalf("settings %d %s", code, body)
	}
	if _, err := r.st.DB().Exec(`CREATE TRIGGER fail_probe_write BEFORE INSERT ON node_ping_stats BEGIN SELECT RAISE(FAIL, 'simulated storage failure'); END`); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	m := spec.ProbeMeasurement{Sequence: 1, At: at.Unix(), Outcome: "ok", LatencyMs: 12, DurationMs: 13}
	host := spec.SystemStatus{Resources: &spec.Resources{Epoch: "run", Sequence: 1, At: at.UnixMilli()}, Pings: []spec.PingResult{{TaskID: 1, Name: "service", At: m.At, LatencyMs: 12, Quality: &spec.PingQuality{ProbeMeasurement: m, Epoch: "run", Type: "tcp", Recent: []spec.ProbeMeasurement{m}}}}}
	if code, body, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: host}, nil); code != 500 || strings.Contains(string(body), "simulated storage failure") {
		t.Fatalf("storage failure must not acknowledge or leak errors: %d %s", code, body)
	}
	if _, err := r.st.DB().Exec(`DROP TRIGGER fail_probe_write`); err != nil {
		t.Fatal(err)
	}
	// Retry the same resource and probe sequence: the failed transaction must
	// not leave a cursor which would discard its uncommitted attempts.
	for range 2 {
		if code, body, _ := r.agent.do("POST", "/api/agent/beat", agentproto.Beat{Host: host}, nil); code != 204 {
			t.Fatalf("retry %d %s", code, body)
		}
	}
	points, err := r.st.NodePingStats(context.Background(), r.nodeID, "m", at.Add(-time.Minute), at.Add(time.Minute))
	if err != nil || len(points) != 1 || points[0].Samples != 1 {
		t.Fatalf("retry lost or recounted attempts: %+v %v", points, err)
	}
}
