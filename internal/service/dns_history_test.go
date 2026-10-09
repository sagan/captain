package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/dns"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestDNSHistoryDurableBeforeWriteAndProviderOutcome(t *testing.T) {
	for _, outcome := range []string{"success", "failed", "unknown"} {
		t.Run(outcome, func(t *testing.T) {
			ctx := context.Background()
			st, n := dnsTestStore(t)
			dom := store.Domain{Name: "example.com", Provider: "cloudflare", CFToken: "not-for-history", AutoDNS: true}
			if err := st.CreateDomain(ctx, &dom); err != nil {
				t.Fatal(err)
			}
			old := dns.Record{ID: "record-1", Name: n.Domain, Type: "A", Content: "198.51.100.20", TTL: 600}
			var writes atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var v any
				switch {
				case r.URL.Path == "/zones":
					v = []map[string]string{{"id": "zone-1"}}
				case r.Method == "GET":
					v = []dns.Record{old}
				default:
					writes.Add(1)
					changes, err := st.DNSChanges(ctx, n.ID, 0)
					if err != nil || len(changes) != 1 || changes[0].Outcome != "unknown" || changes[0].Before == nil || *changes[0].Before != old || changes[0].Wanted.Content != n.PublicAddr || changes[0].Wanted.TTL != 600 {
						t.Errorf("history absent before external write: %+v %v", changes, err)
					}
					if outcome == "failed" {
						w.WriteHeader(403)
						_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"secret-provider-error-not-for-history"}]}`))
						return
					}
					if outcome == "unknown" {
						w.WriteHeader(502)
						_, _ = w.Write([]byte("incomplete-response"))
						return
					}
					var rec dns.Record
					if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
						t.Error(err)
					}
					rec.ID = old.ID
					v = rec
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": v})
			}))
			defer srv.Close()
			d := &DNS{Store: st, Base: srv.URL}
			res := d.Ensure(WithDNSOrigin(ctx, DNSOrigin{NodeID: n.ID, Source: "node", Actor: "admin@example.com"}), n.Domain, n.PublicAddr)
			if writes.Load() != 1 || (res.Error == "") != (outcome == "success") {
				t.Fatalf("result %+v writes %d", res, writes.Load())
			}
			rows, err := st.DNSChanges(ctx, n.ID, 0)
			if err != nil || len(rows) != 1 {
				t.Fatalf("history %v %v", rows, err)
			}
			c := rows[0]
			if c.Outcome != outcome || c.Actor != "admin@example.com" || c.FinishedAt == 0 {
				t.Fatalf("completion %+v", c)
			}
			if outcome == "success" && (c.After == nil || c.After.Content != n.PublicAddr || c.After.ID != old.ID || c.After.TTL != 600) {
				t.Fatal("actual provider result missing", c)
			}
			b, _ := json.Marshal(c)
			if strings.Contains(string(b), "not-for-history") {
				t.Fatal("secret persisted")
			}
			if err := st.DeleteNode(ctx, n.ID); err != nil {
				t.Fatal(err)
			}
			rows, err = st.DNSChanges(ctx, 0, 0)
			if err != nil || len(rows) != 1 || rows[0].NodeID != nil || rows[0].Before.Content != old.Content {
				t.Fatalf("node deletion lost history: %v %v", rows, err)
			}
			if err := st.PruneDNSChanges(ctx, time.Now().AddDate(0, 0, 181)); err != nil {
				t.Fatal(err)
			}
			rows, _ = st.DNSChanges(ctx, 0, 0)
			if len(rows) != 0 {
				t.Fatal("retention failed")
			}
		})
	}
}

func TestDNSHistoryFailurePreventsExternalWrite(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	if err := st.CreateDomain(ctx, &store.Domain{Name: "example.com", Provider: "cloudflare", CFToken: "test", AutoDNS: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TRIGGER fail_dns_journal BEFORE INSERT ON dns_changes BEGIN SELECT RAISE(FAIL, 'disk full'); END`); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		}
		var result any = []dns.Record{}
		if r.URL.Path == "/zones" {
			result = []map[string]string{{"id": "zone"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer srv.Close()
	d := &DNS{Store: st, Base: srv.URL}
	res := d.Ensure(ctx, n.Domain, n.PublicAddr)
	if res.Error == "" || writes.Load() != 0 {
		t.Fatalf("unlogged external mutation %+v %d", res, writes.Load())
	}
}

func TestDNSHistoryCreateAndUnchanged(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	if err := st.CreateDomain(ctx, &store.Domain{Name: "example.com", Provider: "cloudflare", CFToken: "test", AutoDNS: true}); err != nil {
		t.Fatal(err)
	}
	var rec *dns.Record
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch {
		case r.URL.Path == "/zones":
			result = []map[string]string{{"id": "zone"}}
		case r.Method == "GET":
			result = []dns.Record{}
			if rec != nil {
				result = []dns.Record{*rec}
			}
		case r.Method == "POST":
			rec = &dns.Record{}
			_ = json.NewDecoder(r.Body).Decode(rec)
			rec.ID = "created-record"
			result = rec
		default:
			t.Error("unexpected update")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer srv.Close()
	d := &DNS{Store: st, Base: srv.URL}
	for _, want := range []string{"created", "unchanged"} {
		if res := d.Ensure(ctx, n.Domain, n.PublicAddr); res.Action != want || res.Error != "" {
			t.Fatalf("result %+v", res)
		}
	}
	rows, err := st.DNSChanges(ctx, 0, 0)
	if err != nil || len(rows) != 1 || rows[0].Before != nil || rows[0].After.ID != "created-record" {
		t.Fatalf("create history %v %v", rows, err)
	}
}

func TestDNSHistoryCompletionFailureRetainsPreviousRecord(t *testing.T) {
	ctx := context.Background()
	st, n := dnsTestStore(t)
	if err := st.CreateDomain(ctx, &store.Domain{Name: "example.com", Provider: "cloudflare", CFToken: "test", AutoDNS: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TRIGGER fail_dns_finish BEFORE UPDATE ON dns_changes BEGIN SELECT RAISE(FAIL, 'disk full'); END`); err != nil {
		t.Fatal(err)
	}
	old := dns.Record{ID: "record", Name: n.Domain, Type: "A", Content: "198.51.100.20", TTL: 300}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch {
		case r.URL.Path == "/zones":
			result = []map[string]string{{"id": "zone"}}
		case r.Method == "GET":
			result = []dns.Record{old}
		default:
			after := old
			after.Content = n.PublicAddr
			result = after
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer srv.Close()
	d := &DNS{Store: st, Base: srv.URL}
	res := d.Ensure(ctx, n.Domain, n.PublicAddr)
	if res.Action != "updated" || res.Error == "" {
		t.Fatal("completion failure hidden", res)
	}
	rows, err := st.DNSChanges(ctx, 0, 0)
	if err != nil || len(rows) != 1 || rows[0].Outcome != "unknown" || rows[0].Before.Content != old.Content {
		t.Fatalf("lost recovery information: %v %v", rows, err)
	}
}
