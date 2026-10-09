package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zeptop-dev/captain/internal/db"
)

func TestDNSJournalRestoresConnectionSyncPolicy(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "dns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	policy := func() int {
		t.Helper()
		var v int
		if err := conn.QueryRow(`PRAGMA synchronous`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	original := policy()
	for _, fail := range []bool{false, true} {
		if fail {
			if _, err := conn.Exec(`CREATE TRIGGER fail_dns BEFORE INSERT ON dns_changes BEGIN SELECT RAISE(FAIL, 'disk full'); END`); err != nil {
				t.Fatal(err)
			}
		}
		err := s.BeginDNSChange(ctx, &DNSChange{Zone: "example.com", Name: "node.example.com", Action: "updated"})
		if (err != nil) != fail {
			t.Fatalf("unexpected append result %v", err)
		}
		if got := policy(); got != original {
			t.Fatalf("connection sync changed: %d != %d", got, original)
		}
	}
}
