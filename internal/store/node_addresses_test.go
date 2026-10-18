package store

import (
	"context"
	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestNodeAddressFollowingPreservesFixedFamiliesAndRejectsOldIdentity(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "NAT", PublicAddr: "198.51.100.20", V6Addr: "2001:db8::20"}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.CreateNode(ctx, n, "pair", time.Hour))
	_, err = s.RedeemPairCode(ctx, "pair", "token", "node", "test", "linux")
	must(err)
	at := time.Now().Truncate(time.Second)
	sample := func(offset time.Duration, v4, v6 string) {
		t.Helper()
		when := at.Add(offset)
		must(s.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: when.Unix(), IPv4: v4, IPv6: v6}, when))
	}
	reconcile := func(offset time.Duration) *domain.Node {
		t.Helper()
		got, _, err := s.ReconcileNodeAddresses(ctx, n.ID, at.Add(offset))
		must(err)
		return got
	}
	sample(0, "192.0.2.10", "2001:db8::10")
	sample(5*time.Minute, "192.0.2.10", "2001:db8::10")
	if got := reconcile(5 * time.Minute); got.PublicAddr != n.PublicAddr || got.V6Addr != n.V6Addr {
		t.Fatal("fixed entries changed")
	}
	obs, err := s.NodeAddresses(ctx, n.ID)
	must(err)
	if obs.Observed.IPv4.Address != "192.0.2.10" || obs.Observed.IPv6.Address != "2001:db8::10" {
		t.Fatal("fixed mode stopped machine observations")
	}
	must(s.SetAddressPolicy(ctx, n.ID, true, false))
	if got := reconcile(5 * time.Minute); got.PublicAddr != "192.0.2.10" || got.V6Addr != n.V6Addr {
		t.Fatal("family opt-in failed")
	}
	sample(10*time.Minute, "203.0.113.30", "")
	if got := reconcile(10 * time.Minute); got.PublicAddr != "192.0.2.10" || got.V6Addr != n.V6Addr {
		t.Fatal("one probe changed entry")
	}
	// Repeated cached reports do not count as independent confirmation.
	must(s.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: at.Add(10 * time.Minute).Unix(), IPv4: "203.0.113.30"}, at.Add(13*time.Minute)))
	if got := reconcile(13 * time.Minute); got.PublicAddr != "192.0.2.10" {
		t.Fatal("cached report confirmed change")
	}
	sample(15*time.Minute, "203.0.113.30", "")
	if got := reconcile(15 * time.Minute); got.PublicAddr != "203.0.113.30" || got.V6Addr != n.V6Addr {
		t.Fatal("confirmed v4 or retained v6 wrong")
	}
	// A hand-edited entry turns off following only for the changed family.
	edited := reconcile(15 * time.Minute)
	edited.PublicAddr = "198.51.100.20"
	must(s.UpdateNode(ctx, edited))
	obs, err = s.NodeAddresses(ctx, n.ID)
	must(err)
	if obs.AutoIPv4 {
		t.Fatal("manual edit left follow enabled")
	}
	// Enabling IPv6 after failed probes must not apply an old confirmed address.
	must(s.SetAddressPolicy(ctx, n.ID, false, true))
	if got := reconcile(15 * time.Minute); got.V6Addr != n.V6Addr {
		t.Fatal("unknown IPv6 overwrote explicit entry")
	}
	must(s.ResetPairCode(ctx, n.ID, "newpair", time.Hour))
	_, err = s.RedeemPairCode(ctx, "newpair", "newtoken", "node", "test", "linux")
	must(err)
	if err = s.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: at.Unix(), IPv4: "192.0.2.10"}, at); err == nil {
		t.Fatal("old identity report accepted")
	}
	obs, err = s.NodeAddresses(ctx, n.ID)
	must(err)
	if obs.Observed.ReceivedAt != 0 {
		t.Fatal("pairing retained observation")
	}
}
