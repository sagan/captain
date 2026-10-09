package dns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureAddressPreservesExternalRecords(t *testing.T) {
	for _, records := range [][]Record{
		{{ID: "one", Content: "192.0.2.10"}, {ID: "two", Content: "198.51.100.20"}},
		{{ID: "one", Content: "192.0.2.10", Proxied: true}},
	} {
		t.Run(records[0].ID+string(rune('0'+len(records))), func(t *testing.T) {
			writes := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes++
				}
				var result any = records
				if r.URL.Path == "/zones" {
					result = []map[string]string{{"id": "zone"}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
			}))
			defer srv.Close()
			c := Cloudflare{Token: "test", Base: srv.URL}
			action, err := c.EnsureAddress(context.Background(), "example.com", "pool.example.com", "203.0.113.30")
			if err == nil || action != "skipped" || writes != 0 {
				t.Fatalf("external records changed: %s %v %d", action, err, writes)
			}
		})
	}
}
