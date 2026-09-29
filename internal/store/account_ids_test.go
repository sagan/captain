package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestAccountReferencesAndRollback(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	// Check the migration schema independently of the reference list, so a
	// future FK cannot silently be left behind by the renumber operation.
	rows, err := conn.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	rows.Close()
	for _, table := range tables {
		rows, err := conn.Query(`PRAGMA foreign_key_list("` + table + `")`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id, seq int
			var parent, from, to, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				t.Fatal(err)
			}
			var refs []accountRef
			if parent == "users" {
				refs = customerRefs
			} else if parent == "staff" {
				refs = staffRefs
			} else {
				continue
			}
			found := false
			for _, ref := range refs {
				if ref.table == table && ref.column == from {
					found = true
				}
			}
			if !found {
				t.Fatalf("unhandled account FK: %s.%s -> %s.%s", table, from, parent, to)
			}
		}
		rows.Close()
	}
	u := &domain.User{Email: "a@example.com", UUID: "a", SubToken: "a", Status: "active"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO dyn_limits (user_id,mbps,since,until) VALUES (?,2,1,9999999999)`, u.ID); err != nil {
		t.Fatal(err)
	}
	// Inject a late failure after earlier updates to verify rollback really
	// restores the reference rows and the primary key together.
	if _, err := conn.Exec(`CREATE TRIGGER reject_account_move BEFORE UPDATE OF id ON users BEGIN SELECT RAISE(ABORT,'test rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeAccountID(ctx, u.ID, 42, false); err == nil {
		t.Fatal("injected failure was ignored")
	}
	var id int64
	if err := conn.QueryRow(`SELECT user_id FROM dyn_limits`).Scan(&id); err != nil || id != u.ID {
		t.Fatal("references partially moved", id, err)
	}
	if _, err := s.UserByID(ctx, u.ID); err != nil {
		t.Fatal("source disappeared", err)
	}
	if _, err := conn.Exec(`DROP TRIGGER reject_account_move`); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeAccountID(ctx, u.ID, 42, false); err != nil {
		t.Fatal(err)
	}
	// Historical rows without a FK still reserve their numeric owner.
	if _, err := conn.Exec(`INSERT INTO dyn_limits (user_id,mbps,since,until) VALUES (77,2,1,9999999999)`); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeAccountID(ctx, 42, 77, false); !errors.Is(err, ErrIDInUse) {
		t.Fatal("merged unrelated history", err)
	}
}
