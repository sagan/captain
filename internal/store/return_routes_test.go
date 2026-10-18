package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestReturnRouteScheduleAndHistoryIsolation(t *testing.T) {
	s, id, _ := monitorRig(t)
	ctx := context.Background()
	at := time.Now()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.db.Exec(`UPDATE nodes SET token_hash='paired',version=?,last_seen_at=? WHERE id=?`, spec.ReturnRouteMinimumVersion, at.Unix(), id)
	must(err)
	p := RouteSchedule{Enabled: true, IntervalHours: 6, City: "beijing", Family: "ipv6"}
	must(s.SetRouteSchedule(ctx, id, p))
	must(s.ScheduleReturnRoutes(ctx, at))
	h, err := s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 0 {
		t.Fatal("enabling ran immediately")
	}
	_, err = s.db.Exec(`UPDATE node_route_schedules SET next_at=? WHERE node_id=?`, at.Add(-time.Minute).Unix(), id)
	must(err)
	must(s.ScheduleReturnRoutes(ctx, at))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 1 {
		t.Fatal("missing schedule", len(h))
	}
	var params spec.DiagnosticParams
	must(json.Unmarshal(h[0].Params, &params))
	if params.Route == nil || params.Route.Family != "ipv6" {
		t.Fatal(params)
	}
	must(s.ScheduleReturnRoutes(ctx, at))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 1 {
		t.Fatal("duplicate schedule")
	}
	must(s.CompleteNodeJob(ctx, id, h[0].ID, []byte(`{"type":"return_route","outcome":"ok"}`), ""))
	ordinary, err := s.NetworkDiagnostics(ctx, id)
	must(err)
	if len(ordinary) != 0 {
		t.Fatal("route mixed into ordinary diagnostics")
	}
	must(s.PruneStats(ctx, at.Add(25*time.Hour)))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 1 {
		t.Fatal("ordinary retention erased route")
	}
	// A busy diagnostic defers the schedule, rather than dropping a run or
	// allowing two jobs to race on the node.
	must(s.QueueNetworkDiagnostic(ctx, "ordinary", id, spec.DiagnosticRequest{Type: "tcp", Target: "example.com:443"}))
	_, err = s.db.Exec(`UPDATE node_route_schedules SET next_at=? WHERE node_id=?`, at.Unix()-1, id)
	must(err)
	must(s.ScheduleReturnRoutes(ctx, at))
	var next int64
	must(s.db.QueryRow(`SELECT next_at FROM node_route_schedules WHERE node_id=?`, id).Scan(&next))
	if next != at.Add(5*time.Minute).Unix() {
		t.Fatal("busy retry", next)
	}
	must(s.CompleteNodeJob(ctx, id, "ordinary", []byte(`{}`), ""))
	// Unsupported agents cannot receive new scheduled checks.
	_, err = s.db.Exec(`UPDATE nodes SET version='v0.68.1' WHERE id=?`, id)
	must(err)
	_, err = s.db.Exec(`UPDATE node_route_schedules SET next_at=? WHERE node_id=?`, at.Unix()-1, id)
	must(err)
	must(s.ScheduleReturnRoutes(ctx, at))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 1 {
		t.Fatal("old node received check")
	}
	// Re-pairing preserves policy but the unpaired node receives no schedule.
	must(s.ResetPairCode(ctx, id, "new-code", time.Hour))
	_, err = s.db.Exec(`UPDATE node_route_schedules SET next_at=? WHERE node_id=?`, at.Unix()-1, id)
	must(err)
	must(s.ScheduleReturnRoutes(ctx, at))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 1 {
		t.Fatal("unpaired node received check")
	}
	must(s.ScheduleReturnRoutes(ctx, at.Add(31*24*time.Hour)))
	h, err = s.ReturnRoutes(ctx, id)
	must(err)
	if len(h) != 0 {
		t.Fatal("expired history retained")
	}
}

func TestReturnRouteHistoryBoundAndDisabledSchedule(t *testing.T) {
	s, id, _ := monitorRig(t)
	ctx := context.Background()
	req := spec.DiagnosticRequest{Type: "return_route", Route: &spec.ReturnRouteRequest{City: "beijing", Family: "ipv4"}}
	for i := 0; i < 125; i++ {
		jobID := fmt.Sprintf("route-%d", i)
		if err := s.QueueNetworkDiagnostic(ctx, jobID, id, req); err != nil {
			t.Fatal(err)
		}
		if err := s.CompleteNodeJob(ctx, id, jobID, []byte(`{}`), ""); err != nil {
			t.Fatal(err)
		}
	}
	h, err := s.ReturnRoutes(ctx, id)
	if err != nil || len(h) != 120 || h[0].ID != "route-124" {
		t.Fatal(len(h), err)
	}
	at := time.Now()
	_, err = s.db.Exec(`UPDATE nodes SET token_hash='paired',version=?,last_seen_at=? WHERE id=?`, spec.ReturnRouteMinimumVersion, at.Unix(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRouteSchedule(ctx, id, RouteSchedule{Enabled: false, IntervalHours: 6, City: "beijing", Family: "ipv4"}); err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`UPDATE node_route_schedules SET next_at=? WHERE node_id=?`, at.Unix()-1, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ScheduleReturnRoutes(ctx, at); err != nil {
		t.Fatal(err)
	}
	pending, err := s.PendingNodeJobs(ctx, id)
	if err != nil || len(pending) != 0 {
		t.Fatal("disabled scheduled", pending, err)
	}
}
