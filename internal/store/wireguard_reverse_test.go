package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestWireGuardOwnershipTransactions(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	a := &domain.Node{Name: "A"}
	b := &domain.Node{Name: "B"}
	if err := s.CreateNode(ctx, a, "pair-a", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateNode(ctx, b, "pair-b", 0); err != nil {
		t.Fatal(err)
	}
	makeWrite := func(id string) WireGuardWrite {
		return WireGuardWrite{
			Link:  WireGuardLink{ID: id, TransitID: a.ID, ExitID: b.ID, Endpoint: "192.0.2.10:51820", ListenPort: 51820, MTU: 1380},
			User:  domain.Inbound{NodeID: a.ID, Tag: "wg-" + id, Protocol: spec.VLESS, Port: 18443, Core: "xray", Enabled: true},
			Entry: domain.Entry{Name: "WG", DisplayHost: "example.com", DisplayPort: 18443, Enabled: true},
		}
	}
	w := makeWrite("first")
	if err := s.SaveWireGuardLink(ctx, &w); err != nil {
		t.Fatal(err)
	}
	if w.Link.Slot != 1 || !s.ReverseOwnedInbound(ctx, w.User.ID) || !s.ReverseOwnedEntry(ctx, w.Entry.ID) {
		t.Fatal("ownership missing")
	}
	raw, _ := json.Marshal(w.Link)
	if strings.Contains(string(raw), w.Link.TransitKey) || strings.Contains(string(raw), w.Link.ExitKey) {
		t.Fatal("key exposed in view")
	}
	old := w
	w.Link.MTU = 1300
	if err := s.SaveWireGuardLink(ctx, &w); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveWireGuardLink(ctx, &old); !errors.Is(err, ErrReverseConflict) {
		t.Fatal("stale update accepted", err)
	}
	if err := s.DeleteWireGuardLink(ctx, b.ID, w.Link.ID, old.Link.Version); !errors.Is(err, ErrReverseConflict) {
		t.Fatal("stale delete accepted", err)
	}
	if err := s.DeleteWireGuardLink(ctx, b.ID, w.Link.ID, w.Link.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InboundByID(ctx, w.User.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("owned inbound survived", err)
	}
	if _, err := s.EntryByID(ctx, w.Entry.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("owned entry survived", err)
	}
	second := makeWrite("second")
	if err := s.SaveWireGuardLink(ctx, &second); err != nil {
		t.Fatal(err)
	}
	if second.Link.Slot <= w.Link.Slot {
		t.Fatal("deleted slot reused")
	}
	// Deleting B must also remove resources on A, without deleting A itself.
	if _, err := conn.Exec(`DELETE FROM nodes WHERE id=?`, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InboundByID(ctx, second.User.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("exit removal left transit resource", err)
	}
	if _, err := s.NodeByID(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
}

func TestWireGuardReportPairIdentity(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "pair-test"}
	if err := s.CreateNode(ctx, n, "first", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemPairCode(ctx, "first", "old-token", "test", "test", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	report := &agentproto.WireGuardReport{Available: true}
	if err := s.SetWireGuardReport(ctx, n.ID, "old-token", report); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPairCode(ctx, n.ID, "second", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemPairCode(ctx, "second", "new-token", "test", "test", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWireGuardReport(ctx, n.ID, "old-token", report); !errors.Is(err, ErrNotFound) {
		t.Fatal("old identity restored capability", err)
	}
	got, at, err := s.WireGuardReport(ctx, n.ID)
	if err != nil || got != nil || !at.IsZero() {
		t.Fatal("pair inherited old capability", got, at, err)
	}
	if err := s.SetWireGuardReport(ctx, n.ID, "new-token", report); err != nil {
		t.Fatal(err)
	}
}
