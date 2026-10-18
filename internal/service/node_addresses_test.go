package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/dns"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeAddressesDNSOnlyFollowsOptedInFamily(t *testing.T) {
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
	dom := store.Domain{Name: "example.com", Provider: "cloudflare", CFToken: "test-token", AutoDNS: true}
	must(st.CreateDomain(ctx, &dom))
	ingress := &store.Ingress{NodeID: n.ID, Name: "NAT", Kind: "nat", BindIP: "10.10.0.2", EntryHost: "203.0.113.30", EntryDomain: "nat.example.com"}
	must(st.CreateIngress(ctx, ingress))
	var writes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		if r.URL.Path == "/zones" {
			result = []map[string]string{{"id": "zone"}}
		} else if r.Method == "GET" {
			result = []dns.Record{{ID: "record", Name: n.Domain, Type: "A", Content: n.PublicAddr, TTL: 600}}
		} else {
			writes.Add(1)
			var rec dns.Record
			must(json.NewDecoder(r.Body).Decode(&rec))
			if rec.Type != "A" || rec.Content != "198.51.100.20" || rec.Name != n.Domain {
				t.Errorf("wrong automatic record %+v", rec)
			}
			// Intent must already exist before provider I/O; the worker must not hold
			// Accounts across it, otherwise reset/renumber can stall all new readers.
			history, err := st.DNSChanges(ctx, n.ID, 0)
			if err != nil || len(history) != int(writes.Load()) || history[0].Outcome != "unknown" {
				t.Errorf("missing intent: %v %v", history, err)
			}
			if !st.Accounts.TryLock() {
				t.Error("network call held Accounts")
			} else {
				st.Accounts.Unlock()
			}
			if writes.Load() == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			rec.ID = "record"
			result = rec
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer srv.Close()
	worker := &NodeAddresses{Store: st, DNS: &DNS{Store: st, Base: srv.URL}}
	at := time.Now().Truncate(time.Second)
	for _, offset := range []time.Duration{0, 5 * time.Minute} {
		when := at.Add(offset)
		must(st.ObserveNodeAddresses(ctx, n.ID, "token", &agentproto.PublicAddresses{SampledAt: when.Unix(), IPv4: "198.51.100.20", IPv6: "2001:db8::10"}, when))
	}
	worker.Tick(ctx, at.Add(5*time.Minute))
	if writes.Load() != 0 {
		t.Fatal("manual entry wrote DNS")
	}
	must(st.SetAddressPolicy(ctx, n.ID, true, false))
	worker.Tick(ctx, at.Add(5*time.Minute))
	if writes.Load() != 1 {
		t.Fatalf("writes %d", writes.Load())
	}
	got, err := st.NodeAddresses(ctx, n.ID)
	must(err)
	if got.EntryIPv4 != "198.51.100.20" || got.EntryIPv6 != "" || !got.DNSPending {
		t.Fatalf("result %+v", got)
	}
	worker.Tick(ctx, at.Add(6*time.Minute))
	if writes.Load() != 1 {
		t.Fatal("DNS failure retried before five minutes")
	}
	worker.Tick(ctx, at.Add(10*time.Minute))
	if writes.Load() != 2 {
		t.Fatalf("pending DNS not retried: %d", writes.Load())
	}
	got, err = st.NodeAddresses(ctx, n.ID)
	must(err)
	if got.DNSPending {
		t.Fatal("successful retry left pending flag")
	}
	worker.Tick(ctx, at.Add(11*time.Minute))
	if writes.Load() != 2 {
		t.Fatal("successful DNS repeated unchanged write")
	}
	ingresses, err := st.IngressesByNode(ctx, n.ID)
	must(err)
	if len(ingresses) != 1 || ingresses[0].EntryHost != "203.0.113.30" {
		t.Fatal("NAT public entry changed")
	}
}
