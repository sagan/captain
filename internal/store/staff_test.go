package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestAccountIDsAndDeletionAreIndependent(t *testing.T) {
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
	for i, role := range []string{domain.RoleAdmin, domain.RoleOperator, domain.RoleSupport} {
		a := &domain.User{Email: role + "@example.com", Role: role, Status: "active"}
		if err := s.CreateStaff(ctx, a); err != nil || a.ID != int64(i+1) {
			t.Fatalf("staff sequence: %d %v", a.ID, err)
		}
	}
	for i, email := range []string{"admin@example.com", "operator@example.com"} {
		u := &domain.User{Email: email, Role: domain.RoleUser, Status: "active", UUID: email, SubToken: email}
		if err := s.CreateUser(ctx, u); err != nil || u.ID != int64(i+1) {
			t.Fatalf("customer sequence: %d %v", u.ID, err)
		}
		for _, staff := range []bool{false, true} {
			sid := email
			if staff {
				sid = "staff-" + sid
			}
			if err := s.CreateSession(ctx, &domain.Session{ID: sid, UserID: u.ID, Admin: staff, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
			got, isStaff, err := s.SessionUser(ctx, sid, time.Now())
			if err != nil || isStaff != staff || got.IsStaff() != staff || got.ID != u.ID {
				t.Fatalf("session namespace: %+v %v", got, err)
			}
		}
	}
	if err := s.CreateUser(ctx, &domain.User{Role: domain.RoleAdmin}); err == nil {
		t.Fatal("staff inserted into customers")
	}
	if err := s.CreateStaff(ctx, &domain.User{Role: domain.RoleUser}); err == nil {
		t.Fatal("customer inserted into staff")
	}
	plain, _, err := s.CreateAPIToken(ctx, 1, "test", ScopeRead, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetTOTP(ctx, 1, "test-secret", true); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got, scope, err := s.UserByAPIToken(ctx, plain); err != nil || !got.IsAdmin() || scope != ScopeRead {
		t.Fatal("customer deletion affected console token", err)
	}
	if _, _, err := s.SessionUser(ctx, "staff-admin@example.com", time.Now()); err != nil {
		t.Fatal("customer deletion affected staff session", err)
	}
	if _, _, err := s.SessionUser(ctx, "admin@example.com", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatal("customer session not revoked", err)
	}
	if secret, on, err := s.TOTP(ctx, 1); err != nil || !on || secret != "test-secret" {
		t.Fatal("customer deletion affected TOTP", err)
	}

	if err := s.LinkIdentity(ctx, 2, "idp", "subject", "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkStaffIdentity(ctx, 2, "idp", "subject", "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	staffToken, _, err := s.CreateAPIToken(ctx, 2, "test", "full", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteStaff(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if got, staff, err := s.SessionUser(ctx, "operator@example.com", time.Now()); err != nil || staff || got.ID != 2 {
		t.Fatal("staff deletion affected customer session", err)
	}
	if _, _, err := s.SessionUser(ctx, "staff-operator@example.com", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatal("staff session not revoked", err)
	}
	if _, _, err := s.UserByAPIToken(ctx, staffToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("staff token not revoked", err)
	}
	if _, err := s.StaffByIdentity(ctx, "idp", "subject"); !errors.Is(err, ErrNotFound) {
		t.Fatal("staff identity not removed", err)
	}
	if id, err := s.UserByIdentity(ctx, "idp", "subject"); err != nil || id != 2 {
		t.Fatal("staff deletion affected customer identity", err)
	}
}
