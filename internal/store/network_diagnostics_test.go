package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestNetworkDiagnosticAdmissionExpiryRetention(t *testing.T) {
	s, id, _ := monitorRig(t)
	ctx := context.Background()
	req := spec.DiagnosticRequest{Type: "tcp", Target: "example.com:443"}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.QueueNetworkDiagnostic(ctx, fmt.Sprint(i), id, req)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrDiagnosticPending) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatal("parallel admission", accepted.Load())
	}
	jobs, err := s.NetworkDiagnostics(ctx, id)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	old := jobs[0].ID
	if _, err = s.db.Exec(`UPDATE node_jobs SET created_at=? WHERE id=?`, time.Now().Add(-3*time.Minute).Unix(), old); err != nil {
		t.Fatal(err)
	}
	if jobs, err = s.PendingNodeJobs(ctx, id); err != nil || len(jobs) != 0 {
		t.Fatal("expired delivered", jobs, err)
	}
	if err = s.CompleteReportedNodeJob(ctx, id, old, []byte(`{"late":true}`), ""); err != nil {
		t.Fatal(err)
	}
	j, err := s.NodeJob(ctx, id, old)
	if err != nil || j.DoneAt == nil || j.Error != "diagnostic request expired" || len(j.Result) != 0 {
		t.Fatal(j, err)
	}
	// History is strictly bounded; neither diagnostic nor general job pruning
	// can erase the successful external worker result for node removal.
	if err = s.CreateNodeJob(ctx, "removal", id, NodeRemovalKind, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteNodeJob(ctx, id, "removal", []byte(`{"phase":"complete"}`), ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		jobID := fmt.Sprintf("new-%d", i)
		if err = s.QueueNetworkDiagnostic(ctx, jobID, id, req); err != nil {
			t.Fatal(err)
		}
		if err = s.CompleteNodeJob(ctx, id, jobID, []byte(`{"outcome":"ok"}`), ""); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err = s.NetworkDiagnostics(ctx, id)
	if err != nil || len(jobs) != 20 || jobs[0].ID != "new-24" {
		t.Fatal(len(jobs), err)
	}
	if err = s.PruneStats(ctx, time.Now().Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	jobs, err = s.NetworkDiagnostics(ctx, id)
	if err != nil || len(jobs) != 0 {
		t.Fatal(jobs, err)
	}
	if j, err = s.NodeJob(ctx, id, "removal"); err != nil || j.DoneAt == nil {
		t.Fatal("lost removal", j, err)
	}
}
