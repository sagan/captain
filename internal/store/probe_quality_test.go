package store

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestNetworkHistoryQueueDedupeClockAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "network.db")
	conn, err := db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "network"}
	if err = s.CreateNode(ctx, n, "PAIR", time.Hour); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 10, 1, 10, 0, time.UTC)
	skew := at.Add(6 * time.Hour)
	dns, connect := 2.0, 8.0
	sample := func(seq uint64, age time.Duration, latency float64, outcome string) spec.ProbeMeasurement {
		return spec.ProbeMeasurement{Sequence: seq, At: skew.Add(-age).Unix(), LatencyMs: latency, Outcome: outcome, Timings: &spec.ProbeTimings{DNS: &dns, Connect: &connect}}
	}
	samples := []spec.ProbeMeasurement{sample(1, 50*time.Second, 10, "ok"), sample(2, 40*time.Second, 20, "ok"), sample(3, 30*time.Second, -1, "timeout"), sample(4, 0, 100, "ok")}
	q := &spec.PingQuality{ProbeMeasurement: samples[3], Epoch: "run", Type: "tcp", Recent: samples, IntervalSeconds: 10}
	h := spec.SystemStatus{Resources: &spec.Resources{Epoch: "host", Sequence: 1, At: skew.UnixMilli()}, Pings: []spec.PingResult{{TaskID: 1, Name: "edge", LatencyMs: 100, At: q.At, Quality: q}}}
	record := func() {
		t.Helper()
		if err := s.RecordBeat(ctx, n.ID, h, at); err != nil {
			t.Fatal(err)
		}
	}
	record()
	// Different host sample, identical probe batch: no duplicate network rows.
	h.Resources.Sequence++
	record()
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	conn, err = db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s = New(conn)
	h.Resources.Sequence++
	record()
	points, err := s.NodePingStats(ctx, n.ID, "m", at.Add(-time.Hour), at)
	if err != nil || len(points) != 2 {
		t.Fatalf("%+v %v", points, err)
	}
	a, b := points[0], points[1]
	if a.Samples != 3 || a.Lost != 1 || a.AvgMs != 15 || a.TS != at.Add(-time.Minute).Truncate(time.Minute).Unix() || b.Samples != 1 || b.AvgMs != 100 {
		t.Fatalf("dedupe / clock: %+v", points)
	}
	if a.Quality == nil || a.Quality.Samples != 3 || *a.Quality.Jitter != 10 || *a.Quality.Min != 10 || *a.Quality.Max != 20 || *a.Quality.P95 != 20 || a.Quality.Outcomes["timeout"] != 1 || *a.Quality.Timings.DNS != 2 || a.Quality.Timings.TLS != nil || b.Quality.Jitter != nil {
		t.Fatalf("quality: %+v %+v", a.Quality, b.Quality)
	}
	nodes, err := s.MonitorNodes(ctx, at, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	p := nodes[0].Host.Pings[0]
	if p.At != at.Unix() || len(p.Quality.Recent) != 0 || p.Quality.At != p.At {
		t.Fatalf("queue/time leaked into snapshot: %+v", p)
	}
	// Missing transport sequences are unknown, never counted as target failures.
	next := sample(7, 0, 120, "ok")
	q.ProbeMeasurement = next
	q.Recent = []spec.ProbeMeasurement{next}
	h.Resources.Sequence++
	record()
	points, _ = s.NodePingStats(ctx, n.ID, "m", at.Add(-time.Hour), at)
	b = points[1]
	if b.Samples != 2 || b.Lost != 0 || b.Quality.Skipped != 2 || b.Quality.Jitter != nil {
		t.Fatalf("dropped samples became loss/jitter %+v %+v", b, b.Quality)
	}
	// A new epoch starts a new jitter baseline and accepts sequence 1.
	next.Sequence = 1
	q.ProbeMeasurement = next
	q.Recent = nil
	q.Epoch = "reconfigured"
	h.Resources.Sequence++
	record()
	points, _ = s.NodePingStats(ctx, n.ID, "m", at.Add(-time.Hour), at)
	if points[1].Samples != 3 || points[1].Quality.Jitter != nil {
		t.Fatal("epoch baseline", points[1])
	}
	// Corrupt detail must not silently fall back to legacy counting.
	q.ProbeMeasurement.LatencyMs = math.NaN()
	h.Resources.Sequence++
	// NaN can't enter HTTP JSON, so test the network writer transaction directly.
	tx, _ := conn.BeginTx(ctx, nil)
	if err := recordPingHistory(ctx, tx, n.ID, h, at); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	points, _ = s.NodePingStats(ctx, n.ID, "m", at.Add(-time.Hour), at)
	if points[1].Samples != 3 {
		t.Fatal("invalid sample counted")
	}
	if err = s.TouchNode(ctx, n.ID, "", "", spec.SystemStatus{CPUPercent: 55}, nil, nil); err != nil {
		t.Fatal(err)
	}
	latest, err := s.MonitorNodes(ctx, time.Now(), time.Minute, n.ID)
	if err != nil || len(latest) != 1 || len(latest[0].Host.Pings) != 1 || latest[0].Host.CPUPercent != 55 {
		t.Fatal("new minute report erased independent probe snapshot", err)
	}
	if _, err = conn.Exec(`DELETE FROM nodes WHERE id=?`, n.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM node_ping_cursors`).Scan(&count)
	if count != 0 {
		t.Fatal("orphan cursor")
	}
}

func TestNetworkHistogramErrorAndFailureOnly(t *testing.T) {
	a := networkAggregate{}
	for i := 1; i <= 1000; i++ {
		a.add(spec.ProbeMeasurement{LatencyMs: float64(i), Outcome: "ok"}, "tcp", nil, 0)
	}
	q := a.summary()
	if *q.P50 < 500 || *q.P50 > 525 || *q.P95 < 950 || *q.P95 > 997.5 {
		t.Fatalf("unbounded quantile error %+v", q)
	}
	a = networkAggregate{}
	a.add(spec.ProbeMeasurement{LatencyMs: -1, Outcome: "timeout"}, "http", nil, 0)
	q = a.summary()
	if q.P50 != nil || q.P95 != nil || q.Min != nil || q.Jitter != nil || q.Samples != 1 {
		t.Fatal(q)
	}
}

func TestLegacyTaskTimestampDedupePreservesUnknownQuality(t *testing.T) {
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "old"}
	if err = s.CreateNode(ctx, n, "PAIR", time.Hour); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	for i := 0; i < 3; i++ {
		if err = s.RecordBeat(ctx, n.ID, spec.SystemStatus{Pings: []spec.PingResult{{TaskID: 1, Name: "old task", At: at.Unix(), LatencyMs: 10}}}, at); err != nil {
			t.Fatal(err)
		}
	}
	points, err := s.NodePingStats(ctx, n.ID, "m", at.Add(-time.Minute), at)
	if err != nil || len(points) != 1 || points[0].Samples != 1 || points[0].Quality != nil {
		t.Fatalf("legacy history %+v %v", points, err)
	}
}
