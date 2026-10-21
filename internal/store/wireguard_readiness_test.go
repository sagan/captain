package store

import (
	"context"
	"encoding/json"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestWireGuardReadinessFreshnessAndJobExpiry(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "WG test"}
	if err := s.CreateNode(ctx, n, "pair", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemPairCode(ctx, "pair", "token", "test", "test", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	rep := &agentproto.WireGuardReport{Available: true, Setup: &spec.WireGuardSetup{Installable: true, Packages: []string{"wireguard-tools"}}}
	if err := s.SetWireGuardReport(ctx, n.ID, "token", rep); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE nodes SET wireguard_reported_at=? WHERE id=?`, time.Now().Add(-4*time.Minute).Unix(), n.ID); err != nil {
		t.Fatal(err)
	}
	state, err := s.WireGuardReadiness(ctx, n.ID, false)
	if err != nil || state.Ready || state.Fresh || state.Installable {
		t.Fatal(state, err)
	}
	if err := s.CreateNodeJob(ctx, "expired", n.ID, spec.WireGuardSetupKind, json.RawMessage(`{"action":"install"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE node_jobs SET created_at=? WHERE id='expired'`, time.Now().Add(-6*time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.PendingNodeJobs(ctx, n.ID)
	if err != nil || len(jobs) != 0 {
		t.Fatal(jobs, err)
	}
	latest, err := s.LatestWireGuardSetupJob(ctx, n.ID)
	if err != nil || latest.DoneAt == nil || latest.Error == "" {
		t.Fatal(latest, err)
	}
	rep.Setup.Checks = make([]spec.PrivateAccessCheck, 33)
	if err := s.SetWireGuardReport(ctx, n.ID, "token", rep); err == nil {
		t.Fatal("oversize report accepted")
	}
}
