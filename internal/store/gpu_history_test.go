package store

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestGPUHistoryDeduplicatesCachedReadingsAndKeepsUnknown(t *testing.T) {
	s, id, _ := monitorRig(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Minute).Add(10 * time.Second)
	load, temp := 0.0, -10.0
	h := spec.SystemStatus{Valid: &spec.MetricValidity{}, Resources: &spec.Resources{Epoch: "host", Sequence: 1, At: at.UnixMilli(), GPU: &spec.GPUStatus{Epoch: "gpu", Sequence: 1, At: at.UnixMilli(), State: "ok", Devices: []spec.GPUResource{{ID: "gpu1", Utilization: &load, Temperature: &temp}}}}}
	if err := s.RecordBeat(ctx, id, h, at); err != nil {
		t.Fatal(err)
	}
	h.Resources.Sequence++
	h.Resources.At += 1000
	if err := s.RecordBeat(ctx, id, h, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	// Reopen store while retaining database: GPU cursor survives a panel restart.
	s = New(s.db)
	h.Resources.Sequence++
	h.Resources.At += 1000
	if err := s.RecordBeat(ctx, id, h, at.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	history, err := s.ResourceHistory(ctx, id, "1h", "gpu:gpu1:utilization", at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	samples := 0
	for _, p := range history.Points {
		if p.Samples > 0 {
			samples += p.Samples
			if p.Average == nil || *p.Average != 0 {
				t.Fatal("measured zero lost", p)
			}
		}
	}
	if samples != 1 {
		t.Fatal("cached GPU inflated history", samples)
	}
	temp = -5
	h.Resources.GPU.Sequence++
	h.Resources.GPU.At += 15000
	h.Resources.Sequence++
	h.Resources.At = h.Resources.GPU.At
	if err = s.RecordBeat(ctx, id, h, at.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	history, err = s.ResourceHistory(ctx, id, "1h", "gpu:gpu1:temperature", at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range history.Points {
		if p.Samples > 0 && (p.Samples != 2 || *p.Average != -7.5 || *p.Peak != -5) {
			t.Fatal("temperature aggregate", p)
		}
	}
	h.Resources.GPU.State = "error"
	h.Resources.GPU.Devices = nil
	h.Resources.GPU.Sequence++
	h.Resources.Sequence++
	h.Resources.At += 15000
	h.Resources.GPU.At = h.Resources.At
	if err = s.RecordBeat(ctx, id, h, at.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	history, err = s.ResourceHistory(ctx, id, "1h", "gpu:gpu1:utilization", at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	samples = 0
	for _, p := range history.Points {
		samples += p.Samples
	}
	if samples != 2 {
		t.Fatal("failed sample became zero", samples)
	}
	if freshGPU(&spec.Resources{At: at.UnixMilli() + 70000, GPU: &spec.GPUStatus{At: at.UnixMilli(), Epoch: "gpu", Sequence: 5}}, gpuCursor{}) {
		t.Fatal("stale GPU accepted")
	}
}

func TestGPUSnapshotClockAndIsolation(t *testing.T) {
	at := time.Now()
	clock := at.Add(2 * time.Hour).UnixMilli()
	h := spec.SystemStatus{Resources: &spec.Resources{At: clock, GPU: &spec.GPUStatus{State: "ok", At: clock - 17000}}}
	out := MonitorSnapshot(h, at)
	if out.Resources.At != at.UnixMilli() || out.Resources.GPU.At != at.Add(-17*time.Second).UnixMilli() {
		t.Fatal(out.Resources)
	}
	if h.Resources.At != clock || h.Resources.GPU.At != clock-17000 {
		t.Fatal("changed caller")
	}
	h.Resources.GPU.At = clock - 90000
	out = MonitorSnapshot(h, at)
	if out.Resources.GPU.At != at.Add(-90*time.Second).UnixMilli() {
		t.Fatal("stale GPU became fresh")
	}
}
