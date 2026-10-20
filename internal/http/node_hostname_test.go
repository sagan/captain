package http

import (
	"context"
	"strings"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
)

func TestNodeHostnameReports(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	original, err := r.st.NodeByID(ctx, r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	report := func(input any, want string) {
		t.Helper()
		status, body, _ := r.agent.do("POST", "/api/agent/report", input, nil)
		if status != 200 {
			t.Fatalf("report %d %s", status, body)
		}
		node, err := r.st.NodeByID(ctx, r.nodeID)
		if err != nil {
			t.Fatal(err)
		}
		if node.Hostname != want || node.Name != original.Name {
			t.Fatalf("hostname=%q name=%q", node.Hostname, node.Name)
		}
	}
	report(agentproto.Report{Hostname: "example-old"}, "example-old")
	report(agentproto.Report{Hostname: "example-new"}, "example-new")
	report(map[string]any{}, "example-new")
	for _, value := range []string{"", " ", "bad\nname", strings.Repeat("x", 256)} {
		report(agentproto.Report{Hostname: value}, "example-new")
	}
	if err := r.st.ObserveNodeHostname(ctx, r.nodeID, "obsolete-pairing", "example-stale"); err != nil {
		t.Fatal(err)
	}
	node, err := r.st.NodeByID(ctx, r.nodeID)
	if err != nil || node.Hostname != "example-new" {
		t.Fatal("stale identity changed hostname", err)
	}
}
