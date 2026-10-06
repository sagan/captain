package http

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestProbePeriodTrafficSurvivesCounterRestart(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	settings := func(sections []string) {
		t.Helper()
		if code, b, _ := r.c.do("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "public_sections": sections}, nil); code != 200 {
			t.Fatalf("settings: %d %s", code, b)
		}
	}
	settings([]string{"traffic"}) // Persisted usage does not require live network metrics.
	read := func(c *client, up, down, used int64) {
		t.Helper()
		code, b, _ := c.do("GET", "/api/probe", nil, nil)
		if code != 200 {
			t.Fatalf("snapshot: %d %s", code, b)
		}
		type snapshot struct {
			Nodes []struct {
				ID      int64          `json:"id"`
				Traffic map[string]any `json:"traffic"`
			} `json:"nodes"`
		}
		doc := mustJSON[snapshot](t, b)
		for _, n := range doc.Nodes {
			if n.ID != r.nodeID {
				continue
			}
			for key, want := range map[string]int64{"used_up": up, "used_down": down, "used": used} {
				if n.Traffic[key] != float64(want) {
					t.Fatalf("%s: got %v, want %d", key, n.Traffic[key], want)
				}
			}
			return
		}
		t.Fatal("missing node")
	}
	at := time.Now().UTC()
	beat := func(epoch, id string, seq, up, down uint64) {
		t.Helper()
		h := spec.SystemStatus{NetTotalUp: up, NetTotalDown: down, Resources: &spec.Resources{Epoch: epoch, Sequence: seq, Networks: []spec.NetworkResource{{Name: "eth0", ID: id, Included: true, Up: up, Down: down}}}}
		if err := r.st.RecordBeat(ctx, r.nodeID, h, at); err != nil {
			t.Fatal(err)
		}
	}
	beat("agent-1", "boot-1:1", 1, 1000000, 2000000)
	read(r.c, 0, 0, 0) // Explicit zero fields, not omitted or filled from OS counters.
	beat("agent-1", "boot-1:1", 2, 1000300, 2000500)
	read(r.c, 300, 500, 800)
	beat("agent-2", "boot-2:1", 1, 10, 20)
	read(r.c, 300, 500, 800)
	beat("agent-2", "boot-2:1", 2, 80, 130)
	read(r.c, 370, 610, 980)
	for mode, used := range map[string]int64{"sum": 980, "up": 370, "down": 610, "max": 610} {
		code, b, _ := r.c.do("PUT", "/api/admin/nodes/"+itoa(r.nodeID)+"/probe", map[string]any{"Mode": mode, "ResetDay": 1}, nil)
		if code != 200 {
			t.Fatalf("mode: %d %s", code, b)
		}
		read(r.c, 370, 610, used)
	}
	// The additive direction fields must follow traffic visibility, not network.
	settings([]string{"network"})
	read(&client{t: t, srv: r.srv}, 0, 0, 0)
	settings([]string{"traffic"})
	p, err := r.st.NodeProbe(ctx, r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	read(r.c, 370, 610, p.Billed())
	if err := r.st.ResetNodeTraffic(ctx, r.nodeID, at); err != nil {
		t.Fatal(err)
	}
	read(r.c, 0, 0, 0)
}
