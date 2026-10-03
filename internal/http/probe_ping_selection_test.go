package http

import (
	"context"
	"strings"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/store"
)

type probePingTaskTestView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type probePingTestDoc struct {
	Nodes []struct {
		ID    int64                   `json:"id"`
		Tasks []probePingTaskTestView `json:"ping_tasks"`
	} `json:"nodes"`
}

func TestProbePublicPingSelection(t *testing.T) {
	r := newRig(t)
	tasks := []store.PingTask{
		{Name: "Disabled", Type: "tcp", Target: "private.example.com:443", Enabled: false},
		{Name: "Other node", Type: "tcp", Target: "private.example.com:443", Enabled: true, NodeIDs: []int64{r.nodeID + 100}},
		{Name: "First", Type: "tcp", Target: "private.example.com:443", Enabled: true, NodeIDs: []int64{r.nodeID}},
		{Name: "Second", Type: "tcp", Target: "private.example.com:443", Enabled: true},
	}
	for i := range tasks {
		if err := r.st.SavePingTask(context.Background(), &tasks[i]); err != nil {
			t.Fatal(err)
		}
	}
	settings := map[string]any{"enabled": true, "carrier_ping": false}
	set := func() {
		t.Helper()
		if code, b, _ := r.c.do("PUT", "/api/admin/settings/probe", settings, nil); code != 200 {
			t.Fatal(code, string(b))
		}
	}
	read := func() []probePingTaskTestView {
		t.Helper()
		anon := &client{t: t, srv: r.srv}
		code, b, _ := anon.do("GET", "/api/probe", nil, nil)
		if code != 200 || strings.Contains(string(b), "private.example.com") || strings.Contains(string(b), "node_ids") {
			t.Fatal("public probe leaks private configuration", code, string(b))
		}
		doc := mustJSON[probePingTestDoc](t, b)
		for _, n := range doc.Nodes {
			if n.ID == r.nodeID {
				return n.Tasks
			}
		}
		t.Fatal("missing node")
		return nil
	}
	set()
	got := read()
	if len(got) != 2 || got[0].ID != tasks[2].ID || got[1].ID != tasks[3].ID {
		t.Fatal("wrong enabled/assigned task order", got)
	}
	settings["carrier_ping"] = true
	set()
	got = read()
	if len(got) != 5 || got[0].Name != "CT" || got[1].Name != "CU" || got[2].Name != "CM" {
		t.Fatal("carrier identities require no samples", got)
	}
	settings["carriers"] = []spec.Carrier{{Name: "Custom", Addr: "private.example.com:443"}}
	set()
	if got = read(); len(got) != 3 || got[0].Name != "Custom" {
		t.Fatal("custom label lost", got)
	}
	settings["public_sections"] = []string{"system"}
	set()
	if got = read(); len(got) != 0 {
		t.Fatal("hidden latency leaked tasks", got)
	}
}
