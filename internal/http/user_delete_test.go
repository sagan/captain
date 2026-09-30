package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/zeptop-dev/captain/internal/domain"
)

func TestDeleteUserWithHistory(t *testing.T) {
	for _, history := range []string{"order", "commission", "invited_user"} {
		t.Run(history, func(t *testing.T) {
			r := newRig(t)
			ctx := context.Background()
			// A migrated administrator's customer record has the same email
			// and ID as the console account, but is banned and passwordless.
			u := &domain.User{Email: "admin@test", UUID: "legacy", SubToken: "legacy", Status: "banned"}
			if err := r.st.CreateUser(ctx, u); err != nil {
				t.Fatal(err)
			}
			other := &domain.User{Email: "other@example.com", UUID: "other", SubToken: "other", Status: "active"}
			if err := r.st.CreateUser(ctx, other); err != nil {
				t.Fatal(err)
			}
			exec := func(q string, args ...any) {
				t.Helper()
				if _, err := r.st.DB().ExecContext(ctx, q, args...); err != nil {
					t.Fatal(err)
				}
			}
			if history == "invited_user" {
				exec(`UPDATE users SET invited_by = ? WHERE id = ?`, u.ID, other.ID)
			} else {
				p := &domain.Plan{Name: "test", PeriodDays: 30}
				if err := r.st.CreatePlan(ctx, p); err != nil {
					t.Fatal(err)
				}
				buyer := u.ID
				if history == "commission" {
					buyer = other.ID
				}
				o := &domain.Order{No: "history", UserID: buyer, PlanID: p.ID, Gateway: "balance"}
				if err := r.st.CreateOrder(ctx, o); err != nil {
					t.Fatal(err)
				}
				exec(`UPDATE orders SET status = 'paid' WHERE id = ?`, o.ID)
				if history == "commission" {
					exec(`INSERT INTO commissions (order_id,inviter_id,invitee_id,amount_cents,created_at) VALUES (?,?,?,100,1)`, o.ID, u.ID, other.ID)
				}
			}
			code, b, _ := r.c.do("DELETE", "/api/admin/users/"+itoa(u.ID), nil, nil)
			if code != http.StatusConflict {
				t.Fatalf("delete with %s: want 409, got %d: %s", history, code, b)
			}
			if got := mustJSON[map[string]string](t, b)["error"]; got != "user has orders, commissions or invited users; keep the account banned to preserve history" {
				t.Fatalf("missing actionable error: %q", got)
			}
			if got, err := r.st.UserByID(ctx, u.ID); err != nil || got.Status != "banned" || got.PasswordHash != "" {
				t.Fatalf("customer changed after refused deletion: %+v %v", got, err)
			}
			query := `SELECT COUNT(*) FROM orders WHERE no = 'history' AND status = 'paid'`
			if history == "commission" {
				query = `SELECT COUNT(*) FROM commissions WHERE inviter_id = 1 AND amount_cents = 100`
			} else if history == "invited_user" {
				query = `SELECT COUNT(*) FROM users WHERE invited_by = 1`
			}
			var count int
			if err := r.st.DB().QueryRowContext(ctx, query).Scan(&count); err != nil || count != 1 {
				t.Fatalf("history changed after refused deletion: %d %v", count, err)
			}
			if code, b, _ := r.c.do("GET", "/api/admin/me", nil, nil); code != http.StatusOK {
				t.Fatalf("customer deletion affected administrator: %d %s", code, b)
			}
		})
	}
}
