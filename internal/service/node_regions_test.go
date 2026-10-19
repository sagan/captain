package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeRegionsCacheModesAndAddressChanges(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := st.RedeemPairCode(ctx, "dns-test", "token", "node", "test", "linux")
	must(err)
	at := time.Now().Add(-time.Minute)
	observe := func(ip4, ip6 string) {
		t.Helper()
		at = at.Add(time.Second)
		must(st.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: at.Unix(), IPv4: ip4, IPv6: ip6}, at))
	}
	read := func() *store.NodeProbe { p, err := st.NodeProbe(ctx, n.ID); must(err); return p }
	set := func(mode, country string) {
		must(st.UpdateNodeProbeWithRegion(ctx, n.ID, false, store.NodeProbeInfo{Region: country}, 0, 1, "sum", nil, &mode))
	}
	calls := 0
	svc := &NodeRegions{Store: st, Lookup: func(ctx context.Context, ip string) (string, error) {
		calls++
		if !st.Accounts.TryLock() {
			t.Error("network holds Accounts")
		} else {
			st.Accounts.Unlock()
		}
		if !st.Topology.TryLock() {
			t.Error("network holds Topology")
		} else {
			st.Topology.Unlock()
		}
		if ip == "2001:db8::1" {
			return "US", nil
		}
		return "JP", nil
	}}
	if p := read(); p.RegionMode != "auto" || p.Info.Region != "" {
		t.Fatal(p)
	}
	// Fixed entry addresses are never looked up when no observation exists.
	must(svc.Detect(ctx, n.ID, true))
	if calls != 0 {
		t.Fatal(calls)
	}
	observe("198.51.100.20", "2001:db8::1")
	must(svc.Detect(ctx, n.ID, false))
	p := read()
	if p.Info.Region != "JP" || !p.RegionDetection.Conflict || p.RegionDetection.IP != "198.51.100.20" {
		t.Fatal(p)
	}
	must(svc.Detect(ctx, n.ID, false))
	must(svc.Detect(ctx, n.ID, true))
	if calls != 2 {
		t.Fatal("cache bypass", calls)
	}
	// A legacy client echoing the public result must not switch automatic mode off.
	must(st.UpdateNodeProbe(ctx, n.ID, false, p.Info, 100, 1, "sum"))
	if read().RegionMode != "auto" {
		t.Fatal("legacy echo lost auto")
	}
	set("manual", "DE")
	observe("203.0.113.30", "")
	if p = read(); p.Info.Region != "DE" || p.RegionDetection.Country != "" {
		t.Fatal("manual/IP change", p)
	}
	svc.Tick(ctx, time.Now())
	must(svc.Detect(ctx, n.ID, false))
	if calls != 2 {
		t.Fatal("manual scheduled lookup")
	}
	set("off", "")
	must(svc.Detect(ctx, n.ID, true))
	if p = read(); p.Info.Region != "" || p.RegionDetection.Country != "JP" {
		t.Fatal("off overwritten", p)
	}
	set("auto", "")
	if read().Info.Region != "JP" {
		t.Fatal("cached result not used")
	}
	observe("192.0.2.10", "")
	if read().Info.Region != "" {
		t.Fatal("old IP country leaked")
	}
	svc.Lookup = func(context.Context, string) (string, error) { calls++; return "", errors.New("failed") }
	must(svc.Detect(ctx, n.ID, false))
	p = read()
	if p.Info.Region != "" || p.RegionDetection.IPv4.Status != "failed" {
		t.Fatal(p)
	}
	count := calls
	must(svc.Detect(ctx, n.ID, false))
	if calls != count {
		t.Fatal("failed lookup not cached")
	}
	// Missing IPv4 selects IPv6; a failed IPv4 lookup does not silently switch families.
	svc.Lookup = func(context.Context, string) (string, error) { return "US", nil }
	observe("192.0.2.10", "2001:db8::1")
	must(svc.Detect(ctx, n.ID, false))
	if p = read(); p.Info.Region != "" || p.RegionDetection.IPv6.Country != "US" {
		t.Fatal("failed IPv4 silently fell back", p)
	}
	observe("", "2001:db8::1")
	must(svc.Detect(ctx, n.ID, false))
	if p = read(); p.Info.Region != "US" || p.RegionDetection.IP != "2001:db8::1" {
		t.Fatal(p)
	}
	// A legacy user changing country still expresses a manual override.
	must(st.UpdateNodeProbe(ctx, n.ID, false, store.NodeProbeInfo{Region: "FR"}, 0, 1, "sum"))
	if p = read(); p.RegionMode != "manual" || p.Info.Region != "FR" {
		t.Fatal(p)
	}
}

func TestNodeRegionsRejectLateResultsWithoutOverwritingMode(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := st.RedeemPairCode(ctx, "dns-test", "token", "node", "test", "linux")
	must(err)
	at := time.Now().Add(-time.Minute)
	observe := func(ip string) {
		at = at.Add(time.Second)
		must(st.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: at.Unix(), IPv4: ip}, at))
	}
	observe("198.51.100.20")
	svc := &NodeRegions{Store: st, Lookup: func(context.Context, string) (string, error) { observe("203.0.113.30"); return "JP", nil }}
	if err = svc.Detect(ctx, n.ID, false); !errors.Is(err, store.ErrRegionObservationChanged) {
		t.Fatal(err)
	}
	p, _ := st.NodeProbe(ctx, n.ID)
	if p.Info.Region != "" {
		t.Fatal(p)
	}
	svc.Lookup = func(context.Context, string) (string, error) {
		mode := "manual"
		must(st.UpdateNodeProbeWithRegion(ctx, n.ID, false, store.NodeProbeInfo{Region: "DE"}, 0, 1, "sum", nil, &mode))
		return "JP", nil
	}
	must(svc.Detect(ctx, n.ID, false))
	p, _ = st.NodeProbe(ctx, n.ID)
	if p.Info.Region != "DE" || p.RegionMode != "manual" {
		t.Fatal("late result overwrote manual", p)
	}
	// Re-pair/identity changes cannot accept a previously started lookup.
	old, err := st.NodeRegionState(ctx, n.ID)
	must(err)
	wrongIdentity := old
	wrongIdentity.Token = "other-pairing"
	if err = st.SaveRegionDetection(ctx, wrongIdentity, old.Detection); !errors.Is(err, store.ErrRegionObservationChanged) {
		t.Fatal(err)
	}
	// A cleared observation models the same invalidation as repair without touching DNS.
	observe("")
	if err = st.SaveRegionDetection(ctx, old, old.Detection); !errors.Is(err, store.ErrRegionObservationChanged) {
		t.Fatal(err)
	}
}

func TestNodeRegionsOfflineKeepsCountryWithoutQuerying(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	_, err := st.RedeemPairCode(ctx, "dns-test", "token", "node", "test", "linux")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-16 * time.Minute)
	if err = st.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: at.Unix(), IPv4: "198.51.100.20"}, at); err != nil {
		t.Fatal(err)
	}
	state, err := st.NodeRegionState(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SaveRegionDetection(ctx, state, store.RegionDetection{IPv4: store.RegionResult{IP: "198.51.100.20", Country: "JP", Status: "ok", CheckedAt: at.Unix()}}); err != nil {
		t.Fatal(err)
	}
	svc := &NodeRegions{Store: st, Lookup: func(context.Context, string) (string, error) { t.Fatal("stale address queried"); return "", nil }}
	if err = svc.Detect(ctx, n.ID, true); err != nil {
		t.Fatal(err)
	}
	p, err := st.NodeProbe(ctx, n.ID)
	if err != nil || p.Info.Region != "JP" || p.RegionDetection.Fresh {
		t.Fatal(p, err)
	}
}
