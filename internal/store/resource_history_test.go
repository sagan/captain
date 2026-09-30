package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestResourceHistoryPersistenceGapsPeaksAndRetention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	conn, err := db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	node := &domain.Node{Name: "history"}
	if err = s.CreateNode(ctx, node, "PAIR", time.Hour); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	f := func(v float64) *float64 { return &v }
	h := spec.SystemStatus{CPUPercent: 40, Valid: &spec.MetricValidity{CPU: true}, Resources: &spec.Resources{Epoch: "run", Sequence: 1, At: base.Add(365 * 24 * time.Hour).UnixMilli(), Networks: []spec.NetworkResource{{Name: "eth0", UpRate: f(0), DownRate: f(100)}}}}
	record := func(at time.Time) {
		t.Helper()
		if err := s.RecordBeat(ctx, node.ID, h, at); err != nil {
			t.Fatal(err)
		}
	}
	record(base)
	h.Resources.Sequence++
	h.CPUPercent = 0
	h.Resources.Networks[0].DownRate = f(300)
	record(base.Add(20 * time.Second))
	if err := s.RecordBeat(ctx, node.ID, h, base.Add(25*time.Second)); !errors.Is(err, ErrStaleBeat) {
		t.Fatal(err)
	}
	h.Resources.Sequence++
	h.Valid.CPU = false
	h.CPUPercent = 99
	h.Resources.Networks = nil
	record(base.Add(40 * time.Second))
	h.Resources.Sequence++
	h.Valid.CPU = true
	h.CPUPercent = 90
	record(base.Add(2 * time.Minute))
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	conn, err = db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s = New(conn)
	at := base.Add(2 * time.Minute)
	hist, err := s.ResourceHistory(ctx, node.ID, "1h", "host::cpu", at)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist.Points) != 60 {
		t.Fatal(len(hist.Points))
	}
	a, gap, c := hist.Points[57], hist.Points[58], hist.Points[59]
	if a.Samples != 2 || a.Average == nil || *a.Average != 20 || *a.Peak != 40 || gap.Average != nil || gap.Peak != nil || gap.Samples != 0 || *c.Average != 90 {
		t.Fatalf("buckets: %+v %+v %+v", a, gap, c)
	}
	for _, span := range []string{"7d", "30d"} {
		hist, err = s.ResourceHistory(ctx, node.ID, span, "host::cpu", at)
		if err != nil {
			t.Fatal(err)
		}
		p := hist.Points[len(hist.Points)-1]
		if p.Samples != 3 || math.Abs(*p.Average-130.0/3) > 1e-6 || *p.Peak != 90 {
			t.Fatalf("rollup %s %+v", span, p)
		}
	}
	hist, err = s.ResourceHistory(ctx, node.ID, "1h", "network:eth0:down", at)
	if err != nil {
		t.Fatal(err)
	}
	p := hist.Points[57]
	if p.Samples != 2 || *p.Average != 200 || *p.Peak != 300 || hist.Points[59].Average != nil {
		t.Fatalf("NIC history: %+v", hist)
	}
	hist, err = s.ResourceHistory(ctx, node.ID, "1h", "network:eth0:up", at)
	if err != nil || hist.Points[57].Average == nil || *hist.Points[57].Average != 0 {
		t.Fatal("zero lost", err)
	}
	nodes, err := s.MonitorNodes(ctx, at, time.Minute)
	if err != nil || len(nodes) != 1 || nodes[0].SampledAt != at.Unix() || nodes[0].Host.CPUPercent != 90 || nodes[0].Paired {
		t.Fatalf("persisted snapshot: %+v %v", nodes, err)
	}
	// Existing aggregate statistics keep their longer retention.
	if err = s.PruneStats(ctx, base.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var detailed, legacy int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM node_resource_stats WHERE res='m'`).Scan(&detailed)
	_ = conn.QueryRow(`SELECT COUNT(*) FROM node_stats WHERE res='m'`).Scan(&legacy)
	if detailed != 0 || legacy == 0 {
		t.Fatalf("retention %d %d", detailed, legacy)
	}
	if err = s.PruneStats(ctx, base.Add(91*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	_ = conn.QueryRow(`SELECT COUNT(*) FROM node_resource_stats`).Scan(&detailed)
	if detailed != 0 {
		t.Fatal(detailed)
	}
	if err = s.RecordBeat(ctx, node.ID, h, base.Add(92*24*time.Hour)); !errors.Is(err, ErrStaleBeat) {
		t.Fatal("pruning lost replay protection", err)
	}
}

func TestResourceHistoryCardinalityAndDelete(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "bounded"}
	if err = s.CreateNode(ctx, n, "PAIR", time.Hour); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	h := spec.SystemStatus{CPUPercent: 10, Resources: &spec.Resources{Epoch: "run"}}
	for seq := 1; seq <= 2; seq++ {
		h.Resources.Sequence = uint64(seq)
		if seq == 2 {
			h.MemTotal = 100
			h.MemUsed = 20
		}
		h.Resources.CPUs = nil
		for i := 0; i < 700; i++ {
			v := float64(i)
			h.Resources.CPUs = append(h.Resources.CPUs, spec.CPUResource{Name: fmt.Sprintf("cpu%d-%d", seq, i), Percent: &v})
		}
		if err = s.RecordBeat(ctx, n.ID, h, at); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := s.ResourceHistory(ctx, n.ID, "1h", "host::cpu", at)
	if err != nil {
		t.Fatal(err)
	}
	if !hist.Truncated || len(hist.Series) > maxResourceSeries || hist.Points[len(hist.Points)-1].Samples != 2 {
		t.Fatalf("unbounded %+v", hist)
	}
	memory, err := s.ResourceHistory(ctx, n.ID, "1h", "host::memory", at)
	if err != nil || memory.Points[len(memory.Points)-1].Average == nil || *memory.Points[len(memory.Points)-1].Average != 20 {
		t.Fatal("newly available summary crowded out by devices", err)
	}
	if _, err = conn.Exec(`DELETE FROM nodes WHERE id=?`, n.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM node_resource_stats`).Scan(&count)
	if count != 0 {
		t.Fatal("orphan resource history", count)
	}
}

// A representative node, including history compression and SQLite commits.
func BenchmarkResourceBeat(b *testing.B) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		b.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "bench"}
	if err = s.CreateNode(ctx, n, "PAIR", time.Hour); err != nil {
		b.Fatal(err)
	}
	rate := float64(1024)
	used, total := uint64(1<<30), uint64(8<<30)
	h := spec.SystemStatus{CPUPercent: 40, MemUsed: used, MemTotal: total, DiskUsed: used, DiskTotal: total, Resources: &spec.Resources{Epoch: "bench"}}
	for i := 0; i < 16; i++ {
		h.Resources.CPUs = append(h.Resources.CPUs, spec.CPUResource{Name: fmt.Sprintf("cpu%d", i), Percent: &rate})
	}
	for i := 0; i < 4; i++ {
		h.Resources.Networks = append(h.Resources.Networks, spec.NetworkResource{Name: fmt.Sprintf("eth%d", i), ID: fmt.Sprint(i), Included: true, UpRate: &rate, DownRate: &rate})
		h.Resources.Disks = append(h.Resources.Disks, spec.DiskResource{Name: fmt.Sprintf("disk%d", i), ReadRate: &rate, WriteRate: &rate, ReadIOPS: &rate, WriteIOPS: &rate})
		h.Resources.Filesystems = append(h.Resources.Filesystems, spec.FilesystemResource{Mount: fmt.Sprintf("/data%d", i), Used: &used, Total: &total})
		h.Resources.Processes = append(h.Resources.Processes, spec.ProcessResource{Name: fmt.Sprintf("core%d", i), CPU: &rate, RSS: &used})
	}
	at := time.Now().UTC().Truncate(time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Resources.Sequence = uint64(i + 1)
		if err = s.RecordBeat(ctx, n.ID, h, at.Add(time.Duration(i)*10*time.Second)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	var size int64
	_ = conn.QueryRow(`SELECT MAX(length(data)) FROM node_resource_stats`).Scan(&size)
	b.ReportMetric(float64(size), "bytes/bucket")
}
