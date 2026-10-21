package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestWireGuardOptionsStorageAndCapabilityFreshness(t *testing.T) {
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
	a, b := &domain.Node{Name: "A"}, &domain.Node{Name: "B"}
	for _, n := range []*domain.Node{a, b} {
		if err := s.CreateNode(ctx, n, "pair-"+n.Name, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RedeemPairCode(ctx, "pair-"+n.Name, "token-"+n.Name, "test", "test", "linux/amd64"); err != nil {
			t.Fatal(err)
		}
	}
	w := WireGuardWrite{
		Link:  WireGuardLink{ID: "options", TransitID: a.ID, ExitID: b.ID, Endpoint: "192.0.2.10:51820", ListenPort: 51820, MTU: 1380},
		User:  domain.Inbound{NodeID: a.ID, Tag: "wg-options", Protocol: spec.VLESS, Port: 18443, Core: "xray", Enabled: true},
		Entry: domain.Entry{Name: "WG options", DisplayHost: "example.com", DisplayPort: 18443, Enabled: true},
	}
	if err := s.SaveWireGuardLink(ctx, &w); err != nil {
		t.Fatal(err)
	}
	if w.Link.DomainPreference != "" || w.Link.AutoForwarding {
		t.Fatal("legacy options changed")
	}
	w.Link.DomainPreference, w.Link.AutoForwarding = "ipv4", true
	if err := s.SaveWireGuardLink(ctx, &w); err != nil {
		t.Fatal(err)
	}
	links, err := s.WireGuardLinks(ctx, a.ID)
	if err != nil || len(links) != 1 || links[0].DomainPreference != "ipv4" || !links[0].AutoForwarding {
		t.Fatalf("option persistence failed: %+v %v", links, err)
	}
	for _, exit := range []bool{false, true} {
		c, err := links[0].Config(exit, nil)
		if err != nil {
			t.Fatal(err)
		}
		if exit && (!c.AutoForwarding || c.DomainPreference != "") {
			t.Fatal("exit did not receive only its forwarding option")
		}
		if !exit && (c.DomainPreference != "ipv4" || c.AutoForwarding) {
			t.Fatal("transit did not receive only its routing option")
		}
	}
	bad := w
	bad.Link.DomainPreference = "unknown"
	bad.Entry.Name = "must not persist"
	if err := s.SaveWireGuardLink(ctx, &bad); err == nil {
		t.Fatal("store accepted unknown domain preference")
	}
	links, err = s.WireGuardLinks(ctx, a.ID)
	if err != nil || len(links) != 1 || links[0].Version != w.Link.Version || links[0].DomainPreference != "ipv4" {
		t.Fatal("rejected write changed saved options", err)
	}
	entry, err := s.EntryByID(ctx, w.Entry.ID)
	if err != nil || entry.Name != "WG options" {
		t.Fatal("rejected option changed owned entry", err)
	}
	for _, tc := range []struct {
		name            string
		report          *agentproto.WireGuardReport
		domain, forward bool
	}{
		{"omitted", nil, false, false},
		{"legacy", &agentproto.WireGuardReport{Available: true}, false, false},
		{"unavailable", &agentproto.WireGuardReport{DomainPreferenceSupported: true, ForwardingSupported: true}, false, false},
		{"domain only", &agentproto.WireGuardReport{Available: true, DomainPreferenceSupported: true}, true, false},
		{"forward only", &agentproto.WireGuardReport{Available: true, ForwardingSupported: true}, false, true},
		{"both", &agentproto.WireGuardReport{Available: true, DomainPreferenceSupported: true, ForwardingSupported: true}, true, true},
	} {
		if err := s.SetWireGuardReport(ctx, a.ID, "token-A", tc.report); err != nil {
			t.Fatal(err)
		}
		if got := s.WireGuardDomainPreferenceCapable(ctx, a.ID); got != tc.domain {
			t.Errorf("%s: domain support=%v want %v", tc.name, got, tc.domain)
		}
		if got := s.WireGuardForwardingCapable(ctx, a.ID); got != tc.forward {
			t.Errorf("%s: forwarding support=%v want %v", tc.name, got, tc.forward)
		}
	}
	for _, stamp := range []any{nil, time.Now().Add(-5 * time.Minute).Unix(), time.Now().Add(time.Minute).Unix()} {
		if _, err := conn.ExecContext(ctx, `UPDATE nodes SET wireguard_reported_at=? WHERE id=?`, stamp, a.ID); err != nil {
			t.Fatal(err)
		}
		// Resource heartbeats must not revive the independent capability clock.
		if err := s.TouchNode(ctx, a.ID, "test", "", spec.SystemStatus{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		if s.WireGuardDomainPreferenceCapable(ctx, a.ID) || s.WireGuardForwardingCapable(ctx, a.ID) {
			t.Fatalf("invalid capability timestamp %v was accepted", stamp)
		}
	}
}
