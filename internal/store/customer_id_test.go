package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestCustomerIDAllocation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "captain.db")
	conn, err := db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	user := func(name string) *domain.User {
		return &domain.User{Email: name + "@example.com", UUID: name, SubToken: name, Status: "active"}
	}
	create := func(name string, chosen, want int64) *domain.User {
		t.Helper()
		u := user(name)
		var err error
		if chosen == 0 {
			err = s.CreateUser(ctx, u)
		} else {
			err = s.CreateUserWithID(ctx, u, chosen)
		}
		if err != nil || u.ID != want {
			t.Fatalf("create %s: ID=%d, want %d, err=%v", name, u.ID, want, err)
		}
		return u
	}
	create("test", 999999, 999999)
	a := create("first", 0, 1)
	create("reserved", 2, 2)
	// Non-FK history reserves IDs even when no customer row survives.
	if _, err := conn.Exec(`INSERT INTO dyn_limits (user_id,mbps,since,until) VALUES (3,2,1,9999999999)`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{2, 3} {
		if err := s.CreateUserWithID(ctx, user("collision"), id); !errors.Is(err, ErrIDInUse) {
			t.Fatalf("occupied ID %d accepted: %v", id, err)
		}
	}
	for _, id := range []int64{0, -1, MaxAccountID + 1} {
		if err := s.CreateUserWithID(ctx, user("invalid"), id); !errors.Is(err, ErrAccountID) {
			t.Fatalf("invalid ID %d accepted: %v", id, err)
		}
	}
	if err := s.CreateUser(ctx, user("first")); !errors.Is(err, ErrUserEmailInUse) {
		t.Fatal("duplicate email accepted", err)
	}
	// A failure after both sequence writes must roll the writes back too.
	broken := user("broken")
	broken.UUID = a.UUID
	if err := s.CreateUser(ctx, broken); err == nil {
		t.Fatal("duplicate credential accepted")
	}
	b := create("second", 0, 4)
	if b.AgentID != 4 {
		t.Fatal("failed creates consumed accounting IDs", b.AgentID)
	}
	if err := s.ChangeAccountID(ctx, a.ID, 888888, false); err != nil {
		t.Fatal(err)
	}
	create("largest", MaxAccountID, MaxAccountID)
	c := create("third", 0, 5)
	if err := s.DeleteUser(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	create("after-delete", 0, 6)
	// Explicit reuse gets a fresh node identity, not the deleted user's ID.
	reused := create("reused", 5, 5)
	if reused.AgentID <= c.AgentID {
		t.Fatal("reused accounting identity", reused.AgentID, c.AgentID)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	conn, err = db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	s = New(conn)
	create("after-restart", 0, 7)
	// Registration ignores any ID on its domain value; only the explicit
	// administrator entry point may choose it.
	registration := user("registered")
	registration.ID = 777777
	if err := s.CreateUser(ctx, registration); err != nil || registration.ID != 8 {
		t.Fatal("automatic creation accepted an injected ID", registration.ID, err)
	}

	// Concurrent manual and automatic requests serialize the checks, ID
	// allocation and insertion together, without consuming failed choices.
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			u := user(fmt.Sprintf("parallel-%d", i))
			var err error
			if i%2 == 0 {
				err = s.CreateUser(ctx, u)
			} else {
				err = s.CreateUserWithID(ctx, u, 1000+int64(i))
			}
			if err != nil {
				t.Errorf("parallel create: %v", err)
			}
		})
	}
	wg.Wait()
	create("after-parallel", 0, 15)
}
